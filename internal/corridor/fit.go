package corridor

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/ctm"
	"github.com/tlmotan/jalanflow/internal/graph"
)

// Demand nobody has counted is estimated from a handful of numbers
// (decision 0009), found by fitting the model to observed travel times.
// Everything else follows from the road itself, so 30 ramps do not become
// 30 free knobs.

// Section is the part of the route that ends at EndM. The observations'
// end points cut the route into sections, each with its own two numbers,
// because one pair for the whole route cannot put a queue in one place
// without flooding the next.
type Section struct {
	EndM float64
	// RampLoad is each uncounted on-ramp's peak traffic as a share of that
	// ramp's own capacity (lanes × per-lane capacity).
	RampLoad float64
	// ExitScale scales each uncounted off-ramp's share of the traffic
	// reaching it: share = ExitScale × ramp lanes ÷ (ramp lanes + lanes
	// that carry on), capped at Estimate.MaxExitSplit.
	ExitScale float64
}

// Params are the fitted numbers.
type Params struct {
	// MainPeakVPH is the peak traffic entering at the route start.
	MainPeakVPH float64
	// PeakStartMin is how long after the profile start demand reaches its peak.
	PeakStartMin float64
	// Sections in route order. Ramps past the last section's end have no
	// observation covering them and reuse the last section's numbers.
	Sections []Section
}

// at returns the section covering distance m along the route.
func (p Params) at(m float64) Section {
	for _, s := range p.Sections {
		if m <= s.EndM {
			return s
		}
	}
	return p.Sections[len(p.Sections)-1]
}

func clockMin(s string) (float64, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("time %q: want HH:MM", s)
	}
	return float64(t.Hour()*60 + t.Minute()), nil
}

// profile builds the trapezoid described on config.Estimate.
func profile(est *config.Estimate, stepMin int, peakStartMin float64) (config.Profile, error) {
	start, err := clockMin(est.Start)
	if err != nil {
		return config.Profile{}, err
	}
	peakEnd, err := clockMin(est.PeakEnd)
	if err != nil {
		return config.Profile{}, err
	}
	end, err := clockMin(est.End)
	if err != nil {
		return config.Profile{}, err
	}
	peakEnd, end = peakEnd-start, end-start
	if peakStartMin <= 0 || peakStartMin >= peakEnd {
		return config.Profile{}, fmt.Errorf("peak start %.0f min must fall between %s and %s", peakStartMin, est.Start, est.PeakEnd)
	}
	var shape []float64
	for t := 0.0; t < end; t += float64(stepMin) {
		mid := t + float64(stepMin)/2
		var v float64
		switch {
		case mid < peakStartMin:
			v = est.StartLevel + (1-est.StartLevel)*mid/peakStartMin
		case mid <= peakEnd:
			v = 1
		default:
			v = 1 - (1-est.EndLevel)*(mid-peakEnd)/(end-peakEnd)
		}
		shape = append(shape, math.Round(v*1000)/1000)
	}
	return config.Profile{Start: est.Start, Shape: shape, Source: fmt.Sprintf(
		"estimated - fitted trapezoid: %.2f at %s rising to 1 after %.0f min, 1 until %s, falling to %.2f at %s (cmd/calibrate)",
		est.StartLevel, est.Start, peakStartMin, est.PeakEnd, est.EndLevel, est.End)}, nil
}

