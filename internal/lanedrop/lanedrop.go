// Package lanedrop finds places on the corridor route where more lanes come
// in than carry on: merges at junctions and narrowings along a road.
//
// It counts lanes only. It does not know how many cars use a road, so a
// small drop with heavy traffic can jam worse than a big one with little.
package lanedrop

import (
	"fmt"
	"math"
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
	// Leaves and Joins name the nearest named road (ref, else name) where
	// an unnamed ramp starts and ends, following other ramps on the way.
	// Empty for roads that have a name or ref, or if none is found.
	Leaves string `json:"leaves"`
	Joins  string `json:"joins"`
	// Side is where this road sits across the carriageway, looking in the
	// direction of travel: "left", "middle" or "right". It is set at a
	// merge where two or more roads come in (on From) or go out (on To),
	// from each road's own geometry near the junction. Empty when there is
	// only one road on that side of the drop (every narrowing), or when
	// the geometry cannot tell the roads apart.
	Side Side `json:"side"`
}

// Side says where a road joins or leaves, across the road in the direction
// of travel.
type Side string

const (
	Left   Side = "left"
	Middle Side = "middle"
	Right  Side = "right"
)

// sideProbeM is how far along each road, from the junction, the point that
// decides its side is taken. Long enough to clear the junction's own
// rounding, short enough that a ramp curving in still counts by how it
// arrives, not where it started. Geometry only: no capacity meaning.
const sideProbeM = 60.0

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

	road := func(e *graph.Edge, p graph.Segment) Road {
		r := Road{
			OSMWayID:      p.OSMWayID,
			Name:          e.Name,
			Ref:           e.Ref,
			Class:         string(e.Class),
			Lanes:         p.Lanes,
			LanesInferred: p.LanesInferred,
		}
		if isRamp(e) {
			r.Leaves = nearestNamed(e.From, ins, func(x *graph.Edge) graph.NodeID { return x.From })
			r.Joins = nearestNamed(e.To, outs, func(x *graph.Edge) graph.NodeID { return x.To })
		}
		return r
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
		setSides(g, n.Pos, in, out, d.From, d.To)
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

// setSides fills Side on the roads coming in (from, built from in) and
// going out (to, built from out) at a merge at node position at.
//
// The direction of travel is the mean of the unit vectors from where the
// incoming roads arrive to the junction and from the junction to where the
// outgoing roads go. Each road's probe point (sideProbeM along it) is then
// left or right of that line; across a group, the most-left road is Left,
// the most-right is Right and any between are Middle.
func setSides(g *graph.Graph, at graph.LatLon, in, out []*graph.Edge, from, to []Road) {
	inPts := make([]vec, len(in))
	outPts := make([]vec, len(out))
	var heading vec
	for i, e := range in {
		inPts[i] = local(at, probe(g, e, false))
		heading = heading.add(inPts[i].unit().scale(-1))
	}
	for i, e := range out {
		outPts[i] = local(at, probe(g, e, true))
		heading = heading.add(outPts[i].unit())
	}
	if heading.len() == 0 {
		return
	}
	heading = heading.unit()
	rank(from, inPts, heading)
	rank(to, outPts, heading)
}

// rank sets Side on roads (two or more) by how far left of heading each
// probe point lies. Leaves them empty if the most-left and most-right are
// indistinguishable.
func rank(roads []Road, pts []vec, heading vec) {
	if len(roads) < 2 {
		return
	}
	// left > 0: the point is to the left of the heading (x east, y north).
	left := make([]float64, len(pts))
	for i, p := range pts {
		left[i] = heading.cross(p.unit())
	}
	order := make([]int, len(roads))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return left[order[a]] > left[order[b]] })
	if left[order[0]]-left[order[len(order)-1]] < 1e-6 {
		return
	}
	for k, i := range order {
		switch k {
		case 0:
			roads[i].Side = Left
		case len(order) - 1:
			roads[i].Side = Right
		default:
			roads[i].Side = Middle
		}
	}
}

// probe returns the point sideProbeM along e from the junction: from its
// start if forward (an outgoing road), else back from its end (incoming).
// Without geometry it falls back to the far node.
func probe(g *graph.Graph, e *graph.Edge, forward bool) graph.LatLon {
	pts := e.Geometry
	if len(pts) < 2 {
		far := e.From
		if forward {
			far = e.To
		}
		if n, ok := g.Nodes[far]; ok {
			return n.Pos
		}
		return graph.LatLon{}
	}
	if !forward {
		pts = make([]graph.LatLon, len(e.Geometry))
		for i, p := range e.Geometry {
			pts[len(pts)-1-i] = p
		}
	}
	walked := 0.0
	for i := 1; i < len(pts); i++ {
		step := graph.HaversineM(pts[i-1], pts[i])
		if walked+step >= sideProbeM && step > 0 {
			f := (sideProbeM - walked) / step
			return graph.LatLon{
				Lat: pts[i-1].Lat + f*(pts[i].Lat-pts[i-1].Lat),
				Lon: pts[i-1].Lon + f*(pts[i].Lon-pts[i-1].Lon),
			}
		}
		walked += step
	}
	return pts[len(pts)-1]
}

// vec is a flat offset in metres: x east, y north.
type vec struct{ x, y float64 }

// local projects p to metres from origin o (equirectangular; fine over
// the few hundred metres around a junction).
func local(o, p graph.LatLon) vec {
	const mPerDegLat = 111_320.0
	return vec{
		x: (p.Lon - o.Lon) * mPerDegLat * math.Cos(o.Lat*math.Pi/180),
		y: (p.Lat - o.Lat) * mPerDegLat,
	}
}

func (a vec) add(b vec) vec       { return vec{a.x + b.x, a.y + b.y} }
func (a vec) scale(k float64) vec { return vec{a.x * k, a.y * k} }
func (a vec) len() float64        { return math.Hypot(a.x, a.y) }
func (a vec) cross(b vec) float64 { return a.x*b.y - a.y*b.x }
func (a vec) unit() vec {
	if l := a.len(); l > 0 {
		return a.scale(1 / l)
	}
	return a
}

// isRamp reports whether e is an unnamed link road (OSM *_link).
func isRamp(e *graph.Edge) bool {
	return strings.HasSuffix(string(e.Class), "_link") && e.Ref == "" && e.Name == ""
}

// label is how a named road is called: its ref, else its name.
func label(e *graph.Edge) string {
	if e.Ref != "" {
		return e.Ref
	}
	return e.Name
}

// nearestNamed searches breadth-first from node start through unnamed ramps
// (via adj and next: in-edges walking backward, or out-edges walking
// forward) and returns the label of the first named road it meets. Ties at
// the same depth go to the lowest edge ID, so the result is stable.
func nearestNamed(start graph.NodeID, adj map[graph.NodeID][]*graph.Edge, next func(*graph.Edge) graph.NodeID) string {
	seen := map[graph.NodeID]bool{start: true}
	frontier := []graph.NodeID{start}
	for len(frontier) > 0 {
		var edges []*graph.Edge
		for _, n := range frontier {
			edges = append(edges, adj[n]...)
		}
		sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
		for _, e := range edges {
			if !isRamp(e) {
				return label(e)
			}
		}
		frontier = frontier[:0]
		for _, e := range edges {
			if n := next(e); !seen[n] {
				seen[n] = true
				frontier = append(frontier, n)
			}
		}
	}
	return ""
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
