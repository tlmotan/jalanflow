package ctm

import (
	"math"
	"testing"
)

// benchNetwork is a corridor-sized model: a 150-cell mainline with a lane
// drop, a metered on-ramp, an off-ramp and a signalised junction, loaded
// past capacity so queues, capacity drop and the meter all come into play.
func benchNetwork() Network {
	net := Network{DtS: motorwayDt}
	a := chain(&net, repeat(motorway(3), 50)...)
	b := chain(&net, repeat(motorway(3), 50)...)
	c := chain(&net, repeat(motorway(2), 50)...)
	ramp := chain(&net, repeat(motorway(1), 3)...)
	off := chain(&net, repeat(motorway(1), 3)...)
	side := chain(&net, repeat(motorway(1), 3)...)
	tail := chain(&net, repeat(motorway(2), 5)...)
	net.Nodes = append(net.Nodes,
		Node{In: []int{a[49], ramp[2]}, Out: []int{b[0]}, CapDrop: 0.1},
		Node{In: []int{b[49]}, Out: []int{c[0], off[0]}, Split: []float64{0.8, 0.2}, CapDrop: 0.1},
		Node{In: []int{c[49], side[2]}, Out: []int{tail[0]}, Signal: &Signal{CycleS: 90, Phases: []Phase{
			{GreenS: 50, In: []int{0}}, {GreenS: 30, In: []int{1}}}}},
	)
	peak := []float64{3000, 4500, 5800, 5800, 4500, 3000, 1500, 500}
	net.Origins = []Origin{
		{Cell: a[0], IntervalS: 900, DemandVph: peak},
		{Cell: ramp[0], IntervalS: 900, DemandVph: []float64{600, 900, 1200, 1200, 900, 600, 300, 100},
			Meter: &Meter{MeasureCells: []int{a[49], b[0]}, TargetDensityPerLane: 20, GainVph: 70, MinVph: 240, MaxVph: 1800, MaxQueueVeh: 60, UpdateS: 36}},
		{Cell: side[0], IntervalS: 900, DemandVph: []float64{300, 500, 700, 700, 500, 300, 200, 100}},
	}
	return net
}

// TestRunReusedBuffers guards the scratch space Run shares between nodes
// and steps: a stale value from another node would create or lose vehicles.
func TestRunReusedBuffers(t *testing.T) {
	net := benchNetwork()
	res, err := Run(net, 800)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Vehicles) != 800 || len(res.Vehicles[0]) != len(net.Cells) || len(res.MeterRate[0]) != len(net.Origins) {
		t.Fatalf("result tables have the wrong shape")
	}
	dtH := net.DtS / 3600
	held := 0.0 // vehicles in cells at the end of the previous step
	for step := range res.Vehicles {
		in, out, now := 0.0, 0.0, 0.0
		for i, v := range res.Vehicles[step] {
			if v < -1e-9 {
				t.Fatalf("step %d cell %d: negative vehicles %v", step, i, v)
			}
			now += v
			in += res.Inflow[step][i] * dtH
			out += res.Outflow[step][i] * dtH
		}
		// Flow between two cells counts once as in and once as out, so the
		// change in vehicles held must equal in minus out every step.
		if math.Abs(now-held-(in-out)) > 1e-6 {
			t.Fatalf("step %d: vehicles changed by %.9f, flows say %.9f", step, now-held, in-out)
		}
		held = now
	}
	queued := 0.0
	for _, q := range res.OriginQueue[len(res.OriginQueue)-1] {
		queued += q
	}
	if got := queued + held + res.Exited; math.Abs(got-res.Arrived) > 1e-6 {
		t.Fatalf("arrived %.6f, but queued+in network+exited = %.6f", res.Arrived, got)
	}
	// Rows must not share memory: appending to one must not change the next.
	next := res.Vehicles[1][0]
	_ = append(res.Vehicles[0], -1)
	if res.Vehicles[1][0] != next {
		t.Fatal("result rows overlap")
	}
}

func BenchmarkRun(b *testing.B) {
	net := benchNetwork()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Run(net, 800); err != nil {
			b.Fatal(err)
		}
	}
}