// EstimateDemand returns a demand config for route in which the route start
// and every ramp without a counted figure get an estimate from p. Entries
// and exits in base whose source is neither a placeholder nor an earlier
// estimate are real counts and are kept as they are (they keep their own
// profile). year and source label the estimates.
func EstimateDemand(g *graph.Graph, cap *config.Capacity, base *config.Demand, route []graph.EdgeID, opt Options,
	p Params, year int, source string) (*config.Demand, error) {
	if base == nil || base.Estimate == nil {
		return nil, errors.New("corridor: demand config has no estimate block")
	}
	if len(p.Sections) == 0 {
		return nil, errors.New("corridor: params have no sections")
	}
	est := base.Estimate
	out := &config.Demand{
		Corridor: base.Corridor, ProfileStepMin: base.ProfileStepMin, Estimate: est, Note: base.Note,
		Profiles: map[string]config.Profile{},
	}
	real := func(src string) bool { return !config.IsPlaceholder(src) && !config.IsEstimated(src) }
	for _, en := range base.Entries {
		if real(en.Source) {
			out.Entries = append(out.Entries, en)
			out.Profiles[en.Profile] = base.Profiles[en.Profile]
		}
	}
	for _, ex := range base.Exits {
		if real(ex.Source) {
			out.Exits = append(out.Exits, ex)
		}
	}
	prof, err := profile(est, base.ProfileStepMin, p.PeakStartMin)
	if err != nil {
		return nil, fmt.Errorf("corridor: %w", err)
	}
	if old, ok := out.Profiles[est.Profile]; ok && old.Start != prof.Start {
		return nil, fmt.Errorf("corridor: counted profile %q starts at %s, estimate at %s", est.Profile, old.Start, prof.Start)
	}
	if _, ok := out.Profiles[est.Profile]; ok {
		return nil, fmt.Errorf("corridor: estimate profile name %q is used by counted entries; pick another", est.Profile)
	}
	out.Profiles[est.Profile] = prof

	// Lay the route out with the counted data only, to see what is missing.
	m, err := Build(g, cap, out, route, opt)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	key := func(e *graph.Edge) (string, error) {
		k := fmt.Sprintf("way:%d", firstWay(e))
		if seen[k] {
			return "", fmt.Errorf("corridor: two ramps start with OSM way %d; cannot key their estimates", firstWay(e))
		}
		seen[k] = true
		return k, nil
	}
	if m.MainKey == "" {
		n := g.Nodes[g.Edges[route[0]].From]
		en := config.DemandEntry{
			Key: fmt.Sprintf("node:%d", g.Edges[route[0]].From), Name: "traffic entering at the route start",
			PeakVPH: math.Round(p.MainPeakVPH), Profile: est.Profile, Source: source, Year: year,
		}
		if n != nil {
			en.Pin = config.LatLon{Lat: n.Pos.Lat, Lon: n.Pos.Lon}
		}
		out.Entries = append(out.Entries, en)
	}
	for _, r := range m.Ramps {
		if r.DemandKey != "" {
			continue
		}
		e := &g.Edges[r.EdgeID]
		k, err := key(e)
		if err != nil {
			return nil, err
		}
		var pin config.LatLon
		if n := g.Nodes[r.AtNode]; n != nil {
			pin = config.LatLon{Lat: n.Pos.Lat, Lon: n.Pos.Lon}
		}
		switch r.Kind {
		case OnRamp:
			cl, ok := cap.Classes[graph.BaseClass(r.Class)]
			if !ok {
				return nil, fmt.Errorf("corridor: no capacity for class %q", r.Class)
			}
			peak := math.Round(p.at(r.AtM).RampLoad * float64(r.Lanes*cl.PerLaneVPH))
			if peak < 1 {
				peak = 1
			}
			out.Entries = append(out.Entries, config.DemandEntry{Key: k, Name: r.Name, Pin: pin, PeakVPH: peak, Profile: est.Profile, Source: source, Year: year})
		case OffRamp:
			share := p.at(r.AtM).ExitScale * float64(r.Lanes) / float64(r.Lanes+r.MainLanes)
			share = math.Min(share, est.MaxExitSplit)
			out.Exits = append(out.Exits, config.DemandExit{Key: k, Name: r.Name, Pin: pin, SplitRatio: math.Round(share*1000) / 1000, Source: source, Year: year})
		}
	}
	return out, nil
}

// TravelObs is one observed travel time placed on the route.
type TravelObs struct {
	Name    string
	Depart  string  // clock time, HH:MM
	FromM   float64 // distance along the route
	ToM     float64
	Minutes float64 // observed (midpoint of the range read)
}

// QueueObs is one observed queue placed on the route: at Clock the road
// from TailM to HeadM was queued.
type QueueObs struct {
	Name         string
	Clock        string
	TailM, HeadM float64
}

// QueueFit compares one observed queue with the simulation.
type QueueFit struct {
	QueueObs
	Match float64 // share of the cells between tail and head that are queued
	OK    bool    // Match ≥ Estimate.MinQueueMatch
}

