package flow

import (
	"math/rand/v2"
	"testing"
)

type arcSpec struct{ u, v, c int }

func TestMaxFlowAndMinCut(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		arcs    []arcSpec
		s, t    int
		want    int
		wantCut []int // arc ids
	}{
		{
			name: "single pinch: 4 lanes -> 2 lanes -> 4 lanes",
			n:    4,
			arcs: []arcSpec{{0, 1, 4000}, {1, 2, 2000}, {2, 3, 4000}},
			s:    0, t: 3, want: 2000, wantCut: []int{1},
		},
		{
			name: "two parallel roads",
			n:    3,
			arcs: []arcSpec{{0, 1, 3000}, {0, 1, 1500}, {1, 2, 9000}},
			s:    0, t: 2, want: 4500, wantCut: []int{0, 1},
		},
		{
			name: "CLRS classic",
			n:    6,
			arcs: []arcSpec{
				{0, 1, 16}, {0, 2, 13}, {2, 1, 4}, {1, 3, 12}, {3, 2, 9},
				{2, 4, 14}, {4, 3, 7}, {3, 5, 20}, {4, 5, 4},
			},
			s: 0, t: 5, want: 23,
		},
		{
			name: "disconnected",
			n:    3,
			arcs: []arcSpec{{0, 1, 10}},
			s:    0, t: 2, want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := New(tc.n)
			caps := make([]int, len(tc.arcs))
			for i, a := range tc.arcs {
				g.AddArc(a.u, a.v, a.c)
				caps[i] = a.c
			}
			got := g.MaxFlow(tc.s, tc.t)
			if got != tc.want {
				t.Fatalf("max flow = %d, want %d", got, tc.want)
			}
			cut := g.MinCut(tc.s)
			sum := 0
			for _, id := range cut {
				sum += caps[id]
			}
			if sum != got {
				t.Fatalf("cut capacity %d != max flow %d (cut %v)", sum, got, cut)
			}
			if tc.wantCut != nil && !equal(cut, tc.wantCut) {
				t.Fatalf("cut = %v, want %v", cut, tc.wantCut)
			}
		})
	}
}

