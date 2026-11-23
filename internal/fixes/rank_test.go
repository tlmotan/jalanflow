package fixes

import (
	"testing"

	"github.com/tlmotan/jalanflow/internal/graph"
)

// road is a test edge: from, to, lanes per part (per-lane capacity 2000).
type road struct {
	from, to graph.NodeID
	lanes    []int
}

func testGraph(roads []road) *graph.Graph {
	g := &graph.Graph{Nodes: map[graph.NodeID]*graph.Node{}}
	for i, r := range roads {
		for _, id := range []graph.NodeID{r.from, r.to} {
			g.Nodes[id] = &graph.Node{ID: id}
		}
		e := graph.Edge{ID: graph.EdgeID(i), From: r.from, To: r.to, Class: "trunk"}
		for _, l := range r.lanes {
			e.Parts = append(e.Parts, graph.Segment{Lanes: l, CapacityVPH: l * 2000})
		}
		e.Lanes, e.CapacityVPH = e.Parts[0].Lanes, e.Parts[0].CapacityVPH
		for _, p := range e.Parts {
			if p.CapacityVPH < e.CapacityVPH {
				e.Lanes, e.CapacityVPH = p.Lanes, p.CapacityVPH
			}
		}
		g.Edges = append(g.Edges, e)
	}
	return g
}

func TestRankMarksFixesThatHelp(t *testing.T) {
	type want struct {
		helps bool
		gain  int
		why   string
	}
	tests := []struct {
		name    string
		roads   []road
		maxFlow int
		want    map[graph.EdgeID]want
	}{
		{
			name:    "single pinch: +1 lane helps until the next road caps it",
			roads:   []road{{1, 2, []int{4}}, {2, 3, []int{2}}, {3, 4, []int{3}}},
			maxFlow: 4000,
			want:    map[graph.EdgeID]want{1: {helps: true, gain: 2000}},
		},
		{
			name: "two equal pinches in series: neither alone helps",
			// KESAS 2 lanes, then LDP 2 lanes: widening one leaves the other.
			roads:   []road{{1, 2, []int{2}}, {2, 3, []int{4}}, {3, 4, []int{2}}},
			maxFlow: 4000,
			want:    map[graph.EdgeID]want{0: {why: WhyOtherCut}},
		},
		{
			name:    "pinch is two equal stretches of one road",
			roads:   []road{{1, 2, []int{4}}, {2, 3, []int{2, 3, 2}}, {3, 4, []int{4}}},
			maxFlow: 4000,
			want:    map[graph.EdgeID]want{1: {why: WhyOtherPart}},
		},
		{
			name: "two parallel roads: each helps on its own",
			roads: []road{
				{1, 2, []int{1}}, {1, 2, []int{2}}, {2, 3, []int{6}},
			},
			maxFlow: 6000,
			want: map[graph.EdgeID]want{
				0: {helps: true, gain: 2000},
				1: {helps: true, gain: 2000},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := testGraph(tc.roads)
			last := tc.roads[len(tc.roads)-1].to
			res, err := Rank(g, Inbound, []graph.NodeID{1}, []graph.NodeID{last})
			if err != nil {
				t.Fatal(err)
			}
			if res.MaxFlowVPH != tc.maxFlow {
				t.Fatalf("max flow = %d, want %d", res.MaxFlowVPH, tc.maxFlow)
			}
			if len(res.Fixes) != len(tc.want) {
				t.Fatalf("got %d fixes, want %d: %+v", len(res.Fixes), len(tc.want), res.Fixes)
			}
			for _, f := range res.Fixes {
				w, ok := tc.want[f.EdgeID]
				if !ok {
					t.Fatalf("unexpected fix on edge %d", f.EdgeID)
				}
				if f.Helps != w.helps || f.GainVPH != w.gain || f.WhyNoGain != w.why {
					t.Errorf("edge %d: helps=%v gain=%d why=%q, want helps=%v gain=%d why=%q",
						f.EdgeID, f.Helps, f.GainVPH, f.WhyNoGain, w.helps, w.gain, w.why)
				}
			}
		})
	}
}

// TestNoGainIsReallyZero re-runs max flow for fixes marked Helps=false to
// check the skipped re-run would indeed have gained nothing.
func TestNoGainIsReallyZero(t *testing.T) {
	g := testGraph([]road{
		{1, 2, []int{2}}, {2, 3, []int{4}}, {3, 4, []int{2}},
		{1, 5, []int{1}}, {5, 4, []int{1}},
	})
	res, err := Rank(g, Inbound, []graph.NodeID{1}, []graph.NodeID{4})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Fixes {
		if f.Helps {
			continue
		}
		widened, ok := plusOneLane(g.Edges[f.EdgeID])
		if !ok {
			continue
		}
		n, err := newNetwork(g, []graph.NodeID{1}, []graph.NodeID{4}, map[graph.EdgeID]int{f.EdgeID: widened})
		if err != nil {
			t.Fatal(err)
		}
		if got := n.f.MaxFlow(n.s, n.t); got != res.MaxFlowVPH {
			t.Errorf("edge %d marked no-gain but re-run gives %d > %d", f.EdgeID, got, res.MaxFlowVPH)
		}
	}
}

func TestRankNeedsGates(t *testing.T) {
	g := testGraph([]road{{1, 2, []int{2}}})
	if _, err := Rank(g, Inbound, nil, []graph.NodeID{2}); err == nil {
		t.Error("want error with no sources")
	}
	if _, err := Rank(g, Inbound, []graph.NodeID{9}, []graph.NodeID{2}); err == nil {
		t.Error("want error for unknown source node")
	}
}
