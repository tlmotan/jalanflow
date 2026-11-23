// Package fixes ranks "+1 lane" changes on min-cut edges.
package fixes

import (
	"fmt"
	"sort"

	"github.com/tlmotan/jalanflow/internal/flow"
	"github.com/tlmotan/jalanflow/internal/graph"
)

// Direction of analysis.
type Direction string

const (
	Inbound  Direction = "inbound"
	Outbound Direction = "outbound"
)

// Difficulty is a rough feasibility label.
type Difficulty string

const (
	Easy   Difficulty = "easy"
	Medium Difficulty = "medium"
	Hard   Difficulty = "hard"
)

// Fix is one tested change and its effect.
type Fix struct {
	EdgeID        graph.EdgeID   `json:"edge_id"`
	Direction     Direction      `json:"direction"`
	Change        string         `json:"change"`
	GainVPH       int            `json:"gain_vph"`
	NewMaxFlowVPH int            `json:"new_max_flow_vph"`
	NextCutEdges  []graph.EdgeID `json:"next_cut_edges"`
	// Helps is true when this +1 lane alone raises max flow. Only edges in
	// every min cut can (Picard-Queyranne); see flow.InEveryMinCut.
	Helps bool `json:"helps"`
	// WhyNoGain explains a zero gain in plain words; empty when Helps.
	WhyNoGain  string     `json:"why_no_gain,omitempty"`
	Difficulty Difficulty `json:"difficulty"`
	Reasons    []string   `json:"reasons"`
}

// Rate labels an edge using only OSM tags (ARCHITECTURE.md §3.5).
func Rate(e graph.Edge) (Difficulty, []string) {
	var reasons []string
	d := Easy
	if e.Class == "motorway" || e.Class == "trunk" {
		d = Medium
		reasons = append(reasons, "major highway: widening needs road reserve")
	}
	if e.Toll || e.Operator != "" {
		d = Medium
		reasons = append(reasons, "tolled or concession road: needs concessionaire agreement")
	}
	if e.Bridge || e.Tunnel {
		d = Hard
		reasons = append(reasons, "bridge or tunnel: structural work")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "local road: re-striping or junction change may suffice")
	}
	return d, reasons
}

// Plain-language reasons a +1 lane gains nothing.
const (
	WhyOtherCut    = "another set of roads is just as tight, so traffic still queues there; this only helps together with widening those"
	WhyOtherPart   = "another stretch of this same road has as few lanes, so it stays the squeeze"
	WhyNoLaneCount = "lane count unknown, so the per-lane capacity cannot be worked out"
)

// Result is the max flow analysis for one direction.
type Result struct {
	Direction  Direction
	MaxFlowVPH int
	EdgeFlow   []int          // indexed by EdgeID
	CutEdges   []graph.EdgeID // source-side min cut
	Fixes      []Fix          // one per cut edge, best gain first
}

// Rank runs max flow from sources to sinks, then tests "+1 lane" on every
// edge of the source-side min cut. Edges that are not in every min cut
// are marked Helps=false without a re-run: their gain is provably zero.
// The rest are re-run to measure the gain and where the bottleneck moves.
func Rank(g *graph.Graph, dir Direction, sources, sinks []graph.NodeID) (Result, error) {
	net, err := newNetwork(g, sources, sinks, nil)
	if err != nil {
		return Result{}, err
	}
	res := Result{Direction: dir, MaxFlowVPH: net.f.MaxFlow(net.s, net.t)}
	res.EdgeFlow = make([]int, len(g.Edges))
	for i := range g.Edges {
		if id := net.arc[i]; id >= 0 {
			res.EdgeFlow[i] = net.f.Flow(id)
		}
	}
	res.CutEdges = net.edges(net.f.MinCut(net.s))
	every := map[graph.EdgeID]bool{}
	for _, e := range net.edges(net.f.InEveryMinCut(net.s, net.t)) {
		every[e] = true
	}

	for _, eid := range res.CutEdges {
		e := g.Edges[eid]
		d, reasons := Rate(e)
		fx := Fix{
			EdgeID: eid, Direction: dir, Change: "+1 lane",
			NewMaxFlowVPH: res.MaxFlowVPH, NextCutEdges: []graph.EdgeID{},
			Difficulty: d, Reasons: reasons,
		}
		widened, ok := plusOneLane(e)
		switch {
		case !ok:
			fx.WhyNoGain = WhyNoLaneCount
		case widened <= e.CapacityVPH:
			fx.WhyNoGain = WhyOtherPart
		case !every[eid]:
			fx.WhyNoGain = WhyOtherCut
		default:
			after, err := newNetwork(g, sources, sinks, map[graph.EdgeID]int{eid: widened})
			if err != nil {
				return Result{}, err
			}
			fx.NewMaxFlowVPH = after.f.MaxFlow(after.s, after.t)
			fx.GainVPH = fx.NewMaxFlowVPH - res.MaxFlowVPH
			fx.NextCutEdges = after.edges(after.f.MinCut(after.s))
			fx.Helps = fx.GainVPH > 0
		}
		res.Fixes = append(res.Fixes, fx)
	}
	sort.SliceStable(res.Fixes, func(i, j int) bool {
		a, b := res.Fixes[i], res.Fixes[j]
		if a.GainVPH != b.GainVPH {
			return a.GainVPH > b.GainVPH
		}
		return a.EdgeID < b.EdgeID
	})
	return res, nil
}

