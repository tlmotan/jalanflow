package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repoConfig = "../../config"

// The committed config files must always load.
func TestRepoConfigsLoad(t *testing.T) {
	if _, err := LoadCapacity(filepath.Join(repoConfig, "capacity-defaults.json")); err != nil {
		t.Errorf("capacity: %v", err)
	}
	if _, err := LoadDemand(filepath.Join(repoConfig, "demand", "kemuning-pj.json")); err != nil {
		t.Errorf("demand: %v", err)
	}
	if _, err := LoadObserved(filepath.Join(repoConfig, "observed", "kemuning-pj.json")); err != nil {
		t.Errorf("observed: %v", err)
	}
	if _, err := LoadSignals(filepath.Join(repoConfig, "signals", "kemuning-pj.json")); err != nil {
		t.Errorf("signals: %v", err)
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	missingSource := filepath.Join(dir, "nosource.json")
	if err := os.WriteFile(missingSource, []byte(`{"corridor":"x","signals":[{"key":"node:1","name":"a","cycle_s":60,"phases":[{"green_s":20}],"date_observed":"2026-11-23"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, path, want string
	}{
		{"missing file", filepath.Join(dir, "nope.json"), "read"},
		{"bad json", bad, "parse"},
		{"missing source", missingSource, "source"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadSignals(tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestParseOSMKey(t *testing.T) {
	tests := []struct {
		key     string
		kind    OSMKind
		id      int64
		wantErr bool
	}{
		{"node:33987957", OSMNode, 33987957, false},
		{"way:24994898", OSMWay, 24994898, false},
		{"relation:1", "", 0, true},
		{"node:", "", 0, true},
		{"node:-5", "", 0, true},
		{"node:12x", "", 0, true},
		{"33987957", "", 0, true},
		{"way:1>2", "", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			kind, id, err := ParseOSMKey(tc.key)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if kind != tc.kind || id != tc.id {
				t.Fatalf("got %s %d, want %s %d", kind, id, tc.kind, tc.id)
			}
		})
	}
}

func TestIsPlaceholder(t *testing.T) {
	tests := map[string]bool{
		"placeholder - calibrate":   true,
		" Placeholder: guess":       true,
		"HCM 2010":                  false,
		"Tim, stopwatch 2026-11-23": false,
		"":                          false,
	}
	for in, want := range tests {
		if got := IsPlaceholder(in); got != want {
			t.Errorf("IsPlaceholder(%q) = %v, want %v", in, got, want)
		}
	}
}

func validCapacity() *Capacity {
	return &Capacity{
		Classes:      map[string]ClassDefaults{"motorway": {PerLaneVPH: 2000, DefaultLanes: 3, Source: "s"}},
		SignalFactor: 0.5,
		SignalSource: "s",
		CTM: map[string]CTMClass{"motorway": {
			FreeFlowKMH: 100, CriticalDensity: 20, JamDensity: 150, CapacityDrop: 0.1, Source: "s",
		}},
		ALINEA:         ALINEA{GainVPHPerPct: 70, TargetOccupancyPct: 13, MinRateVPH: 240, UpdateS: 60, Source: "s"},
		Signals:        SignalParams{SaturationFlowVPHPL: 1900, LostTimePerPhaseS: 4, Source: "s"},
		Channelisation: Channelisation{CapacityDropReduction: 0.5, Confidence: "low", Source: "s"},
		Restripe:       Restripe{AuxLaneM: 500, Source: "s"},
	}
}

func TestCapacityValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Capacity)
		want   string // "" = valid
	}{
		{"valid", func(*Capacity) {}, ""},
		{"class missing source", func(c *Capacity) {
			c.Classes["motorway"] = ClassDefaults{PerLaneVPH: 2000, DefaultLanes: 3}
		}, "classes.motorway"},
		{"signal source missing", func(c *Capacity) { c.SignalSource = "" }, "signal_source"},
		{"ctm missing for class", func(c *Capacity) { delete(c.CTM, "motorway") }, "ctm.motorway: missing"},
		{"ctm missing source", func(c *Capacity) {
			m := c.CTM["motorway"]
			m.Source = ""
			c.CTM["motorway"] = m
		}, "missing source"},
		{"ctm critical >= jam", func(c *Capacity) {
			m := c.CTM["motorway"]
			m.CriticalDensity = 150
			c.CTM["motorway"] = m
		}, "critical_density"},
		{"ctm capacity drop 1", func(c *Capacity) {
			m := c.CTM["motorway"]
			m.CapacityDrop = 1
			c.CTM["motorway"] = m
		}, "capacity_drop_fraction"},
		{"alinea missing source", func(c *Capacity) { c.ALINEA.Source = "" }, "alinea"},
		{"alinea occupancy 100", func(c *Capacity) { c.ALINEA.TargetOccupancyPct = 100 }, "alinea"},
		{"alinea no minimum rate", func(c *Capacity) { c.ALINEA.MinRateVPH = 0 }, "min_rate_vph"},
		{"restripe missing source", func(c *Capacity) { c.Restripe.Source = "" }, "restripe"},
		{"signals missing source", func(c *Capacity) { c.Signals.Source = "" }, "signals"},
		{"signals zero saturation", func(c *Capacity) { c.Signals.SaturationFlowVPHPL = 0 }, "signals"},
		{"channelisation missing source", func(c *Capacity) { c.Channelisation.Source = "" }, "channelisation"},
		{"channelisation reduction > 1", func(c *Capacity) { c.Channelisation.CapacityDropReduction = 1.5 }, "channelisation"},
		{"channelisation bad confidence", func(c *Capacity) { c.Channelisation.Confidence = "" }, "confidence"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validCapacity()
			tc.mutate(c)
			checkErr(t, c.validate(), tc.want)
		})
	}
}

func validDemand() *Demand {
	return &Demand{
		Corridor:       "kemuning-pj",
		ProfileStepMin: 15,
		Profiles:       map[string]Profile{"am": {Start: "06:00", Shape: []float64{0.5, 1}, Source: "s"}},
		Entries:        []DemandEntry{{Key: "node:1", Name: "in", PeakVPH: 4000, Profile: "am", Source: "s", Year: 2026}},
		Exits:          []DemandExit{{Key: "way:2", Name: "out", SplitRatio: 0.2, Source: "s", Year: 2026}},
	}
}

func TestDemandValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Demand)
		want   string
	}{
		{"valid", func(*Demand) {}, ""},
		{"no entries", func(d *Demand) { d.Entries = nil }, "at least one entry"},
		{"zero step", func(d *Demand) { d.ProfileStepMin = 0 }, "profile_step_min"},
		{"profile missing source", func(d *Demand) {
			d.Profiles["am"] = Profile{Start: "06:00", Shape: []float64{1}}
		}, "profiles.am"},
		{"profile bad start", func(d *Demand) {
			d.Profiles["am"] = Profile{Start: "6am", Shape: []float64{1}, Source: "s"}
		}, "HH:MM"},
		{"profile negative", func(d *Demand) {
			d.Profiles["am"] = Profile{Start: "06:00", Shape: []float64{-1}, Source: "s"}
		}, "negative"},
		{"entry missing source", func(d *Demand) { d.Entries[0].Source = "" }, "entries[0]"},
		{"entry bad key", func(d *Demand) { d.Entries[0].Key = "3.02,101.5" }, "osm key"},
		{"entry zero peak", func(d *Demand) { d.Entries[0].PeakVPH = 0 }, "peak_vph"},
		{"entry unknown profile", func(d *Demand) { d.Entries[0].Profile = "pm" }, "unknown profile"},
		{"entry no year", func(d *Demand) { d.Entries[0].Year = 0 }, "year"},
		{"exit missing source", func(d *Demand) { d.Exits[0].Source = "" }, "exits[0]"},
		{"exit split > 1", func(d *Demand) { d.Exits[0].SplitRatio = 1.2 }, "split_ratio"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := validDemand()
			tc.mutate(d)
			checkErr(t, d.validate(), tc.want)
		})
	}
}

func validObserved() *Observed {
	return &Observed{
		Corridor: "kemuning-pj",
		TravelTimes: []TravelTime{{
			Name: "in", Weekday: "Tue", Depart: "07:30", MinutesLow: 30, MinutesHigh: 55,
			DateRead: "2026-11-23", Source: "s",
		}},
		Queues: []Queue{{Name: "q", Weekday: "Tue", Time: "07:45", DateObserved: "2026-11-23", Source: "s"}},
	}
}

func TestObservedValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Observed)
		want   string
	}{
		{"valid", func(*Observed) {}, ""},
		{"no corridor", func(o *Observed) { o.Corridor = "" }, "corridor"},
		{"travel time missing source", func(o *Observed) { o.TravelTimes[0].Source = "" }, "travel_times[0]"},
		{"bad weekday", func(o *Observed) { o.TravelTimes[0].Weekday = "Tuesday" }, "weekday"},
		{"bad depart", func(o *Observed) { o.TravelTimes[0].Depart = "25:00" }, "HH:MM"},
		{"bad date", func(o *Observed) { o.TravelTimes[0].DateRead = "23/11/2026" }, "YYYY-MM-DD"},
		{"low > high", func(o *Observed) { o.TravelTimes[0].MinutesLow = 60 }, "minutes_low"},
		{"queue missing source", func(o *Observed) { o.Queues[0].Source = "" }, "queues[0]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := validObserved()
			tc.mutate(o)
			checkErr(t, o.validate(), tc.want)
		})
	}
}

func validSignals() *Signals {
	return &Signals{
		Corridor: "kemuning-pj",
		Signals: []Signal{{
			Key: "node:271724963", Name: "j", CycleS: 120,
			Phases:       []Phase{{Name: "a", GreenS: 50}, {Name: "b", GreenS: 50}},
			DateObserved: "2026-11-23", Source: "s",
		}},
	}
}

func TestSignalsValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Signals)
		want   string
	}{
		{"valid", func(*Signals) {}, ""},
		{"missing source", func(s *Signals) { s.Signals[0].Source = "" }, "source"},
		{"way key", func(s *Signals) { s.Signals[0].Key = "way:5" }, "node:<id>"},
		{"no phases", func(s *Signals) { s.Signals[0].Phases = nil }, "phases"},
		{"zero green", func(s *Signals) { s.Signals[0].Phases[0].GreenS = 0 }, "green_s"},
		{"green exceeds cycle", func(s *Signals) { s.Signals[0].CycleS = 90 }, "exceeds cycle"},
		{"bad date", func(s *Signals) { s.Signals[0].DateObserved = "" }, "date_observed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := validSignals()
			tc.mutate(s)
			checkErr(t, s.validate(), tc.want)
		})
	}
}

func checkErr(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want containing %q", err, want)
	}
}