func TestInEveryMinCut(t *testing.T) {
	tests := []struct {
		name string
		n    int
		arcs []arcSpec
		s, t int
		want []int
	}{
		{
			name: "single pinch is in every cut",
			n:    4,
			arcs: []arcSpec{{0, 1, 4000}, {1, 2, 2000}, {2, 3, 4000}},
			s:    0, t: 3, want: []int{1},
		},
		{
			name: "two equal pinches in series: neither alone helps",
			n:    4,
			arcs: []arcSpec{{0, 1, 2000}, {1, 2, 4000}, {2, 3, 2000}},
			s:    0, t: 3, want: nil,
		},
		{
			name: "two parallel roads both in every cut",
			n:    3,
			arcs: []arcSpec{{0, 1, 3000}, {0, 1, 1500}, {1, 2, 9000}},
			s:    0, t: 2, want: []int{0, 1},
		},
		{
			name: "parallel pinch then equal single pinch",
			n:    4,
			arcs: []arcSpec{{0, 1, 1000}, {0, 1, 1000}, {1, 2, 2000}, {2, 3, 5000}},
			s:    0, t: 3, want: nil,
		},
		{
			name: "disconnected: no cut arc can help",
			n:    3,
			arcs: []arcSpec{{0, 1, 10}},
			s:    0, t: 2, want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := New(tc.n)
			for _, a := range tc.arcs {
				g.AddArc(a.u, a.v, a.c)
			}
			g.MaxFlow(tc.s, tc.t)
			if got := g.InEveryMinCut(tc.s, tc.t); !equal(got, tc.want) {
				t.Fatalf("InEveryMinCut = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestInEveryMinCutPredictsGain checks the Picard-Queyranne property on
// random graphs: raising one arc's capacity increases max flow exactly when
// that arc is in every min cut.
func TestInEveryMinCutPredictsGain(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for trial := 0; trial < 300; trial++ {
		n := 3 + rng.IntN(5)
		var arcs []arcSpec
		for i := 0; i < n*2; i++ {
			u, v := rng.IntN(n), rng.IntN(n)
			if u != v {
				arcs = append(arcs, arcSpec{u, v, 1 + rng.IntN(4)})
			}
		}
		s, sink := 0, n-1
		base := buildFlow(n, arcs, -1)
		before := base.MaxFlow(s, sink)
		every := map[int]bool{}
		for _, id := range base.InEveryMinCut(s, sink) {
			every[id] = true
		}
		for id := range arcs {
			after := buildFlow(n, arcs, id).MaxFlow(s, sink)
			if gained := after > before; gained != every[id] {
				t.Fatalf("trial %d arc %d %v: gained=%v, InEveryMinCut=%v (flow %d -> %d)",
					trial, id, arcs[id], gained, every[id], before, after)
			}
		}
	}
}

// buildFlow builds a network from arcs, adding 1 to arc bump (-1 for none).
func buildFlow(n int, arcs []arcSpec, bump int) *Network {
	g := New(n)
	for i, a := range arcs {
		c := a.c
		if i == bump {
			c++
		}
		g.AddArc(a.u, a.v, c)
	}
	return g
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMaxFlowTrace(t *testing.T) {
	tests := []struct {
		name        string
		n           int
		arcs        []arcSpec
		s, t        int
		want        int
		wantPhases  int
		wantReverse bool // some path cancels flow
	}{
		{
			name: "single pinch",
			n:    4,
			arcs: []arcSpec{{0, 1, 4000}, {1, 2, 2000}, {2, 3, 4000}},
			s:    0, t: 3, want: 2000, wantPhases: 1,
		},
		{
			name: "CLRS classic",
			n:    6,
			arcs: []arcSpec{
				{0, 1, 16}, {0, 2, 13}, {2, 1, 4}, {1, 3, 12}, {3, 2, 9},
				{2, 4, 14}, {4, 3, 7}, {3, 5, 20}, {4, 5, 4},
			},
			s: 0, t: 5, want: 23,
		},
		{
			// s=0 a=1 b=2 c=3 d=4 t=5. Phase 1 sends s-a-b-t. Phase 2's
			// only route is s-c-b-a-d-t, which cancels the flow on a->b.
			name: "second phase cancels a->b",
			n:    6,
			arcs: []arcSpec{{0, 1, 1}, {1, 2, 1}, {2, 5, 1}, {0, 3, 1}, {3, 2, 1}, {1, 4, 1}, {4, 5, 1}},
			s:    0, t: 5, want: 2, wantPhases: 2, wantReverse: true,
		},
		{
			name: "disconnected",
			n:    3,
			arcs: []arcSpec{{0, 1, 10}},
			s:    0, t: 2, want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, plain := New(tc.n), New(tc.n)
			for _, a := range tc.arcs {
				g.AddArc(a.u, a.v, a.c)
				plain.AddArc(a.u, a.v, a.c)
			}
			got, trace := g.MaxFlowTrace(tc.s, tc.t)
			if got != tc.want {
				t.Fatalf("max flow = %d, want %d", got, tc.want)
			}
			if p := plain.MaxFlow(tc.s, tc.t); p != got {
				t.Fatalf("MaxFlow = %d, MaxFlowTrace = %d", p, got)
			}
			sum, phases, reverse := 0, 0, false
			flow := make([]int, len(tc.arcs))
			for i, a := range trace {
				if a.Amount <= 0 || len(a.Arcs) == 0 || len(a.Arcs) != len(a.Reverse) {
					t.Fatalf("path %d malformed: %+v", i, a)
				}
				if a.Phase < phases {
					t.Fatalf("path %d phase %d after phase %d", i, a.Phase, phases)
				}
				phases = a.Phase
				sum += a.Amount
				// The path must be connected from s to t.
				at := tc.s
				for j, id := range a.Arcs {
					from, to := tc.arcs[id].u, tc.arcs[id].v
					if a.Reverse[j] {
						from, to = to, from
						flow[id] -= a.Amount
						reverse = true
					} else {
						flow[id] += a.Amount
					}
					if from != at {
						t.Fatalf("path %d breaks at arc %d", i, id)
					}
					at = to
				}
				if at != tc.t {
					t.Fatalf("path %d ends at %d, want %d", i, at, tc.t)
				}
			}
			if sum != got {
				t.Fatalf("sum of amounts %d != max flow %d", sum, got)
			}
			for id := range tc.arcs {
				if flow[id] != g.Flow(id) {
					t.Fatalf("replayed flow on arc %d = %d, Flow = %d", id, flow[id], g.Flow(id))
				}
			}
			if tc.wantPhases > 0 && phases != tc.wantPhases {
				t.Fatalf("phases = %d, want %d", phases, tc.wantPhases)
			}
			if tc.wantReverse && !reverse {
				t.Fatalf("expected a path that cancels flow")
			}
		})
	}
}
