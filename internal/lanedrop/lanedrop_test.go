package lanedrop

import (
	"math"
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

func TestFindNamesRamps(t *testing.T) {
	named := edge(5, 6, "", "motorway", part{50, 2})
	named.Name = "Persiaran Kewajipan"
	g := build(nil, nil,
		edge(1, 2, "E5", "motorway", part{20, 3}),
		edge(2, 3, "", "motorway_link", part{30, 3}),
		edge(3, 4, "", "motorway_link", part{10, 3}, part{11, 2}), // the drop, two ramps from E5
		edge(4, 5, "E11", "motorway", part{40, 3}),
		named,
	)
	cfg := testCfg
	cfg.RouteRefs = []string{"E5", "E11"}
	drops, err := Find(g, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var d *Drop
	for i := range drops {
		if drops[i].Key == "way:10>11" {
			d = &drops[i]
		}
	}
	if d == nil {
		t.Fatalf("narrowing way:10>11 not found in %+v", drops)
	}
	for _, r := range append(d.From, d.To...) {
		if r.Leaves != "E5" || r.Joins != "E11" {
			t.Errorf("ramp way %d: leaves %q joins %q, want E5 → E11", r.OSMWayID, r.Leaves, r.Joins)
		}
	}

	// A named road is not a ramp: no leaves/joins. Without a ref, a ramp
	// uses the road's name.
	g = build(nil, nil,
		edge(1, 2, "E5", "motorway", part{20, 3}),
		edge(2, 3, "", "motorway_link", part{10, 3}, part{11, 2}),
		named,
		edge(3, 5, "", "motorway_link", part{60, 2}),
	)
	drops, err = Find(g, testCfg)
	if err != nil || len(drops) != 1 {
		t.Fatalf("drops %+v, err %v", drops, err)
	}
	if r := drops[0].To[0]; r.Leaves != "E5" || r.Joins != "Persiaran Kewajipan" {
		t.Errorf("leaves %q joins %q, want E5 → Persiaran Kewajipan", r.Leaves, r.Joins)
	}
}

// at places a point x, y metres east and north of (3.0, 101.0).
func at(x, y float64) graph.LatLon {
	const m = 111_320.0
	return graph.LatLon{Lat: 3.0 + y/m, Lon: 101.0 + x/(m*math.Cos(3.0*math.Pi/180))}
}

// placed builds a graph from edges whose nodes sit at the given points.
// Edges keep any Geometry they carry; without it, sides come from nodes.
func placed(nodes map[int]graph.LatLon, edges ...graph.Edge) *graph.Graph {
	g := &graph.Graph{Nodes: map[graph.NodeID]*graph.Node{}, Edges: edges}
	for i := range g.Edges {
		g.Edges[i].ID = graph.EdgeID(i)
	}
	for id, p := range nodes {
		g.Nodes[graph.NodeID(id)] = &graph.Node{ID: graph.NodeID(id), Pos: p}
	}
	return g
}

func TestFindSides(t *testing.T) {
	// Travel is east through node 3 at (0, 0). Driving on the left, an
	// on-ramp from the north joins on the left.
	east := map[int]graph.LatLon{2: at(-1000, 0), 3: at(0, 0), 4: at(1000, 0)}
	with := func(extra map[int]graph.LatLon) map[int]graph.LatLon {
		m := map[int]graph.LatLon{}
		for k, v := range east {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	main := func() []graph.Edge {
		return []graph.Edge{
			edge(2, 3, "E5", "motorway", part{20, 3}),
			edge(3, 4, "E5", "motorway", part{30, 3}),
		}
	}
	curving := edge(1, 3, "", "motorway_link", part{10, 2})
	// Starts south of the road but swings round to arrive from the north.
	curving.Geometry = []graph.LatLon{at(-800, -300), at(-300, -200), at(-150, 100), at(-60, 40), at(0, 0)}

	tests := []struct {
		name     string
		g        *graph.Graph
		wantFrom map[int64]Side // by OSM way id
	}{
		{"ramp joins from the left", placed(with(map[int]graph.LatLon{1: at(-900, 300)}),
			append(main(), edge(1, 3, "", "motorway_link", part{10, 2}))...),
			map[int64]Side{20: Right, 10: Left}},
		{"ramp joins from the right", placed(with(map[int]graph.LatLon{1: at(-900, -300)}),
			append(main(), edge(1, 3, "", "motorway_link", part{10, 2}))...),
			map[int64]Side{20: Left, 10: Right}},
		{"the last metres decide, not where the ramp starts", placed(with(map[int]graph.LatLon{1: at(-800, -300)}),
			append(main(), curving)...),
			map[int64]Side{20: Right, 10: Left}},
		{"three roads in: left, middle, right", placed(with(map[int]graph.LatLon{1: at(-900, 300), 5: at(-900, -300)}),
			append(main(), edge(1, 3, "", "motorway_link", part{10, 2}), edge(5, 3, "", "motorway_link", part{11, 2}))...),
			map[int64]Side{10: Left, 20: Middle, 11: Right}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drops, err := Find(tt.g, testCfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(drops) != 1 {
				t.Fatalf("got %d drops, want 1", len(drops))
			}
			d := drops[0]
			for _, r := range d.From {
				if r.Side != tt.wantFrom[r.OSMWayID] {
					t.Errorf("way %d: side %q, want %q", r.OSMWayID, r.Side, tt.wantFrom[r.OSMWayID])
				}
			}
			for _, r := range d.To {
				if r.Side != "" {
					t.Errorf("single road out: side %q, want empty", r.Side)
				}
			}
		})
	}
}

func TestFindSidesNarrowingHasNone(t *testing.T) {
	// OSM says a lane ends, not which one: a narrowing gets no side.
	drops, err := Find(build(nil, nil, edge(1, 2, "E5", "motorway", part{10, 3}, part{11, 2})), testCfg)
	if err != nil || len(drops) != 1 {
		t.Fatalf("drops %v, err %v", drops, err)
	}
	if s := drops[0].From[0].Side + drops[0].To[0].Side; s != "" {
		t.Errorf("narrowing sides = %q, want none", s)
	}
}

func TestFindSidesStraightLineIsUnknown(t *testing.T) {
	// Every node on one line (the old fixtures): nothing to tell apart.
	drops, err := Find(sriMuda(nil, nil, "E5", "motorway_link"), testCfg)
	if err != nil || len(drops) != 1 {
		t.Fatalf("drops %v, err %v", drops, err)
	}
	for _, r := range drops[0].From {
		if r.Side != "" {
			t.Errorf("way %d: side %q, want empty", r.OSMWayID, r.Side)
		}
	}
}