// CompareQueues reads, for each observed queue, how much of it the
// simulation reproduces.
func (m *Model) CompareQueues(res *ctm.Result, queues []QueueObs, minMatch float64) ([]QueueFit, error) {
	out := make([]QueueFit, len(queues))
	for i, q := range queues {
		step, err := m.StepAt(q.Clock)
		if err != nil {
			return nil, err
		}
		if step >= len(res.Vehicles) {
			return nil, fmt.Errorf("corridor: queue %q at %s is after the simulation ends", q.Name, q.Clock)
		}
		cells := m.CellsBetween(q.TailM, q.HeadM)
		if len(cells) == 0 {
			return nil, fmt.Errorf("corridor: queue %q covers no cells (%.0f m → %.0f m)", q.Name, q.TailM, q.HeadM)
		}
		n := 0
		for _, c := range cells {
			if res.Queued(m.Net, step, c) {
				n++
			}
		}
		match := float64(n) / float64(len(cells))
		out[i] = QueueFit{QueueObs: q, Match: match, OK: match >= minMatch}
	}
	return out, nil
}

// TravelFit compares one observation with the simulation.
type TravelFit struct {
	TravelObs
	SimMinutes float64
	// Arrived is false when the simulated car had not arrived by the end of
	// the run; SimMinutes is then the time simulated so far (a lower bound).
	Arrived bool
}

// ErrPct is the absolute error as a percentage of the observed time.
func (f TravelFit) ErrPct() float64 {
	return math.Abs(f.SimMinutes-f.Minutes) / f.Minutes * 100
}

// CellsBetween returns the mainline cells from distance fromM to toM.
func (m *Model) CellsBetween(fromM, toM float64) []int {
	var out []int
	for _, c := range m.Mainline {
		mid := m.Cells[c].StartM + m.Cells[c].LengthM/2
		if mid >= fromM && mid < toM {
			out = append(out, c)
		}
	}
	return out
}

// Steps returns how many time steps cover the demand profile plus extraS.
func (m *Model) Steps(extraS float64) int {
	longest := 0.0
	for _, o := range m.Net.Origins {
		longest = math.Max(longest, float64(len(o.DemandVph))*o.IntervalS)
	}
	return int(math.Ceil((longest + extraS) / m.Net.DtS))
}

// StepAt converts a clock time to a simulation step.
func (m *Model) StepAt(clock string) (int, error) {
	start, err := clockMin(m.ProfileStart)
	if err != nil {
		return 0, fmt.Errorf("corridor: profile start: %w", err)
	}
	at, err := clockMin(clock)
	if err != nil {
		return 0, err
	}
	if at < start {
		return 0, fmt.Errorf("corridor: %s is before the simulation starts (%s)", clock, m.ProfileStart)
	}
	return int((at - start) * 60 / m.Net.DtS), nil
}

// Compare runs nothing: it reads the simulated travel time for each
// observation out of res.
func (m *Model) Compare(res *ctm.Result, obs []TravelObs) ([]TravelFit, error) {
	fits := make([]TravelFit, len(obs))
	horizonS := float64(len(res.Outflow)) * res.DtS
	for i, o := range obs {
		step, err := m.StepAt(o.Depart)
		if err != nil {
			return nil, err
		}
		cells := m.CellsBetween(o.FromM, o.ToM)
		if len(cells) == 0 {
			return nil, fmt.Errorf("corridor: observation %q covers no cells (%.0f m → %.0f m)", o.Name, o.FromM, o.ToM)
		}
		tS, ok := res.TravelTimeS(m.Net, cells, step)
		if !ok {
			tS = horizonS - float64(step)*res.DtS
		}
		fits[i] = TravelFit{TravelObs: o, SimMinutes: tS / 60, Arrived: ok}
	}
	return fits, nil
}

// MeanErrPct is the mean absolute percentage error over fits.
func MeanErrPct(fits []TravelFit) float64 {
	if len(fits) == 0 {
		return math.Inf(1)
	}
	s := 0.0
	for _, f := range fits {
		s += f.ErrPct()
	}
	return s / float64(len(fits))
}

// FitResult is the outcome of the search.
type FitResult struct {
	Params Params
	ErrPct float64 // mean absolute % error on all travel times
	Demand *config.Demand
	Model  *Model
	Result *ctm.Result
	Fits   []TravelFit
	Queues []QueueFit
	// QueuesOK is true when every observed queue is reproduced.
	QueuesOK bool
	Tried    int
	// AtBound names fitted numbers whose best value sits on the edge of
	// its search range: the true best may lie outside it.
	AtBound []string
	// UncalibratedFromM is where the last observation ends; the model past
	// it is not checked against anything.
	UncalibratedFromM float64
}

