package corridor

import (
	"math"
	"strings"
	"testing"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/ctm"
	"github.com/tlmotan/jalanflow/internal/graph"
)

func estimate() *config.Estimate {
	return &config.Estimate{
		Profile: "fitted", Start: "06:00", StartLevel: 0.5, PeakEnd: "08:00", End: "09:00", EndLevel: 0.5,
		MaxExitSplit: 0.9, MinQueueMatch: 0.8,
		MainPeakVPH: [3]float64{3000, 4500, 500}, RampLoad: [3]float64{0.2, 0.8, 0.2},
		ExitScale: [3]float64{0.3, 0.9, 0.3}, PeakStartMin: [3]float64{30, 60, 30},
		Source: "test",
	}
}

func TestProfile(t *testing.T) {
	p, err := profile(estimate(), 15, 30)
	if err != nil {
		t.Fatal(err)
	}
	// Mid-points of 15-min steps: rise 0.5 → 1 over 30 min, flat to 08:00,
	// fall to 0.5 at 09:00.
	want := []float64{0.625, 0.875, 1, 1, 1, 1, 1, 1, 0.938, 0.813, 0.688, 0.563}
	if len(p.Shape) != len(want) {
		t.Fatalf("shape %v, want %v", p.Shape, want)
	}
	for i := range want {
		if math.Abs(p.Shape[i]-want[i]) > 1e-9 {
			t.Fatalf("shape %v, want %v", p.Shape, want)
		}
	}
	if p.Start != "06:00" || !config.IsEstimated(p.Source) {
		t.Errorf("start %q source %q", p.Start, p.Source)
	}
	if _, err := profile(estimate(), 15, 120); err == nil {
		t.Error("want error: peak start after peak end")
	}
}

func TestEstimateDemand(t *testing.T) {
	g := fixture()
	base := demand() // node:1 and way:201 are counted; way:200 is a placeholder
	base.Estimate = estimate()
	p := Params{MainPeakVPH: 9999, PeakStartMin: 30, Sections: []Section{
		{EndM: 1500, RampLoad: 0.5, ExitScale: 0.6},
		{EndM: 3000, RampLoad: 0.25, ExitScale: 0.9},
	}}
	d, err := EstimateDemand(g, capacity(), base, []graph.EdgeID{0, 1, 2}, opts, p, 2026, "estimated - test")
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]config.DemandEntry{}
	for _, e := range d.Entries {
		entries[e.Key] = e
	}
	exits := map[string]config.DemandExit{}
	for _, x := range d.Exits {
		exits[x.Key] = x
	}
	// Counted figures are untouched; the fitted route-start value is not used.
	if e := entries["node:1"]; e.PeakVPH != 3000 || e.Source != "test" || e.Profile != "am" {
		t.Errorf("counted route start changed: %+v", e)
	}
	if x := exits["way:201"]; x.SplitRatio != 0.2 || x.Source != "test" {
		t.Errorf("counted exit changed: %+v", x)
	}
	// Placeholder on-ramp at 1000 m (section 1): 0.5 × 1 lane × 2000.
	if e := entries["way:200"]; e.PeakVPH != 1000 || !config.IsEstimated(e.Source) || e.Profile != "fitted" {
		t.Errorf("way:200 = %+v, want 1000 veh/h estimated", e)
	}
	// On-ramp at 2000 m (section 2): 0.25 × 2000.
	if e := entries["way:202"]; e.PeakVPH != 500 {
		t.Errorf("way:202 = %+v, want 500 veh/h", e)
	}
	// E10 exit at 1000 m (section 1): 0.6 × 2 lanes ÷ (2 + 2 carrying on).
	if x := exits["way:300"]; math.Abs(x.SplitRatio-0.3) > 1e-9 || !config.IsEstimated(x.Source) {
		t.Errorf("way:300 = %+v, want split 0.3 estimated", x)
	}
	if _, ok := entries["way:999"]; !ok {
		t.Error("counted entry off the route should still be kept: it is real data")
	}
	m, err := Build(g, capacity(), d, []graph.EdgeID{0, 1, 2}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if m.Estimated != 3 {
		t.Errorf("model reports %d estimated figures, want 3", m.Estimated)
	}
	for _, gap := range m.Gaps {
		if strings.Contains(gap, "using 0") || strings.Contains(gap, "placeholder") {
			t.Errorf("gap left after estimating: %s", gap)
		}
	}
}

