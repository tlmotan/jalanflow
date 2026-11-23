// Command calibrate estimates the traffic nobody has counted: it fits the
// corridor simulation to observed travel times and writes the result into
// the demand config, every figure marked "estimated" (decision 0009).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"slices"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/corridor"
	"github.com/tlmotan/jalanflow/internal/graph"
)

func main() {
	graphPath := flag.String("graph", "", "graph.json from build-graph")
	corridorPath := flag.String("corridor", "", "corridor config json")
	capacityPath := flag.String("capacity", "config/capacity-defaults.json", "capacity defaults")
	demandPath := flag.String("demand", "", "demand config json (read, and rewritten unless -dry)")
	observedPath := flag.String("observed", "", "observed travel times and queues json")
	dt := flag.Float64("dt", 5, "simulation time step, seconds")
	dry := flag.Bool("dry", false, "report the fit without writing the demand file")
	fixes := flag.Bool("fixes", false, "also simulate every fix on the fitted baseline and print the ranking")
	flag.Parse()
	if *graphPath == "" || *corridorPath == "" || *demandPath == "" || *observedPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*graphPath, *corridorPath, *capacityPath, *demandPath, *observedPath, *dt, *dry, *fixes); err != nil {
		fmt.Fprintln(os.Stderr, "calibrate:", err)
		os.Exit(1)
	}
}

