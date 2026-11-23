package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Phase 1.5 simulation inputs: traffic demand, observed travel times and
// queues for calibration, and surveyed signal timings. Points are keyed
// by OSM ids like LaneDrops.Confirmed: "node:<id>" or "way:<id>".

// OSMKind is the kind of OSM element a key points at.
type OSMKind string

// OSM element kinds used in keys.
const (
	OSMNode OSMKind = "node"
	OSMWay  OSMKind = "way"
)

// ParseOSMKey splits "node:<id>" or "way:<id>" into kind and positive id.
func ParseOSMKey(key string) (OSMKind, int64, error) {
	kind, idStr, ok := strings.Cut(key, ":")
	if !ok || (kind != string(OSMNode) && kind != string(OSMWay)) {
		return "", 0, fmt.Errorf("osm key %q: want node:<id> or way:<id>", key)
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return "", 0, fmt.Errorf("osm key %q: id must be a positive integer", key)
	}
	return OSMKind(kind), id, nil
}

// IsPlaceholder reports whether a source field marks a value as a guess
// still to be replaced ("placeholder - ...").
func IsPlaceholder(source string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(source)), "placeholder")
}

// IsEstimated reports whether a source field marks a value as fitted by
// cmd/calibrate rather than counted ("estimated - ...").
func IsEstimated(source string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(source)), "estimated")
}

// Estimate configures cmd/calibrate, which fills demand nobody has counted
// by fitting the simulation to observed travel times (decision 0009).
type Estimate struct {
	Profile string `json:"profile"` // name of the profile it writes
	// The fitted profile rises in a straight line from StartLevel at Start
	// to 1 at the fitted peak start, holds 1 until PeakEnd, then falls in a
	// straight line to EndLevel at End.
	Start      string  `json:"start"`
	StartLevel float64 `json:"start_level"`
	PeakEnd    string  `json:"peak_end"`
	End        string  `json:"end"`
	EndLevel   float64 `json:"end_level"`
	// MaxExitSplit caps any estimated off-ramp share.
	MaxExitSplit float64 `json:"max_exit_split"`
	// MinQueueMatch is the share of the road between an observed queue's
	// tail and head that must be queued in the simulation at that time for
	// a candidate to count as reproducing it. Candidates that miss an
	// observed queue are only used if none reproduces it.
	MinQueueMatch float64 `json:"min_queue_match"`
	// Search ranges, each [low, high, step].
	MainPeakVPH  [3]float64 `json:"main_peak_vph"`
	RampLoad     [3]float64 `json:"ramp_load"`
	ExitScale    [3]float64 `json:"exit_scale"`
	PeakStartMin [3]float64 `json:"peak_start_min"` // minutes after Start
	Source       string     `json:"source"`
}

func (e *Estimate) validate() error {
	for _, c := range []string{e.Start, e.PeakEnd, e.End} {
		if err := checkClock(c); err != nil {
			return err
		}
	}
	if e.Profile == "" || e.Source == "" || e.StartLevel <= 0 || e.StartLevel > 1 || e.EndLevel <= 0 || e.EndLevel > 1 ||
		e.MaxExitSplit <= 0 || e.MaxExitSplit >= 1 || e.MinQueueMatch <= 0 || e.MinQueueMatch > 1 {
		return fmt.Errorf("need profile, source, levels in (0,1], 0 < max_exit_split < 1 and 0 < min_queue_match <= 1")
	}
	if !(e.Start < e.PeakEnd && e.PeakEnd < e.End) {
		return fmt.Errorf("need start < peak_end < end")
	}
	for name, r := range map[string][3]float64{"main_peak_vph": e.MainPeakVPH, "ramp_load": e.RampLoad, "exit_scale": e.ExitScale, "peak_start_min": e.PeakStartMin} {
		if r[0] <= 0 || r[1] < r[0] || r[2] <= 0 {
			return fmt.Errorf("%s: want [low, high, step] with 0 < low <= high and step > 0", name)
		}
	}
	return nil
}

// Demand is the traffic entering and leaving one corridor.
type Demand struct {
	Corridor       string             `json:"corridor"`
	ProfileStepMin int                `json:"profile_step_min"`
	Profiles       map[string]Profile `json:"profiles"`
	Entries        []DemandEntry      `json:"entries"`
	Exits          []DemandExit       `json:"exits"`
	// Estimate is optional; cmd/calibrate needs it.
	Estimate *Estimate `json:"estimate,omitempty"`
	Note     string    `json:"_note,omitempty"`
}

