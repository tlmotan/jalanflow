package lanedrop

import (
	"strings"
	"testing"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/graph"
)

// Test-only settings.
var testCfg = config.LaneDrops{
	RouteRefs:        []string{"E5"},
	Classes:          []string{"motorway", "motorway_link"},
	TollPlazaRadiusM: 100,
	Source:           "test",
}

// pos places node i at (3.0, 101 + i/100): about 1.1 km apart.
func pos(i int) graph.LatLon { return graph.LatLon{Lat: 3.0, Lon: 101 + float64(i)/100} }

// part is one OSM way piece: way id and lanes. A negative lane count means
// the count is inferred.
type part struct {
	way   int64
	lanes int
}

func edge(from, to int, ref, class string, parts ...part) graph.Edge {
	e := graph.Edge{From: graph.NodeID(from), To: graph.NodeID(to), Ref: ref, Class: graph.HighwayClass(class)}
	for i, p := range parts {
		s := graph.Segment{OSMWayID: p.way, Lanes: p.lanes}
		if p.lanes < 0 {
			s.Lanes, s.LanesInferred = -p.lanes, true
		}
		// Parts split the edge evenly; the last one ends at To.
		f := float64(i+1) / float64(len(parts))
		s.End = graph.LatLon{Lat: 3.0, Lon: pos(from).Lon + f*(pos(to).Lon-pos(from).Lon)}
		e.Parts = append(e.Parts, s)
	}
	return e
}

func build(signals []int, booths []graph.LatLon, edges ...graph.Edge) *graph.Graph {
	g := &graph.Graph{Nodes: map[graph.NodeID]*graph.Node{}, Edges: edges, TollBooths: booths}
	sig := map[int]bool{}
	for _, s := range signals {
		sig[s] = true
	}
	for i := range g.Edges {
		g.Edges[i].ID = graph.EdgeID(i)
		for _, id := range []graph.NodeID{g.Edges[i].From, g.Edges[i].To} {
			g.Nodes[id] = &graph.Node{ID: id, Pos: pos(int(id)), Signals: sig[int(id)]}
		}
	}
	return g
}

// sriMuda: ramp (2) and E5 (3) meet at node 3 and carry on as E5 (4).
func sriMuda(signals []int, booths []graph.LatLon, mainRef, rampClass string) *graph.Graph {
	return build(signals, booths,
		edge(1, 3, "", rampClass, part{10, 2}),
		edge(2, 3, mainRef, "motorway", part{20, 3}),
		edge(3, 4, mainRef, "motorway", part{30, 4}),
	)
}

type got struct {
	key      string
	kind     Kind
	in, out  int
	inferred bool
}

