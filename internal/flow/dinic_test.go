package flow

import "testing"

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
