package corridor

import (
	"math"
	"strings"
	"testing"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/ctm"
	"github.com/tlmotan/jalanflow/internal/graph"
)

// fixture is a 3 km E5 mainline 1→2→3→4:
//
//	e0 1→2  1000 m, 2 lanes
//	e1 2→3  1000 m, 2 lanes
//	e2 3→4  1000 m, 3 lanes for 500 m then 2 lanes (lane drop at 2500 m)
//	e3 10→2 on-ramp, way 200        e6 2→4 E10 shortcut (other highway), way 300
//	e4 3→11 off-ramp, way 201       e5 12→3 on-ramp, way 202
//
// Node 2 and node 3 each have a road leaving and a road joining.
func fixture() *graph.Graph {
	g := &graph.Graph{Nodes: map[graph.NodeID]*graph.Node{}}
	for _, id := range []graph.NodeID{1, 2, 3, 4, 10, 11, 12} {
		g.Nodes[id] = &graph.Node{ID: id}
	}
	add := func(from, to graph.NodeID, class graph.HighwayClass, ref string, parts ...graph.Segment) {
		e := graph.Edge{ID: graph.EdgeID(len(g.Edges)), From: from, To: to, Class: class, Ref: ref, Parts: parts, CapacityVPH: 2000}
		for _, p := range parts {
			e.LengthM += p.LengthM
			e.OSMWayIDs = append(e.OSMWayIDs, p.OSMWayID)
		}
		e.Lanes = parts[0].Lanes
		g.Edges = append(g.Edges, e)
	}
	seg := func(way int64, lanes int, m float64) graph.Segment {
		return graph.Segment{OSMWayID: way, Lanes: lanes, LengthM: m}
	}
	add(1, 2, "motorway", "E5", seg(100, 2, 1000))
	add(2, 3, "motorway", "E5", seg(101, 2, 1000))
	add(3, 4, "motorway", "E5", seg(102, 3, 500), seg(103, 2, 500))
	add(10, 2, "motorway_link", "", seg(200, 1, 300))
	add(3, 11, "motorway_link", "", seg(201, 1, 300))
	add(12, 3, "motorway_link", "", seg(202, 1, 300))
	add(2, 4, "motorway", "E10", seg(300, 2, 500))
	return g
}

func capacity() *config.Capacity {
	return &config.Capacity{
		Classes: map[string]config.ClassDefaults{"motorway": {PerLaneVPH: 2000, DefaultLanes: 3, Source: "test"}},
		CTM: map[string]config.CTMClass{"motorway": {
			FreeFlowKMH: 100, CriticalDensity: 20, JamDensity: 150, CapacityDrop: 0.1, Source: "test"}},
	}
}

func demand() *config.Demand {
	return &config.Demand{
		ProfileStepMin: 15,
		Profiles:       map[string]config.Profile{"am": {Start: "06:00", Shape: []float64{0.5, 1, 1, 0.5}, Source: "test"}},
		Entries: []config.DemandEntry{
			{Key: "node:1", Name: "mainline", PeakVPH: 3000, Profile: "am", Source: "test"},
			{Key: "way:200", Name: "ramp at 2", PeakVPH: 800, Profile: "am", Source: "placeholder - guess"},
			{Key: "way:999", Name: "elsewhere", PeakVPH: 100, Profile: "am", Source: "test"},
		},
		Exits: []config.DemandExit{{Key: "way:201", Name: "exit at 3", SplitRatio: 0.2, Source: "test"}},
	}
}

var opts = Options{
	RouteRefs:   []string{"E5", "E11"},
	MainClasses: []string{"motorway", "trunk"},
	RampClasses: []string{"motorway_link", "trunk_link"},
	DtS:         5,
}

func TestRouteStaysOnRouteRefs(t *testing.T) {
	g := fixture()
	got, err := Route(g, capacity(), 1, 4, opts)
	if err != nil {
		t.Fatal(err)
	}
	want := []graph.EdgeID{0, 1, 2} // not the shorter E10 edge 2→4
	if len(got) != len(want) {
		t.Fatalf("route %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("route %v, want %v", got, want)
		}
	}
	if _, err := Route(g, capacity(), 4, 1, opts); err == nil {
		t.Fatal("want error: no route against the one-way mainline")
	}
	// A mainline stretch with no ref (OSM tagging gap) may be used, and Build
	// reports it.
	g.Edges[1].Ref = ""
	if got, err := Route(g, capacity(), 1, 4, opts); err != nil || len(got) != 3 {
		t.Fatalf("route across an unreffed stretch: %v, %v", got, err)
	}
	m, err := Build(g, capacity(), nil, got, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(m.Gaps, "\n"), "no ref tag in OSM (way 101") {
		t.Fatalf("unreffed stretch not reported: %v", m.Gaps)
	}
}

