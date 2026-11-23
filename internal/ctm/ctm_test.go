package ctm

import (
	"math"
	"strings"
	"testing"
)

// Test cells use round textbook numbers, not calibrated ones: 500 m cells,
// 2,000 veh/h/lane, 100 km/h free speed, 20 km/h wave speed, 125 veh/km/lane
// jam density. With DtS = 18 s a free-flowing vehicle crosses exactly one cell
// per step.
func motorway(lanes float64) Cell {
	return Cell{LengthM: 500, Lanes: lanes, CapPerLaneVph: 2000, FreeSpeedKph: 100, WaveSpeedKph: 20, JamDensityPerLane: 125}
}

const motorwayDt = 18.0

// chain links cells[from..to) in series and returns the cell indices.
func chain(net *Network, cells ...Cell) []int {
	ids := make([]int, len(cells))
	for j, c := range cells {
		ids[j] = len(net.Cells)
		net.Cells = append(net.Cells, c)
		if j > 0 {
			net.Nodes = append(net.Nodes, Node{In: []int{ids[j-1]}, Out: []int{ids[j]}})
		}
	}
	return ids
}

func repeat(c Cell, n int) []Cell {
	out := make([]Cell, n)
	for i := range out {
		out[i] = c
	}
	return out
}

func TestValidate(t *testing.T) {
	base := func() Network {
		n := Network{DtS: motorwayDt}
		chain(&n, motorway(3), motorway(3))
		return n
	}
	tests := []struct {
		name   string
		mutate func(*Network)
		want   string // substring of error; "" = valid
	}{
		{"valid", func(*Network) {}, ""},
		{"CFL: step too long", func(n *Network) { n.DtS = 20 }, "shorter than one step"},
		{"n-in m-out node", func(n *Network) {
			n.Cells = append(n.Cells, motorway(1), motorway(1))
			n.Nodes = []Node{{In: []int{0, 2}, Out: []int{1, 3}}}
		}, "not supported"},
		{"splits must sum to 1", func(n *Network) {
			n.Cells = append(n.Cells, motorway(1))
			n.Nodes = []Node{{In: []int{0}, Out: []int{1, 2}, Split: []float64{0.5, 0.4}}}
		}, "sum to"},
		{"origin on a cell with an upstream node", func(n *Network) {
			n.Origins = []Origin{{Cell: 1, IntervalS: 60}}
		}, "already has an upstream node"},
		{"greens longer than cycle", func(n *Network) {
			n.Nodes[0].Signal = &Signal{CycleS: 60, Phases: []Phase{{GreenS: 50, In: []int{0}}, {GreenS: 20}}}
		}, "more than the"},
		{"cell with two downstream nodes", func(n *Network) {
			n.Cells = append(n.Cells, motorway(3))
			n.Nodes = append(n.Nodes, Node{In: []int{0}, Out: []int{2}})
		}, "two downstream nodes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := base()
			tc.mutate(&n)
			err := n.Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestAllocate(t *testing.T) {
	tests := []struct {
		name string
		s, p []float64
		R    float64
		want []float64
	}{
		{"everyone fits", []float64{1000, 500}, []float64{3, 1}, 2000, []float64{1000, 500}},
		{"both want more than their share", []float64{6000, 2000}, []float64{3, 1}, 4000, []float64{3000, 1000}},
		{"unused share goes to the other", []float64{6000, 200}, []float64{3, 1}, 4000, []float64{3800, 200}},
		{"zero sender gets nothing", []float64{6000, 0}, []float64{1, 1}, 4000, []float64{4000, 0}},
		{"three inputs, two small", []float64{100, 100, 5000}, []float64{1, 1, 1}, 3000, []float64{100, 100, 2800}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := allocate(tc.s, tc.p, tc.R)
			for i := range got {
				if math.Abs(got[i]-tc.want[i]) > 1e-9 {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// Vehicles are neither created nor lost: arrivals = still queued at origins
// + in the network + exited, through merges, diverges, signals and meters.
func TestConservation(t *testing.T) {
	net := Network{DtS: motorwayDt}
	up := chain(&net, repeat(motorway(3), 5)...)
	ramp := chain(&net, repeat(motorway(1), 2)...)
	down := chain(&net, repeat(motorway(3), 4)...)
	exitA := chain(&net, repeat(motorway(1), 2)...)
	exitB := chain(&net, repeat(motorway(2), 3)...)
	net.Nodes = append(net.Nodes,
		Node{In: []int{up[4], ramp[1]}, Out: []int{down[0]}, CapDrop: 0.1},
		Node{In: []int{down[3]}, Out: []int{exitA[0], exitB[0]}, Split: []float64{0.3, 0.7}},
	)
	net.Origins = []Origin{
		{Cell: up[0], IntervalS: 900, DemandVph: []float64{4000, 6000, 6000, 2000}},
		{Cell: ramp[0], IntervalS: 900, DemandVph: []float64{800, 1800, 1200, 0},
			Meter: &Meter{MeasureCells: []int{up[4], down[0]}, TargetDensityPerLane: 19, GainVph: 40, MinVph: 200, MaxVph: 1800, UpdateS: 60}},
	}
	net.Nodes[0].Signal = &Signal{CycleS: 90, Phases: []Phase{{GreenS: 80, In: []int{0}}}}

	res, err := Run(net, 400)
	if err != nil {
		t.Fatal(err)
	}
	last := len(res.Vehicles) - 1
	inNet, queued := 0.0, 0.0
	for _, v := range res.Vehicles[last] {
		if v < -1e-9 {
			t.Fatalf("negative vehicles: %v", v)
		}
		inNet += v
	}
	for _, q := range res.OriginQueue[last] {
		queued += q
	}
	if got := queued + inNet + res.Exited; math.Abs(got-res.Arrived) > 1e-6 {
		t.Fatalf("arrived %.6f, but queued+in network+exited = %.6f", res.Arrived, got)
	}
	for step := range res.Vehicles {
		for i, c := range net.Cells {
			if k := res.Vehicles[step][i] / (c.LengthM / 1000); k > c.JamDensity()+1e-6 {
				t.Fatalf("step %d cell %d above jam density: %.1f", step, i, k)
			}
		}
	}
}

func TestFreeFlowTravelTime(t *testing.T) {
	net := Network{DtS: motorwayDt}
	route := chain(&net, repeat(motorway(3), 10)...)
	net.Origins = []Origin{{Cell: route[0], IntervalS: 3600, DemandVph: []float64{1000}}}
	res, err := Run(net, 100)
	if err != nil {
		t.Fatal(err)
	}
	want := FreeFlowTimeS(net, route) // 5 km at 100 km/h = 180 s
	if math.Abs(want-180) > 1e-9 {
		t.Fatalf("free-flow time = %v, want 180", want)
	}
	got, ok := res.TravelTimeS(net, route, 20)
	if !ok || math.Abs(got-want) > 1e-6 {
		t.Fatalf("travel time = %v (ok=%v), want %v", got, ok, want)
	}
	if d := res.DelayVehH(net); math.Abs(d) > 1e-3 {
		t.Fatalf("free flow should have no delay, got %.4f veh·h", d)
	}
}

// laneDrop builds 30 cells of 3 lanes then 10 of 2 lanes, fed at demandVph.
func laneDrop(demandVph, capDrop float64) (Network, []int, int) {
	net := Network{DtS: motorwayDt}
	up := chain(&net, repeat(motorway(3), 30)...)
	down := chain(&net, repeat(motorway(2), 10)...)
	net.Nodes = append(net.Nodes, Node{In: []int{up[29]}, Out: []int{down[0]}, CapDrop: capDrop})
	net.Origins = []Origin{{Cell: up[0], IntervalS: 3600, DemandVph: []float64{demandVph, demandVph}}}
	return net, up, down[0]
}

// The queue tail behind a lane drop must move upstream at the shockwave
// speed from the fundamental diagram: (q_b − q_u) / (k_q − k_u).
func TestShockwaveSpeed(t *testing.T) {
	net, up, _ := laneDrop(5000, 0)
	res, err := Run(net, 400)
	if err != nil {
		t.Fatal(err)
	}
	c := motorway(3)
	qu, qb := 5000.0, 4000.0
	ku := qu / c.FreeSpeedKph
	kq := c.JamDensity() - qb/c.WaveSpeedKph
	want := (qb - qu) / (kq - ku) // km/h, negative = moving upstream (-8)

	s1, s2 := 50, 250 // 0.25 h and 1.25 h
	tail1, _, ok1 := res.QueueTailM(net, up, s1)
	tail2, _, ok2 := res.QueueTailM(net, up, s2)
	if !ok1 || !ok2 {
		t.Fatal("expected a queue behind the lane drop")
	}
	got := (tail2 - tail1) / 1000 / (float64(s2-s1) * motorwayDt / 3600)
	if math.Abs(got-want) > 0.15*math.Abs(want) {
		t.Fatalf("queue tail speed = %.2f km/h, want %.2f ± 15%%", got, want)
	}
}

func TestCapacityDrop(t *testing.T) {
	tests := []struct {
		capDrop float64
		want    float64 // discharge through the 2-lane bottleneck once queued
	}{
		{0, 4000},
		{0.1, 3600},
		{0.2, 3200},
	}
	for _, tc := range tests {
		net, _, bottleneck := laneDrop(5000, tc.capDrop)
		res, err := Run(net, 300)
		if err != nil {
			t.Fatal(err)
		}
		got := res.Inflow[250][bottleneck]
		if math.Abs(got-tc.want) > 1e-6 {
			t.Errorf("capDrop %.1f: discharge = %.1f veh/h, want %.1f", tc.capDrop, got, tc.want)
		}
	}
}

// rampMerge: 3-lane mainline + 1-lane on-ramp merging into 3 lanes, with
// capacity drop at the merge. Peak demand (5000 + 1500) exceeds the 6000
// veh/h downstream for an hour.
func rampMerge(meter bool) (Network, []int) {
	net := Network{DtS: motorwayDt}
	up := chain(&net, repeat(motorway(3), 20)...)
	ramp := chain(&net, repeat(motorway(1), 2)...)
	down := chain(&net, repeat(motorway(3), 10)...)
	net.Nodes = append(net.Nodes, Node{In: []int{up[19], ramp[1]}, Out: []int{down[0]}, CapDrop: 0.1})
	// 15-minute profile: mainline builds to 5,000 veh/h and holds an hour.
	o := Origin{Cell: ramp[0], IntervalS: 900, DemandVph: []float64{1500, 1500, 1500, 1500, 1500, 1500, 500, 500}}
	if meter {
		o.Meter = &Meter{MeasureCells: []int{up[19], down[0]}, TargetDensityPerLane: 19.5, GainVph: 40, MinVph: 200, MaxVph: 2000, UpdateS: 36}
	}
	net.Origins = []Origin{
		{Cell: up[0], IntervalS: 900, DemandVph: []float64{4000, 4500, 5000, 5000, 5000, 5000, 3000, 3000}},
		o,
	}
	return net, append(up, down...)
}

// Ramp metering holds the merge just below critical density, so the
// capacity drop never sets in: the merge discharges near 6,000 veh/h instead
// of 5,400, and delay (time beyond free-flow) falls even though ramp drivers
// now wait at the meter.
func TestALINEAReducesDelay(t *testing.T) {
	steps := 3 * 200 // 3 h, long enough for every queue to clear
	plain, _ := rampMerge(false)
	metered, _ := rampMerge(true)
	rp, err := Run(plain, steps)
	if err != nil {
		t.Fatal(err)
	}
	rm, err := Run(metered, steps)
	if err != nil {
		t.Fatal(err)
	}
	if rp.Arrived != rm.Arrived {
		t.Fatalf("both runs must serve the same demand: %v vs %v", rp.Arrived, rm.Arrived)
	}
	dp, dm := rp.DelayVehH(plain), rm.DelayVehH(metered)
	if !(dm < 0.9*dp) {
		t.Fatalf("metering should cut delay by >10%%: unmetered %.1f veh·h, metered %.1f veh·h", dp, dm)
	}
	merged := 22                    // down[0]: 20 mainline cells + 2 ramp cells
	at := 75 * 60 / int(motorwayDt) // 75 min, mid-peak
	if got := rp.Inflow[at][merged]; math.Abs(got-5400) > 1e-6 {
		t.Fatalf("unmetered merge should have broken down to 5,400 veh/h, got %.0f", got)
	}
	if got := rm.Inflow[at][merged]; got < 5700 {
		t.Fatalf("metered merge should discharge ≥ 5,700 veh/h, got %.0f", got)
	}
	for step := range rm.MeterRate {
		if r := rm.MeterRate[step][1]; r < 200-1e-9 || r > 2000+1e-9 {
			t.Fatalf("meter rate %v out of [200, 2000]", r)
		}
	}
}

// urban is a 100 m, 1-lane street cell at 50 km/h; with DtS = 5 s a vehicle
// crosses 69 m per step, inside the CFL limit.
func urban(lanes float64) Cell {
	return Cell{LengthM: 100, Lanes: lanes, CapPerLaneVph: 1800, FreeSpeedKph: 50, WaveSpeedKph: 20, JamDensityPerLane: 140}
}

func TestSignalRedAndGreen(t *testing.T) {
	net := Network{DtS: 5}
	route := chain(&net, repeat(urban(1), 6)...)
	net.Nodes[len(net.Nodes)-1].Signal = &Signal{CycleS: 60, Phases: []Phase{{GreenS: 30, In: []int{0}}}}
	stop := route[4] // the cell before the light
	net.Origins = []Origin{{Cell: route[0], IntervalS: 3600, DemandVph: []float64{1200}}}
	res, err := Run(net, 240)
	if err != nil {
		t.Fatal(err)
	}
	sawSaturated := false
	for step := 120; step < 240; step++ {
		tS := float64(step) * net.DtS
		f := res.Outflow[step][stop]
		if math.Mod(tS, 60) >= 30 {
			if f != 0 {
				t.Fatalf("t=%.0f s is red, but %.1f veh/h crossed", tS, f)
			}
		} else if math.Abs(f-1800) < 1e-6 {
			sawSaturated = true // queue discharging at saturation flow
		}
	}
	if !sawSaturated {
		t.Fatal("expected the queue to discharge at the 1,800 veh/h saturation flow on green")
	}
}

func TestWebster(t *testing.T) {
	c, g, err := Webster(10, []float64{0.5, 1.0 / 6}, 30, 150)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(c-60) > 1e-9 || math.Abs(g[0]-37.5) > 1e-9 || math.Abs(g[1]-12.5) > 1e-9 {
		t.Fatalf("got cycle %.2f greens %v, want 60 [37.5 12.5]", c, g)
	}
	if _, _, err := Webster(10, []float64{0.6, 0.5}, 30, 150); err == nil {
		t.Fatal("want error for an oversaturated junction")
	}
}

// signalJunction: two 1-lane approaches (900 and 300 veh/h) share a light
// into a 2-lane exit street.
func signalJunction(greenMain, greenSide float64) (Network, error) {
	net := Network{DtS: 5}
	a := chain(&net, repeat(urban(1), 8)...)
	b := chain(&net, repeat(urban(1), 8)...)
	out := chain(&net, repeat(urban(2), 4)...)
	net.Nodes = append(net.Nodes, Node{
		In: []int{a[7], b[7]}, Out: []int{out[0]},
		Signal: &Signal{CycleS: 60, Phases: []Phase{{GreenS: greenMain, In: []int{0}}, {GreenS: greenSide, In: []int{1}}}},
	})
	net.Origins = []Origin{
		{Cell: a[0], IntervalS: 3600, DemandVph: []float64{900}},
		{Cell: b[0], IntervalS: 3600, DemandVph: []float64{300}},
	}
	return net, net.Validate()
}

func TestWebsterTimingBeatsBadSplit(t *testing.T) {
	_, g, err := Webster(10, []float64{900.0 / 1800, 300.0 / 1800}, 30, 150)
	if err != nil {
		t.Fatal(err)
	}
	good, err := signalJunction(g[0], g[1])
	if err != nil {
		t.Fatal(err)
	}
	bad, err := signalJunction(15, 35) // main road starved: 450 veh/h of green for 900 veh/h
	if err != nil {
		t.Fatal(err)
	}
	steps := 2 * 720 // 1 h of demand + 1 h to clear
	rg, err := Run(good, steps)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := Run(bad, steps)
	if err != nil {
		t.Fatal(err)
	}
	dg, db := rg.DelayVehH(good), rb.DelayVehH(bad)
	if !(dg < db/2) {
		t.Fatalf("Webster timing delay %.1f veh·h should be well under bad split %.1f veh·h", dg, db)
	}
}
