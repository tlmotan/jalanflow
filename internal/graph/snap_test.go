package graph

import "testing"

// Dual carriageway running east: north side eastbound (1->2), south side
// westbound (4->3), and a side street (5->6) nearer the gate than either.
func snapGraph() *Graph {
	nodes := map[NodeID]LatLon{
		1: {3.0002, 101.000}, 2: {3.0002, 101.010},
		3: {2.9998, 101.000}, 4: {2.9998, 101.010},
		5: {3.0000, 101.0001}, 6: {3.0010, 101.0001},
		8: {3.0002, 101.020},
	}
	g := &Graph{Nodes: map[NodeID]*Node{}}
	for id, p := range nodes {
		g.Nodes[id] = &Node{ID: id, Pos: p}
	}
	add := func(from, to NodeID, class HighwayClass) {
		g.Edges = append(g.Edges, Edge{ID: EdgeID(len(g.Edges)), From: from, To: to, Class: class, CapacityVPH: 6000})
	}
	add(2, 8, "motorway") // runs on past the east gate
	add(1, 2, "motorway")
	add(4, 3, "motorway")
	add(5, 6, "residential")
	return g
}

func TestSnapGate(t *testing.T) {
	g := snapGraph()
	west, east := LatLon{3.0, 101.0}, LatLon{3.0, 101.02}
	main := []string{"motorway"}
	tests := []struct {
		name string
		pos  LatLon
		far  LatLon
		end  GateEnd
		want NodeID
	}{
		{"west source takes eastbound start, not side street", west, east, SourceEnd, 1},
		{"west sink takes westbound end", west, east, SinkEnd, 3},
		// Node 2 ends eastbound 1->2 by the east gate; 2->8 runs on past
		// it. The sink is node 2, where the road arrives at the gate.
		{"sink takes the road arriving at the gate", LatLon{3.0002, 101.010}, west, SinkEnd, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SnapGate(g, tc.pos, tc.far, tc.end, main, 500)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("snapped to %d, want %d", got, tc.want)
			}
		})
	}
	if _, err := SnapGate(g, west, east, SourceEnd, main, 5); err == nil {
		t.Error("want error when nothing within max distance")
	}
}