func run(graphPath, corridorPath, capacityPath, demandPath, observedPath string, dt float64, dry, showFixes bool) error {
	c, err := config.LoadCorridor(corridorPath)
	if err != nil {
		return err
	}
	if c.LaneDrops == nil {
		return fmt.Errorf("%s: no lane_drops block (it names the route refs)", corridorPath)
	}
	capCfg, err := config.LoadCapacity(capacityPath)
	if err != nil {
		return err
	}
	base, err := config.LoadDemand(demandPath)
	if err != nil {
		return err
	}
	observed, err := config.LoadObserved(observedPath)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(graphPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", graphPath, err)
	}
	var g graph.Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return fmt.Errorf("parse %s: %w", graphPath, err)
	}

	// Inbound: west gate → east gate, as in cmd/analyze.
	var west, east graph.LatLon
	for _, gt := range c.Gates {
		p := graph.LatLon{Lat: gt.Pos.Lat, Lon: gt.Pos.Lon}
		if gt.Role == "west" {
			west = p
		} else {
			east = p
		}
	}
	src, err := graph.SnapGate(&g, west, east, graph.SourceEnd, c.GateSnap.Classes, c.GateSnap.MaxM)
	if err != nil {
		return err
	}
	dst, err := graph.SnapGate(&g, east, west, graph.SinkEnd, c.GateSnap.Classes, c.GateSnap.MaxM)
	if err != nil {
		return err
	}
	opt := corridor.Options{RouteRefs: c.LaneDrops.RouteRefs, MainClasses: c.GateSnap.Classes, DtS: dt}
	for _, cl := range c.LaneDrops.Classes {
		if !slices.Contains(opt.MainClasses, cl) {
			opt.RampClasses = append(opt.RampClasses, cl)
		}
	}
	route, err := corridor.Route(&g, capCfg, src, dst, opt)
	if err != nil {
		return err
	}

	var obs []corridor.TravelObs
	year, read := 0, ""
	for _, t := range observed.TravelTimes {
		if config.IsPlaceholder(t.Source) {
			continue
		}
		from, offA := corridor.Locate(&g, route, graph.LatLon{Lat: t.From.Lat, Lon: t.From.Lon})
		to, offB := corridor.Locate(&g, route, graph.LatLon{Lat: t.To.Lat, Lon: t.To.Lon})
		if to <= from {
			return fmt.Errorf("observation %q runs against the inbound route", t.Name)
		}
		if len(obs) == 0 || obs[len(obs)-1].Name != t.Name {
			fmt.Fprintf(os.Stderr, "calibrate: %q → %.2f km to %.2f km along the route (pins %.0f m and %.0f m off it)\n", t.Name, from/1000, to/1000, offA, offB)
		}
		obs = append(obs, corridor.TravelObs{Name: t.Name, Depart: t.Depart, FromM: from, ToM: to, Minutes: (t.MinutesLow + t.MinutesHigh) / 2})
		fmt.Sscanf(t.DateRead, "%d", &year)
		read = t.DateRead
	}
	var queues []corridor.QueueObs
	for _, q := range observed.Queues {
		if config.IsPlaceholder(q.Source) {
			continue
		}
		head, _ := corridor.Locate(&g, route, graph.LatLon{Lat: q.Head.Lat, Lon: q.Head.Lon})
		tail, _ := corridor.Locate(&g, route, graph.LatLon{Lat: q.Tail.Lat, Lon: q.Tail.Lon})
		queues = append(queues, corridor.QueueObs{Name: q.Name, Clock: q.Time, TailM: tail, HeadM: head})
	}
	source := fmt.Sprintf("estimated - fitted to %d Google Maps typical travel times read %s and %d observed queue(s) (cmd/calibrate); not a count", len(obs), read, len(queues))
	fit, err := corridor.Fit(&g, capCfg, base, route, opt, obs, queues, 3600, year, source)
	if err != nil {
		return err
	}

	p := fit.Params
	fmt.Fprintf(os.Stderr, "calibrate: tried %d combinations; best mean error %.1f%%\n", fit.Tried, fit.ErrPct)
	fmt.Fprintf(os.Stderr, "  traffic entering at route start: %.0f veh/h at peak, reached %.0f min after %s\n", p.MainPeakVPH, p.PeakStartMin, base.Estimate.Start)
	for k, sec := range p.Sections {
		fmt.Fprintf(os.Stderr, "  section %d (to %.2f km): on-ramps %.0f%% of their capacity, off-ramps %.2f × lane share\n", k+1, sec.EndM/1000, sec.RampLoad*100, sec.ExitScale)
	}
	fmt.Fprintf(os.Stderr, "  past %.2f km nothing is observed: the model there is NOT calibrated\n", fit.UncalibratedFromM/1000)
	if len(fit.AtBound) > 0 {
		fmt.Fprintf(os.Stderr, "  WARNING: best value sits on the edge of the search range for %v; widen it\n", fit.AtBound)
	}
	for _, f := range fit.Fits {
		late := ""
		if !f.Arrived {
			late = " (had not arrived when the run ended)"
		}
		fmt.Fprintf(os.Stderr, "  %-58s %s  observed %4.1f  simulated %5.1f min  off by %4.1f%%%s\n", f.Name, f.Depart, f.Minutes, f.SimMinutes, f.ErrPct(), late)
	}

	m, res := fit.Model, fit.Result
	for _, q := range fit.Queues {
		verdict := "reproduced"
		if !q.OK {
			verdict = "NOT reproduced"
		}
		fmt.Fprintf(os.Stderr, "  queue %q at %s (%.2f km to %.2f km): %.0f%% queued in the simulation, %s\n", q.Name, q.Clock, q.TailM/1000, q.HeadM/1000, q.Match*100, verdict)
	}
	if !fit.QueuesOK {
		fmt.Fprintln(os.Stderr, "  WARNING: no combination reproduced every observed queue; using the best travel-time match")
	}
	// Where the simulation puts its queues at 30-minute marks, to eyeball.
	for _, clock := range []string{"07:00", "08:00", "09:00"} {
		step, err := m.StepAt(clock)
		if err != nil || step >= len(res.Vehicles) {
			continue
		}
		line := ""
		start := -1.0
		for i, c := range m.Mainline {
			info := m.Cells[c]
			q := res.Queued(m.Net, step, c)
			if q && start < 0 {
				start = info.StartM
			}
			if start >= 0 && (!q || i == len(m.Mainline)-1) {
				line += fmt.Sprintf(" %.1f–%.1f km", start/1000, info.StartM/1000)
				start = -1
			}
		}
		if line == "" {
			line = " none"
		}
		fmt.Fprintf(os.Stderr, "  simulated queues at %s:%s\n", clock, line)
	}
	fmt.Fprintf(os.Stderr, "calibrate: %d figures estimated, %d gaps left\n", m.Estimated, len(m.Gaps))

	if showFixes {
		set, err := m.Scenarios(capCfg, obs, fit.UncalibratedFromM, 3600)
		if err != nil {
			return err
		}
		queues := func(qs [][2]float64) string {
			out := ""
			for _, q := range qs {
				out += fmt.Sprintf(" %.1f–%.1f", q[0]/1000, q[1]/1000)
			}
			if out == "" {
				return " none"
			}
			return out
		}
		b := set.Baseline
		fmt.Fprintf(os.Stderr, "fixes: baseline delay %.0f veh·h, commute %.1f min, %.0f vehicles unserved at the end; queues at %s (km):%s\n",
			b.DelayVehH, b.CommuteMin, b.UnservedVeh, set.AtClock, queues(b.Queues))
		small := 0
		for _, sc := range set.Scenarios {
			if math.Abs(sc.DelaySavedVehH) < 1 {
				small++
				continue
			}
			flag := ""
			if !sc.Calibrated {
				flag = " [uncalibrated stretch]"
			}
			fmt.Fprintf(os.Stderr, "  %+7.0f veh·h  commute %+5.1f min  %-10s at %5.2f km %-22s %s%s\n      %s; queues then:%s\n",
				sc.DelaySavedVehH, -sc.CommuteSavedMin, sc.Type, sc.Site.AtM/1000, sc.Site.Key, sc.Site.Name, flag, sc.Change, queues(sc.Outcome.Queues))
		}
		fmt.Fprintf(os.Stderr, "  %d more fixes change delay by under 1 veh·h\n", small)
	}
	if dry {
		return nil
	}
	out, err := json.MarshalIndent(fit.Demand, "", "  ")
	if err != nil {
		return fmt.Errorf("encode demand: %w", err)
	}
	if err := os.WriteFile(demandPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", demandPath, err)
	}
	fmt.Fprintf(os.Stderr, "calibrate: wrote %s\n", demandPath)
	return nil
}
