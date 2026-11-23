// Package config loads and validates corridor and capacity config files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// LatLon is a WGS84 coordinate.
type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Gate is a named entry/exit point snapped to the nearest graph node.
type Gate struct {
	Name string `json:"name"`
	Pos  LatLon `json:"pos"`
	Role string `json:"role"` // "west" | "east"; direction decides which is source
}

// Corridor describes one study area.
type Corridor struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	BBox  [4]float64 `json:"bbox"` // minLon, minLat, maxLon, maxLat
	Gates []Gate     `json:"gates"`
	// GateSnap says how gates attach to the graph; required.
	GateSnap GateSnap `json:"gate_snap"`
	// LaneDrops configures lane-drop detection; nil disables it.
	LaneDrops *LaneDrops `json:"lane_drops"`
}

// GateSnap configures per-direction gate snapping (ARCHITECTURE §3.3).
type GateSnap struct {
	// Classes a gate may snap onto, e.g. the highway mainline classes.
	// Keeps a gate between two carriageways off nearby side streets.
	Classes []string `json:"classes"`
	// MaxM is how far from the gate a snapped node may be.
	MaxM   float64 `json:"max_m"`
	Source string  `json:"source"`
}

// LaneDrops are the settings for finding places where lanes run out.
type LaneDrops struct {
	// RouteRefs are the road refs (e.g. "E5") that make up the corridor
	// route; only drops on these roads or their ramps are reported.
	RouteRefs []string `json:"route_refs"`
	// Classes are the highway classes checked (grade-separated roads).
	Classes []string `json:"classes"`
	// TollPlazaRadiusM skips drops this close to a toll booth: plazas widen
	// briefly and narrow back, which is not a real loss of lanes.
	TollPlazaRadiusM float64 `json:"toll_plaza_radius_m"`
	Source           string  `json:"source"`
	// Confirmed are drops checked on the ground, matched by Key.
	Confirmed []ConfirmedDrop `json:"confirmed"`
}

// ConfirmedDrop marks one detected drop as checked by a person.
type ConfirmedDrop struct {
	// Key is "node:<osm node id>" for a merge or "way:<from>><to>" for a
	// narrowing between two OSM ways, as printed by cmd/analyze.
	Key    string `json:"key"`
	Name   string `json:"name"`
	Pin    LatLon `json:"pin"` // where the person reported it
	Source string `json:"source"`
}

func (l *LaneDrops) validate() error {
	if len(l.RouteRefs) == 0 || len(l.Classes) == 0 || l.TollPlazaRadiusM <= 0 || l.Source == "" {
		return fmt.Errorf("lane_drops: need route_refs, classes, toll_plaza_radius_m > 0 and source")
	}
	for i, c := range l.Confirmed {
		if c.Key == "" || c.Name == "" || c.Source == "" {
			return fmt.Errorf("lane_drops.confirmed[%d]: need key, name and source", i)
		}
	}
	return nil
}

// ClassDefaults are per-road-class assumptions.
type ClassDefaults struct {
	PerLaneVPH   int    `json:"per_lane_vph"`
	DefaultLanes int    `json:"default_lanes"` // per direction, used when untagged
	Source       string `json:"source"`
}

// Capacity holds all capacity assumptions.
type Capacity struct {
	Classes      map[string]ClassDefaults `json:"classes"`
	SignalFactor float64                  `json:"signal_factor"`
	SignalSource string                   `json:"signal_source"`
	// Phase 1.5 simulation parameters; required by LoadCapacity.
	CTM            map[string]CTMClass `json:"ctm"`
	ALINEA         ALINEA              `json:"alinea"`
	Signals        SignalParams        `json:"signals"`
	Channelisation Channelisation      `json:"channelisation"`
	Restripe       Restripe            `json:"restripe"`
}

// CTMClass is the triangular fundamental diagram for one road class in
// the Cell Transmission Model (Daganzo 1994). Per-lane capacity is the
// class's PerLaneVPH; FreeFlowKMH × CriticalDensity should equal it.
type CTMClass struct {
	FreeFlowKMH     float64 `json:"free_flow_kmh"`
	CriticalDensity float64 `json:"critical_density_vpkmpl"` // veh/km/lane
	JamDensity      float64 `json:"jam_density_vpkmpl"`      // veh/km/lane
	// CapacityDrop is the fraction of capacity lost once a queue forms
	// (queue discharge rate = capacity × (1 − CapacityDrop)).
	CapacityDrop float64 `json:"capacity_drop_fraction"`
	Source       string  `json:"source"`
}

// ALINEA holds ramp-metering controller settings (Papageorgiou 1991):
// r(k) = r(k−1) + GainVPHPerPct × (TargetOccupancyPct − occupancy(k)).
type ALINEA struct {
	GainVPHPerPct      float64 `json:"gain_vph_per_pct"`
	TargetOccupancyPct float64 `json:"target_occupancy_pct"`
	// MinRateVPH is the lowest rate the meter may fall to; UpdateS is how
	// often it adjusts.
	MinRateVPH float64 `json:"min_rate_vph"`
	UpdateS    float64 `json:"update_s"`
	Source     string  `json:"source"`
}