// Travel times generated from known numbers must be matched exactly when
// those numbers are on the search grid and the queue stays in its section.
func TestFitMatchesKnownDemand(t *testing.T) {
	g := fixture()
	base := &config.Demand{Corridor: "t", ProfileStepMin: 15, Estimate: estimate()}
	route := []graph.EdgeID{0, 1, 2}
	// The queue is in section 1 (4000 × 0.7 + 1600 into a 4000 veh/h road);
	// section 2 flows freely, so it does not reach back into section 1.
	truth := Params{MainPeakVPH: 4000, PeakStartMin: 30, Sections: []Section{
		{EndM: 1800, RampLoad: 0.8, ExitScale: 0.6},
		{EndM: 3000, RampLoad: 0.2, ExitScale: 0.9},
	}}
	d, err := EstimateDemand(g, capacity(), base, route, opts, truth, 2026, "estimated - truth")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Build(g, capacity(), d, route, opts)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ctm.Run(m.Net, m.Steps(3600))
	if err != nil {
		t.Fatal(err)
	}
	var obs []TravelObs
	for _, to := range []float64{1800, 3000} {
		for _, c := range []string{"06:30", "07:00", "07:30", "08:00"} {
			obs = append(obs, TravelObs{Name: "x", Depart: c, FromM: 0, ToM: to})
		}
	}
	fits, err := m.Compare(res, obs)
	if err != nil {
		t.Fatal(err)
	}
	congested := false
	for i, f := range fits {
		obs[i].Minutes = f.SimMinutes
		if f.SimMinutes > 1.5*f.ToM/1000/100*60 {
			congested = true
		}
	}
	if !congested {
		t.Fatal("fixture should be congested, or the fit has nothing to find")
	}
	fit, err := Fit(g, capacity(), base, route, opts, obs, nil, 3600, 2026, "estimated - fit")
	if err != nil {
		t.Fatal(err)
	}
	if fit.ErrPct > 1e-6 {
		t.Fatalf("fit error %.4f%%, want 0; params %+v", fit.ErrPct, fit.Params)
	}
	if fit.UncalibratedFromM != 3000 || len(fit.Params.Sections) != 2 {
		t.Errorf("sections %+v, uncalibrated from %.0f", fit.Params.Sections, fit.UncalibratedFromM)
	}
	if first := 4*2*4*3 + 4*3; fit.Tried < first {
		t.Errorf("tried %d combinations, want at least the first pass (%d)", fit.Tried, first)
	}
}

func TestLocate(t *testing.T) {
	// Two edges heading east along 3.0° N, 0.01° of longitude each.
	g := &graph.Graph{Nodes: map[graph.NodeID]*graph.Node{}}
	pts := []graph.LatLon{{Lat: 3, Lon: 101.00}, {Lat: 3, Lon: 101.01}, {Lat: 3, Lon: 101.02}}
	for i := 0; i < 2; i++ {
		g.Edges = append(g.Edges, graph.Edge{ID: graph.EdgeID(i), Geometry: pts[i : i+2], LengthM: graph.HaversineM(pts[i], pts[i+1])})
	}
	edge := g.Edges[0].LengthM
	tests := []struct {
		name          string
		pos           graph.LatLon
		along, offset float64
	}{
		{"on the second edge, 100 m north of it", graph.LatLon{Lat: 3 + 100/111320.0, Lon: 101.015}, edge * 1.5, 100},
		{"before the start clamps to 0", graph.LatLon{Lat: 3, Lon: 100.99}, 0, edge},
	}
	for _, tc := range tests {
		along, off := Locate(g, []graph.EdgeID{0, 1}, tc.pos)
		if math.Abs(along-tc.along) > 2 || math.Abs(off-tc.offset) > 2 {
			t.Errorf("%s: along %.1f off %.1f, want %.1f and %.1f", tc.name, along, off, tc.along, tc.offset)
		}
	}
}

// Two traffic patterns can give similar travel times; an observed queue
// picks the one with the queue in the right place.
func TestFitPrefersCandidatesThatReproduceQueues(t *testing.T) {
	g := fixture()
	base := &config.Demand{Corridor: "t", ProfileStepMin: 15, Estimate: estimate()}
	route := []graph.EdgeID{0, 1, 2}
	obs := []TravelObs{{Name: "x", Depart: "07:00", FromM: 0, ToM: 3000, Minutes: 1.8}} // free flow
	free, err := Fit(g, capacity(), base, route, opts, obs, nil, 3600, 2026, "estimated - fit")
	if err != nil {
		t.Fatal(err)
	}
	if free.ErrPct > 1 || !free.QueuesOK {
		t.Fatalf("without a queue observation the free-flow match should win: %.1f%%", free.ErrPct)
	}
	queues := []QueueObs{{Name: "q", Clock: "07:00", TailM: 0, HeadM: 1000}}
	fit, err := Fit(g, capacity(), base, route, opts, obs, queues, 3600, 2026, "estimated - fit")
	if err != nil {
		t.Fatal(err)
	}
	if !fit.QueuesOK || fit.Queues[0].Match < 0.8 {
		t.Fatalf("queue not reproduced: %+v", fit.Queues)
	}
	if fit.ErrPct <= free.ErrPct {
		t.Fatalf("reproducing the queue must cost travel-time error here: %.1f%% vs %.1f%%", fit.ErrPct, free.ErrPct)
	}
}