// maxRefitRounds bounds the re-fitting loop in Fit; it normally settles
// in one or two rounds.
const maxRefitRounds = 4

func axis(r [3]float64) []float64 {
	var out []float64
	for v := r[0]; v <= r[1]+r[2]*1e-9; v += r[2] {
		out = append(out, v)
	}
	return out
}

// Fit searches the ranges in base.Estimate for the Params whose simulated
// travel times are closest to obs (mean absolute % error), among those
// that reproduce every observed queue. Travel times alone do not say where
// the queue is: on the real corridor two very different traffic patterns
// matched them within 10–12%, and only one had the queue where drivers
// meet it. A candidate that misses a queue is used only if none reproduces
// it (QueuesOK is then false).
//
// It works section by section, upstream first. Stage 1 fits the route-start
// traffic, the peak start and the first section's two numbers against the
// observations ending there, with everything further on left free-flowing
// (lightest ramps, busiest exits). Each later stage keeps what is already
// fitted and fits the next section's two numbers against all observations
// so far. Then each section is re-fitted against every observation until
// nothing changes. This is a greedy search, not a guarantee: when queues
// from one section reach deep into another it can settle on a worse match
// than a full search would, and ErrPct says how good the match is.
// Every stage is a plain grid search; ties go to the earliest
// combination, so the result is deterministic. extraS is simulated time
// after the demand profile ends, so late departures can arrive.
func Fit(g *graph.Graph, cap *config.Capacity, base *config.Demand, route []graph.EdgeID, opt Options,
	obs []TravelObs, queues []QueueObs, extraS float64, year int, source string) (*FitResult, error) {
	if base == nil || base.Estimate == nil {
		return nil, errors.New("corridor: demand config has no estimate block")
	}
	if len(obs) == 0 {
		return nil, errors.New("corridor: no travel-time observations to fit")
	}
	est := base.Estimate
	loads, scales := axis(est.RampLoad), axis(est.ExitScale)

	var ends []float64
	for _, o := range obs {
		ends = append(ends, o.ToM)
	}
	sort.Float64s(ends)
	ends = slices.Compact(ends)

	run := func(p Params, sel []TravelObs, selQ []QueueObs) (*FitResult, error) {
		d, err := EstimateDemand(g, cap, base, route, opt, p, year, source)
		if err != nil {
			return nil, err
		}
		m, err := Build(g, cap, d, route, opt)
		if err != nil {
			return nil, err
		}
		res, err := ctm.Run(m.Net, m.Steps(extraS))
		if err != nil {
			return nil, err
		}
		fits, err := m.Compare(res, sel)
		if err != nil {
			return nil, err
		}
		qf, err := m.CompareQueues(res, selQ, est.MinQueueMatch)
		if err != nil {
			return nil, err
		}
		ok := true
		for _, q := range qf {
			ok = ok && q.OK
		}
		return &FitResult{Params: p, ErrPct: MeanErrPct(fits), Demand: d, Model: m, Result: res, Fits: fits, Queues: qf, QueuesOK: ok}, nil
	}
	// search returns the index of the best candidate: lowest error among
	// those reproducing the queues, else lowest error overall.
	search := func(cands []Params, sel []TravelObs, selQ []QueueObs) (int, error) {
		errs := make([]float64, len(cands))
		oks := make([]bool, len(cands))
		fails := make([]error, len(cands))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i := range cands {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				r, err := run(cands[i], sel, selQ)
				if err != nil {
					fails[i] = err
					return
				}
				errs[i], oks[i] = r.ErrPct, r.QueuesOK
			}(i)
		}
		wg.Wait()
		best := 0
		for i := range cands {
			if fails[i] != nil {
				return 0, fails[i]
			}
			if (oks[i] && !oks[best]) || (oks[i] == oks[best] && errs[i] < errs[best]) {
				best = i
			}
		}
		return best, nil
	}

	// Start with every section free-flowing.
	cur := Params{MainPeakVPH: est.MainPeakVPH[0], PeakStartMin: est.PeakStartMin[0]}
	for _, e := range ends {
		cur.Sections = append(cur.Sections, Section{EndM: e, RampLoad: loads[0], ExitScale: scales[len(scales)-1]})
	}
	with := func(p Params, k int, load, scale float64) Params {
		p.Sections = slices.Clone(p.Sections)
		p.Sections[k].RampLoad, p.Sections[k].ExitScale = load, scale
		return p
	}
	// stage grid-searches section k's two numbers (and, for the first
	// section, the route-start traffic and peak start) against sel.
	tried := 0
	stage := func(k int, sel []TravelObs, selQ []QueueObs) error {
		var cands []Params
		mains, starts := []float64{cur.MainPeakVPH}, []float64{cur.PeakStartMin}
		if k == 0 {
			mains, starts = axis(est.MainPeakVPH), axis(est.PeakStartMin)
		}
		for _, q := range mains {
			for _, t := range starts {
				for _, l := range loads {
					for _, x := range scales {
						p := with(cur, k, l, x)
						p.MainPeakVPH, p.PeakStartMin = q, t
						cands = append(cands, p)
					}
				}
			}
		}
		best, err := search(cands, sel, selQ)
		if err != nil {
			return err
		}
		cur = cands[best]
		tried += len(cands)
		return nil
	}
	for k, end := range ends {
		var sel []TravelObs
		for _, o := range obs {
			if o.ToM <= end {
				sel = append(sel, o)
			}
		}
		var selQ []QueueObs
		for _, q := range queues {
			if q.HeadM <= end {
				selQ = append(selQ, q)
			}
		}
		if err := stage(k, sel, selQ); err != nil {
			return nil, err
		}
	}
	// A queue in a later section can reach back into an earlier one, which
	// the first pass could not know. Re-fit each section against every
	// observation, holding the others, until nothing changes.
	for round := 0; round < maxRefitRounds && len(ends) > 1; round++ {
		before := fmt.Sprint(cur)
		for k := range ends {
			if err := stage(k, obs, queues); err != nil {
				return nil, err
			}
		}
		if fmt.Sprint(cur) == before {
			break
		}
	}

	out, err := run(cur, obs, queues)
	if err != nil {
		return nil, err
	}
	out.Tried = tried
	out.UncalibratedFromM = ends[len(ends)-1]
	edge := func(name string, v float64, r [3]float64) {
		a := axis(r)
		if len(a) > 1 && (v <= a[0]+1e-9 || v >= a[len(a)-1]-1e-9) {
			out.AtBound = append(out.AtBound, name)
		}
	}
	edge("main_peak_vph", cur.MainPeakVPH, est.MainPeakVPH)
	edge("peak_start_min", cur.PeakStartMin, est.PeakStartMin)
	for k, sec := range cur.Sections {
		edge(fmt.Sprintf("section %d ramp_load", k+1), sec.RampLoad, est.RampLoad)
		edge(fmt.Sprintf("section %d exit_scale", k+1), sec.ExitScale, est.ExitScale)
	}
	return out, nil
}