// Restripe is the re-striping fix: the joining ramp keeps its own lane
// for AuxLaneM before merging (decision 0008).
type Restripe struct {
	AuxLaneM float64 `json:"aux_lane_m"`
	Source   string  `json:"source"`
}

// SignalParams are signal-timing defaults (Webster 1958, HCM).
type SignalParams struct {
	SaturationFlowVPHPL float64 `json:"saturation_flow_vphpl"`
	LostTimePerPhaseS   float64 `json:"lost_time_per_phase_s"`
	Source              string  `json:"source"`
}

// Channelisation is the assumed effect of a divider or solid line that
// stops late lane changes at a merge.
type Channelisation struct {
	// CapacityDropReduction is the share of CTMClass.CapacityDrop removed.
	CapacityDropReduction float64 `json:"capacity_drop_reduction"`
	Confidence            string  `json:"confidence"` // "low" | "medium" | "high"
	Source                string  `json:"source"`
}

func (c *Capacity) validate() error {
	if len(c.Classes) == 0 || c.SignalFactor <= 0 || c.SignalFactor > 1 || c.SignalSource == "" {
		return fmt.Errorf("need classes, 0 < signal_factor <= 1 and signal_source")
	}
	for name, cl := range c.Classes {
		if cl.PerLaneVPH <= 0 || cl.DefaultLanes <= 0 || cl.Source == "" {
			return fmt.Errorf("classes.%s: need per_lane_vph > 0, default_lanes > 0 and source", name)
		}
		ctm, ok := c.CTM[name]
		if !ok {
			return fmt.Errorf("ctm.%s: missing (every class needs CTM parameters)", name)
		}
		if err := ctm.validate(); err != nil {
			return fmt.Errorf("ctm.%s: %w", name, err)
		}
	}
	a := c.ALINEA
	if a.GainVPHPerPct <= 0 || a.TargetOccupancyPct <= 0 || a.TargetOccupancyPct >= 100 || a.Source == "" {
		return fmt.Errorf("alinea: need gain_vph_per_pct > 0, 0 < target_occupancy_pct < 100 and source")
	}
	if a.MinRateVPH <= 0 || a.UpdateS <= 0 {
		return fmt.Errorf("alinea: need min_rate_vph > 0 and update_s > 0")
	}
	if c.Restripe.AuxLaneM <= 0 || c.Restripe.Source == "" {
		return fmt.Errorf("restripe: need aux_lane_m > 0 and source")
	}
	s := c.Signals
	if s.SaturationFlowVPHPL <= 0 || s.LostTimePerPhaseS < 0 || s.Source == "" {
		return fmt.Errorf("signals: need saturation_flow_vphpl > 0, lost_time_per_phase_s >= 0 and source")
	}
	ch := c.Channelisation
	if ch.CapacityDropReduction < 0 || ch.CapacityDropReduction > 1 || ch.Source == "" {
		return fmt.Errorf("channelisation: need 0 <= capacity_drop_reduction <= 1 and source")
	}
	switch ch.Confidence {
	case "low", "medium", "high":
	default:
		return fmt.Errorf("channelisation: confidence must be low, medium or high, got %q", ch.Confidence)
	}
	return nil
}

func (c CTMClass) validate() error {
	if c.FreeFlowKMH <= 0 || c.CriticalDensity <= 0 || c.JamDensity <= c.CriticalDensity {
		return fmt.Errorf("need free_flow_kmh > 0 and 0 < critical_density < jam_density")
	}
	if c.CapacityDrop < 0 || c.CapacityDrop >= 1 {
		return fmt.Errorf("need 0 <= capacity_drop_fraction < 1")
	}
	if c.Source == "" {
		return fmt.Errorf("missing source")
	}
	return nil
}

// LoadCorridor reads and validates a corridor config.
func LoadCorridor(path string) (*Corridor, error) {
	var c Corridor
	if err := load(path, &c); err != nil {
		return nil, err
	}
	if c.ID == "" || len(c.Gates) < 2 {
		return nil, fmt.Errorf("corridor %s: need id and at least 2 gates", path)
	}
	if len(c.GateSnap.Classes) == 0 || c.GateSnap.MaxM <= 0 || c.GateSnap.Source == "" {
		return nil, fmt.Errorf("corridor %s: gate_snap needs classes, max_m > 0 and source", path)
	}
	roles := map[string]int{}
	for _, gt := range c.Gates {
		roles[gt.Role]++
	}
	if roles["west"] == 0 || roles["east"] == 0 || roles["west"]+roles["east"] != len(c.Gates) {
		return nil, fmt.Errorf("corridor %s: gates need role west or east, at least one of each", path)
	}
	if c.LaneDrops != nil {
		if err := c.LaneDrops.validate(); err != nil {
			return nil, fmt.Errorf("corridor %s: %w", path, err)
		}
	}
	return &c, nil
}

// LoadCapacity reads and validates capacity defaults.
func LoadCapacity(path string) (*Capacity, error) {
	var c Capacity
	if err := load(path, &c); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("capacity %s: %w", path, err)
	}
	return &c, nil
}

func load(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