// plusOneLane returns the edge capacity after adding one lane to its
// narrowest part (first one on ties). Per-lane capacity is that part's
// capacity / lanes, so class and signal factors carry over. Other parts
// still cap the edge: merging keeps the minimum.
func plusOneLane(e graph.Edge) (int, bool) {
	parts := e.Parts
	if len(parts) == 0 {
		parts = []graph.Segment{{Lanes: e.Lanes, CapacityVPH: e.CapacityVPH}}
	}
	k := 0
	for i, p := range parts {
		if p.CapacityVPH < parts[k].CapacityVPH {
			k = i
		}
	}
	if parts[k].Lanes <= 0 {
		return 0, false
	}
	capacity := parts[k].CapacityVPH * (parts[k].Lanes + 1) / parts[k].Lanes
	for i, p := range parts {
		if i != k {
			capacity = min(capacity, p.CapacityVPH)
		}
	}
	return capacity, true
}

// network is a flow.Network built from a graph, with arc <-> edge maps.
type network struct {
	f    *flow.Network
	s, t int
	arc  []int                // EdgeID -> arc id, -1 if no arc
	edge map[int]graph.EdgeID // arc id -> EdgeID (road arcs only)
}

// newNetwork builds the flow network. override replaces edge capacities.
// Junctions are unlimited; gates link to a super-source / super-sink.
func newNetwork(g *graph.Graph, sources, sinks []graph.NodeID, override map[graph.EdgeID]int) (*network, error) {
	if len(sources) == 0 || len(sinks) == 0 {
		return nil, fmt.Errorf("fixes: need at least one source and one sink")
	}
	ids := make([]graph.NodeID, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	idx := make(map[graph.NodeID]int, len(ids))
	for i, id := range ids {
		idx[id] = i
	}
	n := &network{f: flow.New(len(ids) + 2), s: len(ids), t: len(ids) + 1,
		arc: make([]int, len(g.Edges)), edge: map[int]graph.EdgeID{}}
	for i, e := range g.Edges {
		n.arc[i] = -1
		u, okU := idx[e.From]
		v, okV := idx[e.To]
		if !okU || !okV {
			return nil, fmt.Errorf("fixes: edge %d references unknown node", e.ID)
		}
		c := e.CapacityVPH
		if o, ok := override[e.ID]; ok {
			c = o
		}
		if c <= 0 {
			continue
		}
		id := n.f.AddArc(u, v, c)
		n.arc[i] = id
		n.edge[id] = e.ID
	}
	for _, s := range sources {
		u, ok := idx[s]
		if !ok {
			return nil, fmt.Errorf("fixes: source node %d not in graph", s)
		}
		n.f.AddArc(n.s, u, flow.Inf)
	}
	for _, t := range sinks {
		v, ok := idx[t]
		if !ok {
			return nil, fmt.Errorf("fixes: sink node %d not in graph", t)
		}
		n.f.AddArc(v, n.t, flow.Inf)
	}
	return n, nil
}

// edges maps arc ids to road edges, dropping gate arcs.
func (n *network) edges(arcs []int) []graph.EdgeID {
	out := []graph.EdgeID{}
	for _, id := range arcs {
		if e, ok := n.edge[id]; ok {
			out = append(out, e)
		}
	}
	return out
}
