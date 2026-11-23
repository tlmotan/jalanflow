package ctm

import (
	"errors"
	"math"
)

// Result holds the state of every cell at every step. Rows are steps,
// columns are cells (or origins).
type Result struct {
	DtS         float64
	Vehicles    [][]float64 // vehicles in each cell at the end of each step
	Outflow     [][]float64 // veh/h leaving each cell during each step
	Inflow      [][]float64 // veh/h entering each cell during each step
	OriginQueue [][]float64 // vehicles waiting at each origin at the end of each step
	MeterRate   [][]float64 // metering rate in force at each origin (veh/h); +Inf if unmetered
	Entered     float64     // vehicles that left an origin queue into the network
	Arrived     float64     // vehicles that joined an origin queue (demand)
	Exited      float64     // vehicles that left through exit cells
}

// Run simulates steps time steps from an empty network.
//
// The step loop allocates nothing: per-cell constants are worked out once,
// node scratch space is reused, and each Result table is one block cut into
// rows. A calibration or fix sweep calls Run many times, so this matters.
func Run(net Network, steps int) (*Result, error) {
	if err := net.Validate(); err != nil {
		return nil, err
	}
	if steps <= 0 {
		return nil, errors.New("ctm: steps must be > 0")
	}
	nc, no := len(net.Cells), len(net.Origins)
	dtH := net.DtS / 3600

	// Per-cell constants, using the same expressions as the Cell methods.
	lenKm := make([]float64, nc)
	capV := make([]float64, nc)
	kCrit := make([]float64, nc)
	hasDown := make([]bool, nc)
	for i, c := range net.Cells {
		lenKm[i] = c.LengthM / 1000
		capV[i] = c.CapacityVph()
		kCrit[i] = c.CriticalDensity()
	}

	// Merge priorities (lanes when not given) and the widest node, for
	// scratch space shared by every node and step.
	prio := make([][]float64, len(net.Nodes))
	maxIn, maxOut := 0, 0
	for ni, nd := range net.Nodes {
		maxIn, maxOut = max(maxIn, len(nd.In)), max(maxOut, len(nd.Out))
		for _, c := range nd.In {
			hasDown[c] = true
		}
		prio[ni] = nd.Priority
		if nd.Priority == nil && len(nd.Out) == 1 {
			p := make([]float64, len(nd.In))
			for j, c := range nd.In {
				p[j] = net.Cells[c].Lanes
			}
			prio[ni] = p
		}
	}
	sBuf := make([]float64, maxIn)
	fBuf := make([]float64, maxIn)
	activeBuf := make([]bool, maxIn)
	greenBuf := make([]bool, maxIn)
	rvBuf := make([]float64, maxOut)

	r := &Result{
		DtS:         net.DtS,
		Vehicles:    table(steps, nc),
		Outflow:     table(steps, nc),
		Inflow:      table(steps, nc),
		OriginQueue: table(steps, no),
		MeterRate:   table(steps, no),
	}
	veh := make([]float64, nc)
	queue := make([]float64, no)
	rate := make([]float64, no)
	for oi, o := range net.Origins {
		rate[oi] = math.Inf(1)
		if o.Meter != nil {
			rate[oi] = o.Meter.MaxVph
		}
	}

	send := make([]float64, nc)
	recv := make([]float64, nc)
	for step := 0; step < steps; step++ {
		tS := float64(step) * net.DtS
		for i := range net.Cells {
			c := &net.Cells[i]
			k := veh[i] / lenKm[i]
			q := capV[i]
			if k <= kCrit[i] {
				send[i] = c.FreeSpeedKph * k
			} else {
				send[i] = q
			}
			// Never send more than the cell holds.
			send[i] = math.Min(send[i], veh[i]/dtH)
			recv[i] = math.Min(q, c.WaveSpeedKph*(c.JamDensity()-k))
			if recv[i] < 0 {
				recv[i] = 0
			}
		}

		out, in := r.Outflow[step], r.Inflow[step]
		for ni := range net.Nodes {
			nd := &net.Nodes[ni]
			s := sBuf[:len(nd.In)]
			queued := false
			for j, c := range nd.In {
				s[j] = send[c]
				if veh[c]/lenKm[c] > kCrit[c]*(1+1e-9) {
					queued = true
				}
			}
			if nd.Signal != nil {
				g := nd.Signal.greenInto(greenBuf[:len(nd.In)], tS)
				for j := range s {
					if !g[j] {
						s[j] = 0
					}
				}
			}
			rv := rvBuf[:len(nd.Out)]
			for j, c := range nd.Out {
				rv[j] = recv[c]
				if queued && nd.CapDrop > 0 {
					rv[j] = math.Min(rv[j], (1-nd.CapDrop)*capV[c])
				}
			}
			switch {
			case len(nd.Out) == 1:
				f := allocateInto(fBuf[:len(nd.In)], activeBuf[:len(nd.In)], s, prio[ni], rv[0])
				for j, c := range nd.In {
					out[c] += f[j]
					in[nd.Out[0]] += f[j]
				}
			default: // diverge, FIFO: a blocked branch holds everyone back
				f := s[0]
				for j := range nd.Out {
					if nd.Split[j] > 0 {
						f = math.Min(f, rv[j]/nd.Split[j])
					}
				}
				out[nd.In[0]] += f
				for j, c := range nd.Out {
					in[c] += f * nd.Split[j]
				}
			}
		}

		exitFlow := 0.0
		for i := range net.Cells {
			if !hasDown[i] {
				out[i] = send[i] // exits discharge freely
				exitFlow += out[i]
			}
		}

		// Origins: arrivals join the point queue, then enter as the cell and meter allow.
		for oi := range net.Origins {
			o := &net.Origins[oi]
			idx := int(tS / o.IntervalS)
			d := 0.0
			if idx < len(o.DemandVph) {
				d = o.DemandVph[idx]
			}
			queue[oi] += d * dtH
			r.Arrived += d * dtH
			if m := o.Meter; m != nil {
				if step > 0 && math.Mod(tS, m.UpdateS) < net.DtS-eps {
					kpl := 0.0
					for _, c := range m.MeasureCells {
						kpl = math.Max(kpl, veh[c]/lenKm[c]/net.Cells[c].Lanes)
					}
					rate[oi] += m.GainVph * (m.TargetDensityPerLane - kpl)
					rate[oi] = math.Max(m.MinVph, math.Min(m.MaxVph, rate[oi]))
				}
			}
			allowed := rate[oi]
			if o.Meter != nil && o.Meter.MaxQueueVeh > 0 && queue[oi] > o.Meter.MaxQueueVeh {
				allowed = o.Meter.MaxVph
			}
			f := math.Min(queue[oi]/dtH, math.Min(recv[o.Cell], allowed))
			queue[oi] -= f * dtH
			if queue[oi] < 0 {
				queue[oi] = 0
			}
			in[o.Cell] += f
			r.Entered += f * dtH
		}

		for i := range veh {
			veh[i] += (in[i] - out[i]) * dtH
			if veh[i] < 0 && veh[i] > -1e-6 {
				veh[i] = 0
			}
		}
		r.Exited += exitFlow * dtH

		copy(r.Vehicles[step], veh)
		copy(r.OriginQueue[step], queue)
		copy(r.MeterRate[step], rate)
	}
	return r, nil
}