// Profile is a time-of-day shape: Shape[i] × peak_vph is the demand in the
// i-th step (Demand.ProfileStepMin long) starting at Start ("HH:MM").
type Profile struct {
	Start  string    `json:"start"`
	Shape  []float64 `json:"shape"`
	Source string    `json:"source"`
}

// DemandEntry is traffic entering the network at an OSM node or way.
type DemandEntry struct {
	Key     string  `json:"key"`
	Name    string  `json:"name"`
	Pin     LatLon  `json:"pin"`
	PeakVPH float64 `json:"peak_vph"`
	Profile string  `json:"profile"` // name in Demand.Profiles
	Source  string  `json:"source"`
	Year    int     `json:"year"`
}

// DemandExit is an off-ramp taking SplitRatio of the mainline flow
// arriving at its diverge.
type DemandExit struct {
	Key        string  `json:"key"`
	Name       string  `json:"name"`
	Pin        LatLon  `json:"pin"`
	SplitRatio float64 `json:"split_ratio"`
	Source     string  `json:"source"`
	Year       int     `json:"year"`
}

// Observed holds real-world measurements used to calibrate the simulator.
type Observed struct {
	Corridor    string       `json:"corridor"`
	TravelTimes []TravelTime `json:"travel_times"`
	Queues      []Queue      `json:"queues"`
}

// TravelTime is a Google Maps "typical" travel time range for one
// departure time on one weekday.
type TravelTime struct {
	Name        string  `json:"name"`
	From        LatLon  `json:"from"`
	To          LatLon  `json:"to"`
	Weekday     string  `json:"weekday"` // Mon..Sun
	Depart      string  `json:"depart"`  // HH:MM
	MinutesLow  float64 `json:"minutes_low"`
	MinutesHigh float64 `json:"minutes_high"`
	DateRead    string  `json:"date_read"` // YYYY-MM-DD
	Source      string  `json:"source"`
}

// Queue is one observed queue: Head is the bottleneck, Tail how far back
// the queue reached at Time.
type Queue struct {
	Name         string `json:"name"`
	Head         LatLon `json:"head"`
	Tail         LatLon `json:"tail"`
	Weekday      string `json:"weekday"`
	Time         string `json:"time"`
	DateObserved string `json:"date_observed"`
	Source       string `json:"source"`
}

// Signals holds surveyed signal timings for one corridor.
type Signals struct {
	Corridor string   `json:"corridor"`
	Signals  []Signal `json:"signals"`
}

// Signal is one signalised junction, keyed by its OSM node.
type Signal struct {
	Key          string  `json:"key"` // node:<id>
	Name         string  `json:"name"`
	Pin          LatLon  `json:"pin"`
	CycleS       float64 `json:"cycle_s"`
	Phases       []Phase `json:"phases"`
	DateObserved string  `json:"date_observed"`
	Source       string  `json:"source"`
}

// Phase is one signal phase's green time.
type Phase struct {
	Name   string  `json:"name"`
	GreenS float64 `json:"green_s"`
}

// LoadDemand reads and validates a demand file.
func LoadDemand(path string) (*Demand, error) {
	var d Demand
	if err := load(path, &d); err != nil {
		return nil, err
	}
	if err := d.validate(); err != nil {
		return nil, fmt.Errorf("demand %s: %w", path, err)
	}
	return &d, nil
}

// LoadObserved reads and validates an observations file.
func LoadObserved(path string) (*Observed, error) {
	var o Observed
	if err := load(path, &o); err != nil {
		return nil, err
	}
	if err := o.validate(); err != nil {
		return nil, fmt.Errorf("observed %s: %w", path, err)
	}
	return &o, nil
}

// LoadSignals reads and validates a signal-timings file.
func LoadSignals(path string) (*Signals, error) {
	var s Signals
	if err := load(path, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("signals %s: %w", path, err)
	}
	return &s, nil
}