func build(t *testing.T) *Model {
	t.Helper()
	g := fixture()
	m, err := Build(g, capacity(), demand(), []graph.EdgeID{0, 1, 2}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildCells(t *testing.T) {
	m := build(t)
	minM := 100 / 3.6 * opts.DtS // 138.9 m: CFL at 100 km/h
	total := 0.0
	for k, c := range m.Mainline {
		info := m.Cells[c]
		if info.Kind != Mainline {
			t.Fatalf("mainline cell %d has kind %s", c, info.Kind)
		}
		if math.Abs(info.StartM-total) > 1e-6 {
			t.Fatalf("mainline cell %d starts at %.1f, want %.1f (gap or overlap)", k, info.StartM, total)
		}
		if info.LengthM < minM-1e-9 {
			t.Fatalf("cell %d is %.1f m, shorter than one step at free speed (%.1f m)", c, info.LengthM, minM)
		}
		total += info.LengthM
	}
	if math.Abs(total-3000) > 1e-6 {
		t.Fatalf("mainline cells add to %.1f m, want 3000", total)
	}
	// Lanes follow the OSM parts, including the drop inside edge e2.
	tests := []struct {
		atM   float64
		lanes int
	}{{500, 2}, {1500, 2}, {2250, 3}, {2750, 2}}
	for _, tc := range tests {
		c := cellAt(m, tc.atM)
		if got := m.Cells[c].Lanes; got != tc.lanes {
			t.Errorf("at %.0f m: %d lanes, want %d", tc.atM, got, tc.lanes)
		}
	}
	if m.ProfileStart != "06:00" || m.MainKey != "node:1" {
		t.Errorf("profile start %q main key %q", m.ProfileStart, m.MainKey)
	}
}

func cellAt(m *Model, x float64) int {
	for _, c := range m.Mainline {
		if i := m.Cells[c]; i.StartM <= x && x < i.StartM+i.LengthM {
			return c
		}
	}
	return -1
}

// nodeFeeding returns the ctm node whose Out contains cell c.
func nodeFeeding(net ctm.Network, c int) *ctm.Node {
	for i := range net.Nodes {
		for _, o := range net.Nodes[i].Out {
			if o == c {
				return &net.Nodes[i]
			}
		}
	}
	return nil
}

func TestBuildRampsAndNodes(t *testing.T) {
	m := build(t)
	g := fixture()
	minM := 100 / 3.6 * opts.DtS
	type want struct {
		kind  Kind
		edge  graph.EdgeID
		atM   float64
		key   string
		peak  float64
		split float64
	}
	wants := []want{
		{OffRamp, 6, 1000, "", 0, 0},          // E10 shortcut: no split configured
		{OnRamp, 3, 1000, "way:200", 800, 0},  // joins one cell after the diverge at node 2
		{OffRamp, 4, 2000, "way:201", 0, 0.2}, // exit at node 3
		{OnRamp, 5, 2000, "", 0, 0},           // no demand configured
	}
	if len(m.Ramps) != len(wants) {
		t.Fatalf("got %d ramps, want %d: %+v", len(m.Ramps), len(wants), m.Ramps)
	}
	for i, w := range wants {
		r := m.Ramps[i]
		if r.Kind != w.kind || r.EdgeID != w.edge || r.AtM != w.atM || r.DemandKey != w.key || r.PeakVPH != w.peak || r.Split != w.split {
			t.Errorf("ramp %d = %+v, want %+v", i, r, w)
		}
		// Cells are ≥ 138.9 m: the 300 m ramps get 2, the 500 m E10 edge 3.
		if wantCells := int(g.Edges[w.edge].LengthM / minM); len(r.Cells) != wantCells {
			t.Errorf("ramp %d has %d cells, want %d", i, len(r.Cells), wantCells)
		}
	}

	// Diverge at node 3 (2000 m): mainline keeps 0.8, exit takes 0.2.
	afterDiverge := cellAt(m, 2000+1)
	nd := nodeFeeding(m.Net, afterDiverge)
	if nd == nil || len(nd.Out) != 2 || nd.Out[1] != m.Ramps[2].Cells[0] ||
		math.Abs(nd.Split[0]-0.8) > 1e-9 || math.Abs(nd.Split[1]-0.2) > 1e-9 {
		t.Fatalf("diverge at 2000 m = %+v", nd)
	}
	// Merge one cell later, with capacity drop.
	afterMerge := cellAt(m, 2000+minM+1)
	nd = nodeFeeding(m.Net, afterMerge)
	if nd == nil || len(nd.In) != 2 || nd.In[1] != m.Ramps[3].Cells[1] || nd.CapDrop != 0.1 {
		t.Fatalf("merge after node 3 = %+v", nd)
	}
	if got := m.Cells[nd.In[0]].StartM + m.Cells[nd.In[0]].LengthM; math.Abs(got-(2000+minM)) > 1e-6 {
		t.Fatalf("merge after node 3 at %.1f m, want %.1f", got, 2000+minM)
	}
	// Lane drop 3 → 2 at 2500 m gets capacity drop; the 2 → 3 widening does not.
	if nd := nodeFeeding(m.Net, cellAt(m, 2500+1)); nd == nil || nd.CapDrop != 0.1 {
		t.Fatalf("lane drop node = %+v, want CapDrop 0.1", nd)
	}
	if nd := nodeFeeding(m.Net, cellAt(m, 2000+minM+1)+1); nd == nil || nd.CapDrop != 0 {
		t.Fatalf("plain node = %+v, want CapDrop 0", nd)
	}
	// On-ramp demand = profile × peak; the unmatched on-ramp has no origin.
	var rampOrigin *ctm.Origin
	for i := range m.Net.Origins {
		if m.Net.Origins[i].Cell == m.Ramps[1].Cells[0] {
			rampOrigin = &m.Net.Origins[i]
		}
		if m.Net.Origins[i].Cell == m.Ramps[3].Cells[0] {
			t.Fatal("unmatched on-ramp should have no origin")
		}
	}
	if rampOrigin == nil || rampOrigin.IntervalS != 900 || rampOrigin.DemandVph[1] != 800 || rampOrigin.DemandVph[0] != 400 {
		t.Fatalf("ramp origin = %+v", rampOrigin)
	}
}

func TestBuildReportsGaps(t *testing.T) {
	m := build(t)
	all := strings.Join(m.Gaps, "\n")
	for _, want := range []string{
		"no split ratio for off-ramp to E10 (way 300",
		"no demand for on-ramp from unnamed motorway_link (way 202) (way 202, joins at node:3)",
		"on-ramp from unnamed motorway_link (way 200) demand (way:200) is a placeholder",
		"demand entry way:999 (elsewhere) is not on this route",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("gaps missing %q; got:\n%s", want, all)
		}
	}
}

func TestBuiltModelRuns(t *testing.T) {
	m := build(t)
	res, err := ctm.Run(m.Net, 2*720) // 2 h at 5 s
	if err != nil {
		t.Fatal(err)
	}
	inNet := 0.0
	for _, v := range res.Vehicles[len(res.Vehicles)-1] {
		inNet += v
	}
	queued := 0.0
	for _, q := range res.OriginQueue[len(res.OriginQueue)-1] {
		queued += q
	}
	if math.Abs(res.Arrived-(inNet+queued+res.Exited)) > 1e-6 {
		t.Fatalf("not conserved: arrived %.3f, in %.3f queued %.3f exited %.3f", res.Arrived, inNet, queued, res.Exited)
	}
	// 3000 + 800 veh/h at peak into a 2-lane (4000 veh/h) mainline that
	// loses 20% at the exit: no queue should form.
	tt, ok := res.TravelTimeS(m.Net, m.Mainline, 360)
	if !ok || math.Abs(tt-ctm.FreeFlowTimeS(m.Net, m.Mainline)) > 1 {
		t.Fatalf("mid-peak travel time %.1f s (ok=%v), want free flow %.1f s", tt, ok, ctm.FreeFlowTimeS(m.Net, m.Mainline))
	}
}

func TestBuildErrors(t *testing.T) {
	g := fixture()
	if _, err := Build(g, capacity(), demand(), nil, opts); err == nil {
		t.Error("want error for empty route")
	}
	d := demand()
	d.Exits[0].SplitRatio = 1
	if _, err := Build(g, capacity(), d, []graph.EdgeID{0, 1, 2}, opts); err == nil || !strings.Contains(err.Error(), "sum to ≥ 1") {
		t.Errorf("want split error, got %v", err)
	}
	if m, err := Build(g, capacity(), nil, []graph.EdgeID{0, 1, 2}, opts); err != nil || len(m.Net.Origins) != 0 {
		t.Errorf("nil demand should build with no origins, got %v", err)
	}
}
