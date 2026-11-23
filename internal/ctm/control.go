package ctm

import (
	"errors"
	"fmt"
	"math"
)

// Signal is a fixed-time traffic light on a node. Phases run in order from
// the start of the cycle; any time left after the last green is all-red
// (lost time). During a phase only the listed In cells may send.
type Signal struct {
	CycleS  float64
	OffsetS float64
	Phases  []Phase
}

// Phase is one green period.
type Phase struct {
	GreenS float64
	In     []int // indices into Node.In that have green
}

func (s *Signal) validate(nIn int) error {
	if s.CycleS <= 0 {
		return errors.New("signal: CycleS must be > 0")
	}
	total := 0.0
	for pi, p := range s.Phases {
		if p.GreenS <= 0 {
			return fmt.Errorf("signal: phase %d: GreenS must be > 0", pi)
		}
		total += p.GreenS
		for _, i := range p.In {
			if i < 0 || i >= nIn {
				return fmt.Errorf("signal: phase %d: in index %d out of range", pi, i)
			}
		}
	}
	if total > s.CycleS+eps {
		return fmt.Errorf("signal: greens sum to %.1f s, more than the %.1f s cycle", total, s.CycleS)
	}
	return nil
}

// green reports which In indices have green at time tS.
func (s *Signal) green(tS float64, nIn int) []bool {
	return s.greenInto(make([]bool, nIn), tS)
}

// greenInto is green writing into g (one entry per In cell); it returns g.
func (s *Signal) greenInto(g []bool, tS float64) []bool {
	clear(g)
	t := math.Mod(tS+s.OffsetS, s.CycleS)
	if t < 0 {
		t += s.CycleS
	}
	for _, p := range s.Phases {
		if t < p.GreenS {
			for _, i := range p.In {
				g[i] = true
			}
			return g
		}
		t -= p.GreenS
	}
	return g // all-red
}

// Webster returns the delay-minimising cycle length and effective greens
// (Webster 1958): C0 = (1.5 L + 5) / (1 − Y), green_i = (C0 − L) y_i / Y,
// where L is total lost time per cycle (s) and y_i the critical flow ratio
// (flow / saturation flow) of each phase. The result is clamped to
// [minCycleS, maxCycleS]. It fails when the junction is oversaturated
// (Y ≥ 1): no timing can serve that demand.
func Webster(lostTimeS float64, flowRatios []float64, minCycleS, maxCycleS float64) (cycleS float64, greensS []float64, err error) {
	if len(flowRatios) == 0 {
		return 0, nil, errors.New("webster: no phases")
	}
	y := 0.0
	for _, r := range flowRatios {
		if r <= 0 {
			return 0, nil, errors.New("webster: flow ratios must be > 0")
		}
		y += r
	}
	if y >= 1 {
		return 0, nil, fmt.Errorf("webster: junction oversaturated (Y = %.2f ≥ 1)", y)
	}
	c := (1.5*lostTimeS + 5) / (1 - y)
	c = math.Max(minCycleS, math.Min(maxCycleS, c))
	if c <= lostTimeS {
		return 0, nil, errors.New("webster: cycle not longer than lost time")
	}
	greensS = make([]float64, len(flowRatios))
	for i, r := range flowRatios {
		greensS[i] = (c - lostTimeS) * r / y
	}
	return c, greensS, nil
}

// Meter is an ALINEA ramp meter (Papageorgiou, Hadj-Salem & Blosseville
// 1991): every UpdateS it nudges the metering rate towards keeping the
// merge area at its target density:
//
//	r(k) = r(k−1) + Gain × (target − measured)
//
// using per-lane density as the occupancy stand-in. measured is the highest
// per-lane density among MeasureCells. A real ALINEA detector sits in the
// merge area, which is queued during a breakdown; in a CTM that queue sits in
// the mainline cell just before the merge node, while the cell after it
// flows freely, so both should be listed. If the ramp queue
// exceeds MaxQueueVeh the meter opens to MaxVph so the queue does not spill
// onto the local road.
type Meter struct {
	MeasureCells         []int
	TargetDensityPerLane float64 // veh/km/lane, usually near critical
	GainVph              float64 // veh/h per (veh/km/lane) of error
	MinVph, MaxVph       float64
	MaxQueueVeh          float64 // 0 = no queue override
	UpdateS              float64
}

func (m *Meter) validate(nCells int) error {
	if len(m.MeasureCells) == 0 {
		return errors.New("meter: no measure cells")
	}
	for _, c := range m.MeasureCells {
		if c < 0 || c >= nCells {
			return errors.New("meter: measure cell out of range")
		}
	}
	if m.TargetDensityPerLane <= 0 || m.GainVph <= 0 || m.UpdateS <= 0 {
		return errors.New("meter: target, gain and update interval must be > 0")
	}
	if m.MinVph < 0 || m.MaxVph <= m.MinVph {
		return errors.New("meter: need 0 ≤ MinVph < MaxVph")
	}
	return nil
}
