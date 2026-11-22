package graph

// simplify merges chains of arcs through pass-through nodes.
//
// A node v is pass-through for arc a (u→v) when the road just continues:
//   - one-way: v has exactly one in-arc (a) and one out-arc (v→w, w≠u), or
//   - two-way: v has exactly two in-arcs (from u and w) and two out-arcs
//     (to u and w), with u≠w; a continues with v→w.
//
// and the two arcs have the same road attributes (sameRoad). Lanes and
// signals may differ: that is the point. The merged edge takes Lanes,
// LanesInferred and CapacityVPH from its lowest-capacity part, so a short
// 2-lane pinch inside a 4-lane road survives merging. Never average.
//
// Output order is deterministic for a given input order.
func simplify(edges []Edge) []Edge {
	in := map[NodeID][]int{}
	out := map[NodeID][]int{}
	for i, e := range edges {
		out[e.From] = append(out[e.From], i)
		in[e.To] = append(in[e.To], i)
	}

	next := make([]int, len(edges))
	hasPrev := make([]bool, len(edges))
	for a := range edges {
		next[a] = continuation(edges, in, out, a)
		if next[a] >= 0 {
			hasPrev[next[a]] = true
		}
	}

	used := make([]bool, len(edges))
	var res []Edge
	for a := range edges {
		if hasPrev[a] {
			continue
		}
		var chain []int
		for c := a; c >= 0 && !used[c]; c = next[c] {
			used[c] = true
			chain = append(chain, c)
		}
		res = append(res, mergeChain(edges, chain)...)
	}
	// Arcs left over form closed rings of pass-through nodes (no way on or
	// off). Keep them unmerged; they can't affect flow between gates.
	for a := range edges {
		if !used[a] {
			res = append(res, edges[a])
		}
	}
	return res
}

// continuation returns the arc that a continues into, or -1.
func continuation(edges []Edge, in, out map[NodeID][]int, a int) int {
	u, v := edges[a].From, edges[a].To
	ins, outs := in[v], out[v]
	b := -1
	switch {
	case len(ins) == 1 && len(outs) == 1:
		if edges[outs[0]].To != u {
			b = outs[0]
		}
	case len(ins) == 2 && len(outs) == 2:
		other := ins[0]
		if other == a {
			other = ins[1]
		}
		w := edges[other].From
		if w == u {
			return -1 // parallel arcs from the same neighbour: a real fork
		}
		o0, o1 := edges[outs[0]].To, edges[outs[1]].To
		switch {
		case o0 == w && o1 == u:
			b = outs[0]
		case o1 == w && o0 == u:
			b = outs[1]
		}
	}
	if b < 0 || !sameRoad(edges[a], edges[b]) {
		return -1
	}
	return b
}

// sameRoad reports whether two arcs may merge. Bridge and tunnel are
// compared so fix difficulty stays per structure, not per chain.
func sameRoad(a, b Edge) bool {
	return a.Class == b.Class && a.Name == b.Name && a.Ref == b.Ref &&
		a.Toll == b.Toll && a.Operator == b.Operator &&
		a.Bridge == b.Bridge && a.Tunnel == b.Tunnel &&
		a.MaxSpeedKPH == b.MaxSpeedKPH
}

// mergeChain merges consecutive arcs into one edge. A chain that returns to
// its own start would become a self-loop, so it is left unmerged.
func mergeChain(edges []Edge, chain []int) []Edge {
	first, last := edges[chain[0]], edges[chain[len(chain)-1]]
	if len(chain) == 1 || first.From == last.To {
		res := make([]Edge, len(chain))
		for i, c := range chain {
			res[i] = edges[c]
		}
		return res
	}

	m := first
	m.To = last.To
	m.OSMWayIDs = nil
	m.Parts = nil
	m.Geometry = nil
	m.LengthM = 0
	for i, c := range chain {
		e := edges[c]
		for _, id := range e.OSMWayIDs {
			if n := len(m.OSMWayIDs); n == 0 || m.OSMWayIDs[n-1] != id {
				m.OSMWayIDs = append(m.OSMWayIDs, id)
			}
		}
		geom := e.Geometry
		if i > 0 {
			geom = geom[1:] // shared joint
		}
		m.Geometry = append(m.Geometry, geom...)
		m.LengthM += e.LengthM
		m.Parts = append(m.Parts, e.Parts...)
	}

	// Bottleneck = lowest capacity; on a tie prefer a part with tagged lanes.
	bn := m.Parts[0]
	for _, p := range m.Parts[1:] {
		if p.CapacityVPH < bn.CapacityVPH ||
			(p.CapacityVPH == bn.CapacityVPH && bn.LanesInferred && !p.LanesInferred) {
			bn = p
		}
	}
	m.Lanes, m.LanesInferred, m.CapacityVPH = bn.Lanes, bn.LanesInferred, bn.CapacityVPH
	return []Edge{m}
}