// table returns rows zeroed slices of length cols backed by one allocation.
// Each row's capacity is capped so appending to one cannot spill into the next.
func table(rows, cols int) [][]float64 {
	t := make([][]float64, rows)
	block := make([]float64, rows*cols)
	for i := range t {
		t[i] = block[i*cols : (i+1)*cols : (i+1)*cols]
	}
	return t
}

// allocate splits supply R among senders with demands s and priorities p
// (Daganzo 1995, generalised to n inputs): everyone is served if they fit;
// otherwise each gets its priority share, and share a sender cannot use is
// handed to the others.
func allocate(s, p []float64, R float64) []float64 {
	return allocateInto(make([]float64, len(s)), make([]bool, len(s)), s, p, R)
}

// allocateInto is allocate writing into f, with active as scratch space;
// both must have len(s). It returns f.
func allocateInto(f []float64, active []bool, s, p []float64, R float64) []float64 {
	total := 0.0
	for _, v := range s {
		total += v
	}
	if total <= R {
		copy(f, s)
		return f
	}
	for i := range active {
		f[i] = 0
		active[i] = s[i] > 0
	}
	rem := R
	for {
		psum := 0.0
		for i, a := range active {
			if a {
				psum += p[i]
			}
		}
		if psum == 0 || rem <= 0 {
			return f
		}
		share := rem / psum
		changed := false
		for i, a := range active {
			if a && s[i] <= share*p[i] {
				f[i] = s[i]
				rem -= s[i]
				active[i] = false
				changed = true
			}
		}
		if !changed {
			for i, a := range active {
				if a {
					f[i] = share * p[i]
				}
			}
			return f
		}
	}
}
