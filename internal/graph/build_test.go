package graph

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/osm"
)

// Test-only capacity numbers, chosen to make arithmetic easy.
var testCfg = &config.Capacity{
	Classes: map[string]config.ClassDefaults{
		"trunk":   {PerLaneVPH: 1000, DefaultLanes: 2},
		"primary": {PerLaneVPH: 500, DefaultLanes: 1},
	},
	SignalFactor: 0.5,
}

type way struct {
	id    int64
	nodes []int64
	tags  map[string]string
}

// data places node i at (3.0, 101 + i/1000) so lengths are positive and
// distinct; signals lists traffic-signal nodes.
func data(ways []way, signals ...int64) *osm.Data {
	d := &osm.Data{Nodes: map[int64]osm.Node{}}
	sig := map[int64]bool{}
	for _, s := range signals {
		sig[s] = true
	}
	for _, w := range ways {
		d.Ways = append(d.Ways, osm.Way{ID: w.id, Nodes: w.nodes, Tags: w.tags})
		for _, n := range w.nodes {
			d.Nodes[n] = osm.Node{ID: n, Lat: 3.0, Lon: 101 + float64(n)/1000, Signals: sig[n]}
		}
	}
	return d
}

func t2(kv ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// arc is the comparable shape of an edge for table tests.
type arc struct {
	From, To NodeID
	Lanes    int
	Inferred bool
	Cap      int
	Ways     []int64
	Parts    int
}

func arcs(g *Graph) []arc {
	var r []arc
	for _, e := range g.Edges {
		r = append(r, arc{e.From, e.To, e.Lanes, e.LanesInferred, e.CapacityVPH, e.OSMWayIDs, len(e.Parts)})
	}
	return r
}

func TestBuild(t *testing.T) {
	trunk := func(kv ...string) map[string]string {
		return t2(append([]string{"highway", "trunk", "name", "Lebuhraya Shah Alam"}, kv...)...)
	}
	tests := []struct {
		name    string
		ways    []way
		signals []int64
		want    []arc
	}{
		{
			name: "two-way splits lanes and gets both arcs",
			ways: []way{{10, []int64{1, 2, 3}, trunk("lanes", "6")}},
			want: []arc{
				{1, 3, 3, false, 3000, []int64{10}, 1},
				{3, 1, 3, false, 3000, []int64{10}, 1},
			},
		},
		{
			name: "oneway=yes forward only",
			ways: []way{{10, []int64{1, 2}, trunk("oneway", "yes", "lanes", "3")}},
			want: []arc{{1, 2, 3, false, 3000, []int64{10}, 1}},
		},
		{
			name: "oneway=-1 backward only",
			ways: []way{{10, []int64{1, 2}, trunk("oneway", "-1", "lanes", "3")}},
			want: []arc{{2, 1, 3, false, 3000, []int64{10}, 1}},
		},
		{
			name: "untagged lanes use class default and are flagged",
			ways: []way{{10, []int64{1, 2}, t2("highway", "primary_link", "oneway", "yes")}},
			want: []arc{{1, 2, 1, true, 500, []int64{10}, 1}},
		},
		{
			name: "T junction is not merged",
			ways: []way{
				{10, []int64{1, 2, 3}, trunk("oneway", "yes", "lanes", "2")},
				{20, []int64{2, 4}, trunk("oneway", "yes", "lanes", "1")},
			},
			want: []arc{
				{1, 2, 2, false, 2000, []int64{10}, 1},
				{2, 3, 2, false, 2000, []int64{10}, 1},
				{2, 4, 1, false, 1000, []int64{20}, 1},
			},
		},
		{
			// The REVIEW.md #7 test: a 2-lane pinch inside a 4-lane road
			// must survive merging. Capacity is the MIN, never the average.
			name: "pinch survives merging (min, not average)",
			ways: []way{
				{10, []int64{1, 2}, trunk("lanes", "8")},
				{20, []int64{2, 3}, trunk("lanes", "4")},
				{30, []int64{3, 4}, trunk("lanes", "8")},
			},
			want: []arc{
				{1, 4, 2, false, 2000, []int64{10, 20, 30}, 3},
				{4, 1, 2, false, 2000, []int64{30, 20, 10}, 3},
			},
		},
		{
			name: "bridge in the middle blocks merging",
			ways: []way{
				{10, []int64{1, 2}, trunk("oneway", "yes", "lanes", "2")},
				{20, []int64{2, 3}, trunk("oneway", "yes", "lanes", "2", "bridge", "yes")},
				{30, []int64{3, 4}, trunk("oneway", "yes", "lanes", "2")},
			},
			want: []arc{
				{1, 2, 2, false, 2000, []int64{10}, 1},
				{2, 3, 2, false, 2000, []int64{20}, 1},
				{3, 4, 2, false, 2000, []int64{30}, 1},
			},
		},
		{
			name:    "mid-way signal splits, then merge keeps the signalised minimum",
			ways:    []way{{10, []int64{1, 2, 3}, trunk("oneway", "yes", "lanes", "2")}},
			signals: []int64{2},
			want:    []arc{{1, 3, 2, false, 1000, []int64{10}, 2}},
		},
		{
			name: "closed way with no junction is dropped",
			ways: []way{
				{10, []int64{1, 2, 3, 1}, trunk("oneway", "yes")},
			},
			want: nil,
		},
		{
			name: "oneway chain into two-way road is a junction",
			ways: []way{
				{10, []int64{1, 2}, trunk("oneway", "yes", "lanes", "2")},
				{20, []int64{2, 3}, trunk("lanes", "4")},
			},
			want: []arc{
				{1, 2, 2, false, 2000, []int64{10}, 1},
				{2, 3, 2, false, 2000, []int64{20}, 1},
				{3, 2, 2, false, 2000, []int64{20}, 1},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := Build(data(tc.ways, tc.signals...), testCfg, "test")
			if err != nil {
				t.Fatal(err)
			}
			if got := arcs(g); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("edges:\n got %+v\nwant %+v", got, tc.want)
			}
			for i, e := range g.Edges {
				if e.ID != EdgeID(i) {
					t.Errorf("edge %d has ID %d", i, e.ID)
				}
				for _, n := range []NodeID{e.From, e.To} {
					if g.Nodes[n] == nil {
						t.Errorf("edge %d endpoint %d missing from Nodes", i, n)
					}
				}
			}
		})
	}
}