func TestFind(t *testing.T) {
	tests := []struct {
		name string
		g    *graph.Graph
		want []got
	}{
		{"merge loses a lane", sriMuda(nil, nil, "E5", "motorway_link"),
			[]got{{"node:3", Merge, 5, 4, false}}},
		{"balanced merge", build(nil, nil,
			edge(1, 3, "", "motorway_link", part{10, 2}),
			edge(2, 3, "E5", "motorway", part{20, 3}),
			edge(3, 4, "E5", "motorway", part{30, 5})), nil},
		{"signals: lanes take turns", sriMuda([]int{3}, nil, "E5", "motorway_link"), nil},
		{"near toll booth", sriMuda(nil, []graph.LatLon{{Lat: 3.0, Lon: 101.0305}}, "E5", "motorway_link"), nil},
		{"toll booth far away", sriMuda(nil, []graph.LatLon{{Lat: 3.0, Lon: 101.5}}, "E5", "motorway_link"),
			[]got{{"node:3", Merge, 5, 4, false}}},
		{"off route", sriMuda(nil, nil, "E1", "motorway_link"), nil},
		{"surface road at junction", sriMuda(nil, nil, "E5", "primary"), nil},
		{"ref list contains route", sriMuda(nil, nil, "E1;E5", "motorway_link"),
			[]got{{"node:3", Merge, 5, 4, false}}},
		{"merge counts lanes at the node, not the edge minimum", build(nil, nil,
			edge(1, 3, "", "motorway_link", part{10, 1}, part{11, 2}),
			edge(2, 3, "E5", "motorway", part{20, 3}),
			edge(3, 4, "E5", "motorway", part{30, 4}, part{31, 4})),
			// The ramp widens 1 → 2 before the node; 2 + 3 arrive, 4 leave.
			// Using the edge minimum (1) would hide the drop.
			[]got{{"node:3", Merge, 5, 4, false}}},
		{"narrowing on route", build(nil, nil,
			edge(1, 2, "E5", "motorway", part{10, 3}, part{11, 2})),
			[]got{{"way:10>11", Narrowing, 3, 2, false}}},
		{"narrowing on ramp off the route", build(nil, nil,
			edge(1, 2, "E5", "motorway", part{20, 3}),
			edge(2, 3, "", "motorway_link", part{10, 3}, part{11, 2})),
			[]got{{"way:10>11", Narrowing, 3, 2, false}}},
		{"narrowing on ramp far from route", build(nil, nil,
			edge(1, 2, "E1", "motorway", part{20, 3}),
			edge(2, 3, "", "motorway_link", part{10, 3}, part{11, 2})), nil},
		{"widening is not a drop", build(nil, nil,
			edge(1, 2, "E5", "motorway", part{10, 2}, part{11, 3})), nil},
		{"inferred lanes flagged", build(nil, nil,
			edge(1, 2, "E5", "motorway", part{10, 4}, part{11, -3})),
			[]got{{"way:10>11", Narrowing, 4, 3, true}}},
		{"ranked by squeeze then lanes lost", build(nil, nil,
			edge(1, 2, "E5", "motorway", part{10, 4}, part{11, 3}), // 0.75
			edge(3, 4, "E5", "motorway", part{12, 3}, part{13, 1}), // 0.33
			edge(5, 6, "E5", "motorway", part{14, 6}, part{15, 2}), // 0.33, loses 4
		),
			[]got{
				{"way:14>15", Narrowing, 6, 2, false},
				{"way:12>13", Narrowing, 3, 1, false},
				{"way:10>11", Narrowing, 4, 3, false},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drops, err := Find(tt.g, testCfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(drops) != len(tt.want) {
				t.Fatalf("got %d drops %+v, want %d", len(drops), drops, len(tt.want))
			}
			for i, w := range tt.want {
				d := drops[i]
				g := got{d.Key, d.Kind, d.LanesIn, d.LanesOut, d.LanesInferred}
				if g != w {
					t.Errorf("drop %d = %+v, want %+v", i, g, w)
				}
				if d.Rank != i+1 || d.LanesLost != d.LanesIn-d.LanesOut {
					t.Errorf("drop %d: rank %d, lost %d", i, d.Rank, d.LanesLost)
				}
			}
		})
	}
}

func TestFindNarrowingPosition(t *testing.T) {
	g := build(nil, nil, edge(1, 2, "E5", "motorway", part{10, 3}, part{11, 2}))
	drops, err := Find(g, testCfg)
	if err != nil || len(drops) != 1 {
		t.Fatalf("drops %v, err %v", drops, err)
	}
	if want := g.Edges[0].Parts[0].End; drops[0].Pos != want {
		t.Errorf("Pos = %v, want end of first part %v", drops[0].Pos, want)
	}
}

func TestFindConfirmed(t *testing.T) {
	cfg := testCfg
	cfg.Confirmed = []config.ConfirmedDrop{{Key: "node:3", Name: "Sri Muda", Source: "local driver"}}
	drops, err := Find(sriMuda(nil, nil, "E5", "motorway_link"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c := drops[0].Confirmed; c == nil || c.Name != "Sri Muda" || c.Source != "local driver" {
		t.Errorf("Confirmed = %+v", c)
	}

	cfg.Confirmed = []config.ConfirmedDrop{{Key: "node:99", Name: "Gone", Source: "x"}}
	if _, err := Find(sriMuda(nil, nil, "E5", "motorway_link"), cfg); err == nil || !strings.Contains(err.Error(), "node:99") {
		t.Errorf("stale key: err = %v, want error naming node:99", err)
	}
}
