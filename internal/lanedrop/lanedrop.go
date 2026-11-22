// Package lanedrop finds places on the corridor route where more lanes come
// in than carry on: merges at junctions and narrowings along a road.
//
// It counts lanes only. It does not know how many cars use a road, so a
// small drop with heavy traffic can jam worse than a big one with little.
package lanedrop

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/graph"
)

// Kind says where the lanes are lost.
type Kind string

const (
	// Merge is a junction where incoming lanes exceed outgoing lanes.
	Merge Kind = "merge"
	// Narrowing is a road losing lanes between two OSM ways, with no junction.
	Narrowing Kind = "narrowing"
)

// Road is one road feeding into or leaving a drop.
type Road struct {
	OSMWayID      int64  `json:"osm_way_id"`
	Name          string `json:"name"`
	Ref           string `json:"ref"`
	Class         string `json:"class"`
	Lanes         int    `json:"lanes"`
	LanesInferred bool   `json:"lanes_inferred"`
}

// Confirmation records that a person checked the drop on the ground.
type Confirmation struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// Drop is one place where lanes run out (mirror in web/src/types.ts).
type Drop struct {
	Rank      int          `json:"rank"` // 1 = biggest squeeze
	Key       string       `json:"key"`  // stable across rebuilds; see config.ConfirmedDrop
	Kind      Kind         `json:"kind"`
	Pos       graph.LatLon `json:"pos"`
	LanesIn   int          `json:"lanes_in"`
	LanesOut  int          `json:"lanes_out"`
	LanesLost int          `json:"lanes_lost"`
	// Squeeze is LanesOut / LanesIn; smaller is a bigger squeeze.
	Squeeze float64 `json:"squeeze"`
	From    []Road  `json:"from"`
	To      []Road  `json:"to"`
	// LanesInferred is true if any lane count here is a class default
	// (no lanes tag in OSM), so the drop may not be real.
	LanesInferred bool          `json:"lanes_inferred"`
	Confirmed     *Confirmation `json:"confirmed"`
}

// Find returns the lane drops on the route, biggest squeeze first.
//
// A merge is a node without traffic signals (at signals, more lanes in than
// out is normal: movements take turns) where every road is in cfg.Classes,
// at least one road is on cfg.RouteRefs, and lanes in > lanes out. Lanes are
// counted on the part of each edge that touches the node.
//
// A narrowing is two consecutive parts of one edge where the second has
// fewer lanes. The edge must be on the route, or be a ramp touching it.
//
// Drops within cfg.TollPlazaRadiusM of a toll booth are skipped. Every
// cfg.Confirmed key must match a drop, so a stale key fails loudly.
func Find(g *graph.Graph, cfg config.LaneDrops) ([]Drop, error) {
	ins := map[graph.NodeID][]*graph.Edge{}
	outs := map[graph.NodeID][]*graph.Edge{}
	for i := range g.Edges {
		e := &g.Edges[i]
		outs[e.From] = append(outs[e.From], e)
		ins[e.To] = append(ins[e.To], e)
	}
	classOK := set(cfg.Classes)
	refs := set(cfg.RouteRefs)
	onRoute := func(e *graph.Edge) bool {
		for _, r := range strings.Split(e.Ref, ";") {
			if refs[strings.TrimSpace(r)] {
				return true
			}
		}
		return false
	}
	nearToll := func(p graph.LatLon) bool {
		for _, t := range g.TollBooths {
			if graph.HaversineM(p, t) < cfg.TollPlazaRadiusM {
				return true
			}
		}
		return false
	}

	var drops []Drop
	for _, id := range sortedNodeIDs(g) {
		n := g.Nodes[id]
		in, out := ins[id], outs[id]
		if n.Signals || len(in) == 0 || len(out) == 0 {
			continue
		}
		all := append(append([]*graph.Edge{}, in...), out...)
		ok, route := true, false
		for _, e := range all {
			ok = ok && classOK[string(e.Class)]
			route = route || onRoute(e)
		}
		if !ok || !route || nearToll(n.Pos) {
			continue
		}
		d := Drop{Key: fmt.Sprintf("node:%d", id), Kind: Merge, Pos: n.Pos}
		for _, e := range in {
			d.From = append(d.From, road(e, e.Parts[len(e.Parts)-1]))
		}
		for _, e := range out {
			d.To = append(d.To, road(e, e.Parts[0]))
		}
		if d.finish() {
			drops = append(drops, d)
		}
	}

	for i := range g.Edges {
		e := &g.Edges[i]
		if !classOK[string(e.Class)] || !(onRoute(e) || touchesRoute(e, ins, outs, onRoute)) {
			continue
		}
		for k := 1; k < len(e.Parts); k++ {
			a, b := e.Parts[k-1], e.Parts[k]
			if b.Lanes >= a.Lanes || nearToll(a.End) {
				continue
			}
			d := Drop{
				Key:  fmt.Sprintf("way:%d>%d", a.OSMWayID, b.OSMWayID),
				Kind: Narrowing,
				Pos:  a.End,
				From: []Road{road(e, a)},
				To:   []Road{road(e, b)},
			}
			d.finish()
			drops = append(drops, d)
		}
	}

	sort.Slice(drops, func(i, j int) bool {
		a, b := drops[i], drops[j]
		if a.Squeeze != b.Squeeze {
			return a.Squeeze < b.Squeeze
		}
		if a.LanesLost != b.LanesLost {
			return a.LanesLost > b.LanesLost
		}
		return a.Key < b.Key
	})
	byKey := map[string]int{}
	for i := range drops {
		drops[i].Rank = i + 1
		if _, dup := byKey[drops[i].Key]; dup {
			return nil, fmt.Errorf("lane drops: duplicate key %s", drops[i].Key)
		}
		byKey[drops[i].Key] = i
	}
	for _, c := range cfg.Confirmed {
		i, ok := byKey[c.Key]
		if !ok {
			return nil, fmt.Errorf("lane drops: confirmed %q (%s) not found; OSM data changed? re-check the key", c.Name, c.Key)
		}
		drops[i].Confirmed = &Confirmation{Name: c.Name, Source: c.Source}
	}
	return drops, nil
}

// finish fills the lane totals and reports whether lanes were lost.
func (d *Drop) finish() bool {
	for _, r := range d.From {
		d.LanesIn += r.Lanes
		d.LanesInferred = d.LanesInferred || r.LanesInferred
	}
	for _, r := range d.To {
		d.LanesOut += r.Lanes
		d.LanesInferred = d.LanesInferred || r.LanesInferred
	}
	d.LanesLost = d.LanesIn - d.LanesOut
	if d.LanesIn > 0 {
		d.Squeeze = float64(d.LanesOut) / float64(d.LanesIn)
	}
	return d.LanesLost > 0
}

func road(e *graph.Edge, p graph.Segment) Road {
	return Road{
		OSMWayID:      p.OSMWayID,
		Name:          e.Name,
		Ref:           e.Ref,
		Class:         string(e.Class),
		Lanes:         p.Lanes,
		LanesInferred: p.LanesInferred,
	}
}

// touchesRoute reports whether e starts or ends at a node with a route road.
func touchesRoute(e *graph.Edge, ins, outs map[graph.NodeID][]*graph.Edge, onRoute func(*graph.Edge) bool) bool {
	for _, n := range []graph.NodeID{e.From, e.To} {
		for _, x := range append(append([]*graph.Edge{}, ins[n]...), outs[n]...) {
			if onRoute(x) {
				return true
			}
		}
	}
	return false
}

func sortedNodeIDs(g *graph.Graph) []graph.NodeID {
	ids := make([]graph.NodeID, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func set(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}