// Locate projects pos onto the route and returns the distance along the
// route of the nearest point and how far pos is from it.
func Locate(g *graph.Graph, route []graph.EdgeID, pos graph.LatLon) (alongM, offM float64) {
	offM = math.Inf(1)
	at := 0.0
	const mPerDeg = 111320.0
	cos := math.Cos(pos.Lat * math.Pi / 180)
	xy := func(p graph.LatLon) (float64, float64) {
		return (p.Lon - pos.Lon) * mPerDeg * cos, (p.Lat - pos.Lat) * mPerDeg
	}
	for _, id := range route {
		e := &g.Edges[id]
		geom := e.Geometry
		if len(geom) < 2 {
			if a, b := g.Nodes[e.From], g.Nodes[e.To]; a != nil && b != nil {
				geom = []graph.LatLon{a.Pos, b.Pos}
			}
		}
		raw := 0.0
		for i := 0; i+1 < len(geom); i++ {
			raw += graph.HaversineM(geom[i], geom[i+1])
		}
		scale := 1.0
		if raw > 0 {
			scale = e.LengthM / raw
		}
		run := 0.0
		for i := 0; i+1 < len(geom); i++ {
			ax, ay := xy(geom[i])
			bx, by := xy(geom[i+1])
			dx, dy := bx-ax, by-ay
			seg := math.Hypot(dx, dy)
			t := 0.0
			if seg > 0 {
				t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/(seg*seg)))
			}
			if d := math.Hypot(ax+t*dx, ay+t*dy); d < offM {
				offM, alongM = d, at+(run+t*seg)*scale
			}
			run += seg
		}
		at += e.LengthM
	}
	return alongM, offM
}
