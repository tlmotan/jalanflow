package corridor

import (
	"math"
	"strings"
	"testing"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/graph"
)

func fixCapacity() *config.Capacity {
	c := capacity()
	c.ALINEA = config.ALINEA{GainVPHPerPct: 70, TargetOccupancyPct: 13, MinRateVPH: 240, UpdateS: 60, Source: "test"}
	c.Channelisation = config.Channelisation{CapacityDropReduction: 0.5, Confidence: "low", Source: "test"}
	c.Restripe = config.Restripe{AuxLaneM: 300, Source: "test"}
	return c
}

// congested is the fixture with 3000 + 1600 veh/h meeting at node 2 on a
// 2-lane (4000 veh/h) road: a queue forms behind that merge.
func congested(t *testing.T) *Model {
	t.Helper()
	d := demand()
	d.Entries[1].PeakVPH = 1600
	d.Profiles["am"] = config.Profile{Start: "06:00", Shape: []float64{1, 1, 1, 1, 0.2, 0.2, 0.2, 0.2}, Source: "test"}
	m, err := Build(fixture(), fixCapacity(), d, []graph.EdgeID{0, 1, 2}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSites(t *testing.T) {
	m := congested(t)
	minM := 100 / 3.6 * opts.DtS
	want := []struct {
		kind             SiteKind
		key              string
		atM              float64
		in, out, rampLns int
	}{
		{MergeSite, "node:2", 1000 + minM, 2, 2, 1},
		{MergeSite, "node:3", 2000 + minM, 3, 3, 1},
		{LaneDropSite, "way:102>103", 2500, 3, 2, 0},
	}
	got := m.Sites()
	if len(got) != len(want) {
		t.Fatalf("got %d sites, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		s := got[i]
		if s.Kind != w.kind || s.Key != w.key || math.Abs(s.AtM-w.atM) > 1e-6 || s.LanesIn != w.in || s.LanesOut != w.out || s.RampLanes != w.rampLns {
			t.Errorf("site %d = %+v, want %+v", i, s, w)
		}
	}
}

func TestApply(t *testing.T) {
	m := congested(t)
	c := fixCapacity()
	sites := m.Sites()
	merge2, merge3, drop := sites[0], sites[1], sites[2]
	lanesAt := func(netCells []float64, x float64) float64 { return netCells[cellAt(m, x)] }
	lanes := func(s Site, ft FixType) ([]float64, bool) {
		net, _, ok := m.apply(c, s, ft)
		out := make([]float64, len(net.Cells))
		for i, cell := range net.Cells {
			out[i] = cell.Lanes
		}
		return out, ok
	}

	// +1 lane at the lane drop: the 2-lane tail becomes 3 lanes and the
	// node stops being a bottleneck.
	l, ok := lanes(drop, PlusLane)
	if !ok || lanesAt(l, 2750) != 3 || lanesAt(l, 2250) != 3 {
		t.Errorf("plus lane at the drop: lanes %v at 2750 m, %v at 2250 m", lanesAt(l, 2750), lanesAt(l, 2250))
	}
	if net, _, _ := m.apply(c, drop, PlusLane); net.Nodes[drop.node].CapDrop != 0 {
		t.Error("lane drop node should lose its capacity drop once widened")
	}
	// +1 lane at merge 2 runs to where the road already widens (2000 m).
	l, _ = lanes(merge2, PlusLane)
	if lanesAt(l, 1500) != 3 || lanesAt(l, 1900) != 3 || lanesAt(l, 2250) != 3 || lanesAt(l, 500) != 2 {
		t.Errorf("plus lane at merge 2: %v %v %v %v", lanesAt(l, 500), lanesAt(l, 1500), lanesAt(l, 1900), lanesAt(l, 2250))
	}
	// Re-stripe: the extra lane lasts about aux_lane_m (whole cells), then
	// a new lane drop appears with a capacity drop; the merge loses its own.
	net, change, ok := m.apply(c, merge2, Restripe)
	if !ok || !strings.Contains(change, "2 → 3 lanes") {
		t.Fatalf("restripe: ok=%v change=%q", ok, change)
	}
	if net.Cells[cellAt(m, 1200)].Lanes != 3 || net.Cells[cellAt(m, 1800)].Lanes != 2 {
		t.Errorf("restripe lanes: %v at 1200 m, %v at 1800 m", net.Cells[cellAt(m, 1200)].Lanes, net.Cells[cellAt(m, 1800)].Lanes)
	}
	if net.Nodes[merge2.node].CapDrop != 0 {
		t.Error("restriped merge should have no capacity drop")
	}
	ends := 0
	for i, nd := range net.Nodes {
		if nd.CapDrop > 0 && m.Net.Nodes[i].CapDrop == 0 {
			ends++
		}
	}
	if ends != 1 {
		t.Errorf("want one new lane drop where the extra lane ends, got %d", ends)
	}
	// Dividers: half the capacity drop, merge shares unchanged.
	net, _, ok = m.apply(c, merge2, Channelise)
	if !ok || net.Nodes[merge2.node].CapDrop != 0.05 || net.Nodes[merge2.node].Priority != nil {
		t.Errorf("channelise: ok=%v node=%+v", ok, net.Nodes[merge2.node])
	}
	// Ramp meter: on the ramp's origin, target and gain converted from
	// occupancy % with the jam density (150): 13 % → 19.5, 70 → 46.67.
	net, _, ok = m.apply(c, merge2, RampMeter)
	var meters int
	for _, o := range net.Origins {
		if o.Meter != nil {
			meters++
			if math.Abs(o.Meter.TargetDensityPerLane-19.5) > 1e-9 || math.Abs(o.Meter.GainVph-70*100/150.0) > 1e-9 ||
				o.Meter.MinVph != 240 || o.Meter.MaxVph != 2000 || o.Meter.UpdateS != 60 || o.Meter.MaxQueueVeh <= 0 {
				t.Errorf("meter = %+v", o.Meter)
			}
		}
	}
	if !ok || meters != 1 {
		t.Errorf("ramp meter: ok=%v meters=%d", ok, meters)
	}
	for _, o := range m.Net.Origins {
		if o.Meter != nil {
			t.Fatal("apply must not change the baseline network")
		}
	}
	// Not applicable: no traffic on the ramp at node 3 to meter; a lane
	// drop has no ramp to re-stripe or meter.
	for _, tc := range []struct {
		s  Site
		ft FixType
	}{{merge3, RampMeter}, {drop, Restripe}, {drop, RampMeter}, {drop, Channelise}} {
		if _, _, ok := m.apply(c, tc.s, tc.ft); ok {
			t.Errorf("%s at %s should not apply", tc.ft, tc.s.Key)
		}
	}
}

func TestScenarios(t *testing.T) {
	m := congested(t)
	obs := []TravelObs{{Name: "trip", Depart: "06:30", FromM: 0, ToM: 3000}, {Name: "short", Depart: "06:30", FromM: 0, ToM: 1000}}
	set, err := m.Scenarios(fixCapacity(), obs, 2200, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if set.Baseline.DelayVehH < 50 || len(set.Baseline.Queues) == 0 || set.Baseline.Queues[0][0] != 0 {
		t.Fatalf("baseline should be queued from the route start: %+v", set.Baseline)
	}
	free := 3000 / 1000.0 / 100 * 60
	if set.Baseline.CommuteMin < 1.5*free {
		t.Fatalf("baseline commute %.1f min should be well above free flow %.1f", set.Baseline.CommuteMin, free)
	}
	byID := map[string]Scenario{}
	for i, sc := range set.Scenarios {
		byID[sc.ID] = sc
		if i > 0 && sc.DelaySavedVehH > set.Scenarios[i-1].DelaySavedVehH {
			t.Fatal("scenarios not sorted by delay saved")
		}
		if math.Abs(sc.DelaySavedVehH-(set.Baseline.DelayVehH-sc.Outcome.DelayVehH)) > 1e-9 {
			t.Fatalf("%s: delay saved does not match outcomes", sc.ID)
		}
	}
	// A lane after the jammed merge clears it: nearly all delay goes.
	best := byID["plus_lane-node:2"]
	if !best.Helps || best.DelaySavedVehH < 0.9*set.Baseline.DelayVehH || best.CommuteSavedMin <= 0 || len(best.Outcome.Queues) != 0 {
		t.Errorf("plus lane at the jammed merge: %+v (baseline %.1f veh·h)", best, set.Baseline.DelayVehH)
	}
	if set.Scenarios[0].Site.Key != "node:2" {
		t.Errorf("best fix should be at the jammed merge, got %s", set.Scenarios[0].ID)
	}
	// Downstream of the jam nothing is queued, so a lane there saves nothing.
	if far := byID["plus_lane-way:102>103"]; far.Helps || math.Abs(far.DelaySavedVehH) > 1 {
		t.Errorf("plus lane where nothing queues: %+v", far)
	}
	// Dividers recover part of the lost flow: some saving, less than a lane.
	div := byID["channelise-node:2"]
	if !div.Helps || div.DelaySavedVehH >= best.DelaySavedVehH || div.Confidence != "low" {
		t.Errorf("dividers at the jammed merge: %+v", div)
	}
	if !byID["plus_lane-node:2"].Calibrated || byID["plus_lane-way:102>103"].Calibrated {
		t.Error("sites past 2200 m must be marked uncalibrated, earlier ones calibrated")
	}
}