func (d *Demand) validate() error {
	if d.Corridor == "" || d.ProfileStepMin <= 0 || len(d.Entries) == 0 {
		return fmt.Errorf("need corridor, profile_step_min > 0 and at least one entry")
	}
	for name, p := range d.Profiles {
		if err := checkClock(p.Start); err != nil {
			return fmt.Errorf("profiles.%s.start: %w", name, err)
		}
		if len(p.Shape) == 0 || p.Source == "" {
			return fmt.Errorf("profiles.%s: need shape and source", name)
		}
		for i, v := range p.Shape {
			if v < 0 {
				return fmt.Errorf("profiles.%s.shape[%d]: negative", name, i)
			}
		}
	}
	if d.Estimate != nil {
		if err := d.Estimate.validate(); err != nil {
			return fmt.Errorf("estimate: %w", err)
		}
	}
	for i, e := range d.Entries {
		if _, _, err := ParseOSMKey(e.Key); err != nil {
			return fmt.Errorf("entries[%d]: %w", i, err)
		}
		if e.Name == "" || e.PeakVPH <= 0 || e.Source == "" || e.Year <= 0 {
			return fmt.Errorf("entries[%d]: need name, peak_vph > 0, source and year", i)
		}
		if _, ok := d.Profiles[e.Profile]; !ok {
			return fmt.Errorf("entries[%d]: unknown profile %q", i, e.Profile)
		}
	}
	for i, x := range d.Exits {
		if _, _, err := ParseOSMKey(x.Key); err != nil {
			return fmt.Errorf("exits[%d]: %w", i, err)
		}
		if x.Name == "" || x.SplitRatio < 0 || x.SplitRatio > 1 || x.Source == "" || x.Year <= 0 {
			return fmt.Errorf("exits[%d]: need name, 0 <= split_ratio <= 1, source and year", i)
		}
	}
	return nil
}

func (o *Observed) validate() error {
	if o.Corridor == "" {
		return fmt.Errorf("need corridor")
	}
	for i, t := range o.TravelTimes {
		if t.Name == "" || t.Source == "" {
			return fmt.Errorf("travel_times[%d]: need name and source", i)
		}
		if err := checkWhen(t.Weekday, t.Depart, t.DateRead); err != nil {
			return fmt.Errorf("travel_times[%d]: %w", i, err)
		}
		if t.MinutesLow <= 0 || t.MinutesHigh < t.MinutesLow {
			return fmt.Errorf("travel_times[%d]: need 0 < minutes_low <= minutes_high", i)
		}
	}
	for i, q := range o.Queues {
		if q.Name == "" || q.Source == "" {
			return fmt.Errorf("queues[%d]: need name and source", i)
		}
		if err := checkWhen(q.Weekday, q.Time, q.DateObserved); err != nil {
			return fmt.Errorf("queues[%d]: %w", i, err)
		}
	}
	return nil
}

func (s *Signals) validate() error {
	if s.Corridor == "" {
		return fmt.Errorf("need corridor")
	}
	for i, sg := range s.Signals {
		kind, _, err := ParseOSMKey(sg.Key)
		if err != nil {
			return fmt.Errorf("signals[%d]: %w", i, err)
		}
		if kind != OSMNode {
			return fmt.Errorf("signals[%d]: key must be node:<id>, got %q", i, sg.Key)
		}
		if sg.Name == "" || sg.Source == "" || sg.CycleS <= 0 || len(sg.Phases) == 0 {
			return fmt.Errorf("signals[%d]: need name, source, cycle_s > 0 and phases", i)
		}
		if _, err := time.Parse(time.DateOnly, sg.DateObserved); err != nil {
			return fmt.Errorf("signals[%d]: date_observed %q: want YYYY-MM-DD", i, sg.DateObserved)
		}
		var green float64
		for j, p := range sg.Phases {
			if p.GreenS <= 0 {
				return fmt.Errorf("signals[%d].phases[%d]: need green_s > 0", i, j)
			}
			green += p.GreenS
		}
		if green > sg.CycleS {
			return fmt.Errorf("signals[%d]: total green %.0f s exceeds cycle %.0f s", i, green, sg.CycleS)
		}
	}
	return nil
}

var weekdays = map[string]bool{"Mon": true, "Tue": true, "Wed": true, "Thu": true, "Fri": true, "Sat": true, "Sun": true}

func checkWhen(weekday, clock, date string) error {
	if !weekdays[weekday] {
		return fmt.Errorf("weekday %q: want Mon..Sun", weekday)
	}
	if err := checkClock(clock); err != nil {
		return err
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return fmt.Errorf("date %q: want YYYY-MM-DD", date)
	}
	return nil
}

func checkClock(s string) error {
	if _, err := time.Parse("15:04", s); err != nil {
		return fmt.Errorf("time %q: want HH:MM", s)
	}
	return nil
}
