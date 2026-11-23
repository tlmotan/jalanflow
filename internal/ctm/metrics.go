package ctm

import "math"

// TotalTimeSpentVehH is the vehicle-hours spent in the network, including
// time waiting in origin queues.
func (r *Result) TotalTimeSpentVehH() float64 {
	dtH := r.DtS / 3600
	tts := 0.0
	for step := range r.Vehicles {
		for _, v := range r.Vehicles[step] {
			tts += v * dtH
		}
		for _, q := range r.OriginQueue[step] {
			tts += q * dtH
		}
	}
	return tts
}

// DelayVehH is total time spent minus the time the same vehicles would have
// needed at free speed through the cells they entered.
func (r *Result) DelayVehH(net Network) float64 {
	dtH := r.DtS / 3600
	free := 0.0
	for step := range r.Inflow {
		for i, f := range r.Inflow[step] {
			c := net.Cells[i]
			free += f * dtH * (c.LengthM / 1000) / c.FreeSpeedKph
		}
	}
	return r.TotalTimeSpentVehH() - free
}

// SpeedKph is the mean speed in cell i during step: outflow / density,
// capped at free speed; free speed when the cell is (nearly) empty.
func (r *Result) SpeedKph(net Network, step, i int) float64 {
	c := net.Cells[i]
	k := r.vehAtStart(step, i) / (c.LengthM / 1000)
	if k < 1e-6 {
		return c.FreeSpeedKph
	}
	v := r.Outflow[step][i] / k
	return math.Max(1, math.Min(c.FreeSpeedKph, v))
}

func (r *Result) vehAtStart(step, i int) float64 {
	if step == 0 {
		return 0
	}
	return r.Vehicles[step-1][i]
}

// TravelTimeS is the experienced travel time along route (cells in order)
// for a vehicle entering its first cell at the start of departStep: it
// crosses each cell at that cell's speed at the moment it is there. ok is
// false if the simulation ends before the vehicle arrives.
func (r *Result) TravelTimeS(net Network, route []int, departStep int) (tS float64, ok bool) {
	t := float64(departStep) * r.DtS
	end := float64(len(r.Outflow)) * r.DtS
	for _, i := range route {
		left := net.Cells[i].LengthM
		for left > 1e-9 {
			step := int(t / r.DtS)
			if step >= len(r.Outflow) {
				return 0, false
			}
			v := r.SpeedKph(net, step, i) / 3.6 // m/s
			stepEnd := float64(step+1) * r.DtS
			if d := v * (stepEnd - t); d >= left {
				t += left / v
				left = 0
			} else {
				left -= d
				t = stepEnd
			}
			if t > end {
				return 0, false
			}
		}
	}
	return t - float64(departStep)*r.DtS, true
}

// FreeFlowTimeS is the time to cross route at free speed.
func FreeFlowTimeS(net Network, route []int) float64 {
	t := 0.0
	for _, i := range route {
		c := net.Cells[i]
		t += c.LengthM / (c.FreeSpeedKph / 3.6)
	}
	return t
}

// Queued reports whether cell i is above critical density at the end of step.
func (r *Result) Queued(net Network, step, i int) bool {
	c := net.Cells[i]
	return r.Vehicles[step][i]/(c.LengthM/1000) > c.CriticalDensity()*(1+1e-6)
}

// QueueTailM returns, along route at the end of step, the distance from the
// route start to the upstream end of the queue that reaches back from the
// most downstream queued cell, and that queue's length. ok is false if no
// cell on the route is queued.
func (r *Result) QueueTailM(net Network, route []int, step int) (tailM, lengthM float64, ok bool) {
	pos := make([]float64, len(route)+1)
	for j, i := range route {
		pos[j+1] = pos[j] + net.Cells[i].LengthM
	}
	head := -1
	for j := len(route) - 1; j >= 0; j-- {
		if r.Queued(net, step, route[j]) {
			head = j
			break
		}
	}
	if head < 0 {
		return 0, 0, false
	}
	tail := head
	for tail > 0 && r.Queued(net, step, route[tail-1]) {
		tail--
	}
	return pos[tail], pos[head+1] - pos[tail], true
}