func TestBuildMergedGeometry(t *testing.T) {
	tr := t2("highway", "trunk", "oneway", "yes")
	d := data([]way{{10, []int64{1, 2}, tr}, {20, []int64{2, 3, 4}, tr}})
	g, err := Build(d, testCfg, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(g.Edges))
	}
	e := g.Edges[0]
	if len(e.Geometry) != 4 {
		t.Errorf("geometry has %d points, want 4 (joint not duplicated)", len(e.Geometry))
	}
	var sum float64
	for _, p := range e.Parts {
		sum += p.LengthM
	}
	if math.Abs(e.LengthM-sum) > 1e-6 || math.Abs(e.LengthM-pathLength(e.Geometry)) > 1e-6 {
		t.Errorf("length %f, parts sum %f, geometry %f", e.LengthM, sum, pathLength(e.Geometry))
	}
	if got := g.Nodes[3]; got != nil {
		t.Error("pass-through node 3 should not be a graph node")
	}
}

func TestBuildBackwardGeometryReversed(t *testing.T) {
	d := data([]way{{10, []int64{1, 2, 3}, t2("highway", "trunk", "oneway", "-1")}})
	g, err := Build(d, testCfg, "test")
	if err != nil {
		t.Fatal(err)
	}
	geom := g.Edges[0].Geometry
	if geom[0] != (LatLon{3.0, 101.003}) || geom[2] != (LatLon{3.0, 101.001}) {
		t.Fatalf("backward geometry not reversed: %v", geom)
	}
}

func TestBuildErrors(t *testing.T) {
	d := data([]way{{10, []int64{1, 2}, t2("highway", "motorway")}})
	if _, err := Build(d, testCfg, "test"); err == nil || !strings.Contains(err.Error(), "motorway") {
		t.Fatalf("err = %v, want unknown class error", err)
	}
	if _, err := Build(nil, testCfg, "test"); err == nil {
		t.Fatal("nil data: want error")
	}
}

func TestBuildOSMDate(t *testing.T) {
	d := data([]way{{10, []int64{1, 2}, t2("highway", "trunk")}})
	d.Timestamp = time.Date(2026, 11, 20, 21, 0, 0, 0, time.UTC)
	g, err := Build(d, testCfg, "kemuning-pj")
	if err != nil {
		t.Fatal(err)
	}
	if g.OSMDate != "2026-11-20" || g.Corridor != "kemuning-pj" {
		t.Fatalf("got date %q corridor %q", g.OSMDate, g.Corridor)
	}
}

func TestHaversine(t *testing.T) {
	// One degree of latitude on the mean sphere = R·π/180.
	got := haversineM(LatLon{0, 0}, LatLon{1, 0})
	want := earthRadiusM * math.Pi / 180
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("got %f, want %f", got, want)
	}
}

func TestBuildTollBoothsAndPartEnd(t *testing.T) {
	d := data([]way{
		{1, []int64{1, 2, 3}, t2("highway", "trunk", "oneway", "yes", "lanes", "3")},
		{2, []int64{3, 4}, t2("highway", "trunk", "oneway", "yes", "lanes", "2")},
	})
	n := d.Nodes[2]
	n.TollBooth = true
	d.Nodes[2] = n
	g, err := Build(d, testCfg, "t")
	if err != nil {
		t.Fatal(err)
	}
	if want := []LatLon{{Lat: 3.0, Lon: 101.002}}; !reflect.DeepEqual(g.TollBooths, want) {
		t.Errorf("TollBooths = %v, want %v", g.TollBooths, want)
	}
	if len(g.Edges) != 1 || len(g.Edges[0].Parts) != 2 {
		t.Fatalf("want 1 merged edge with 2 parts, got %+v", g.Edges)
	}
	// Part 1 (3 lanes) ends at node 3, where the 2-lane part begins.
	if got, want := g.Edges[0].Parts[0].End, (LatLon{Lat: 3.0, Lon: 101.003}); got != want {
		t.Errorf("Parts[0].End = %v, want %v", got, want)
	}
}
