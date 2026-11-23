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
	if len(c.Classes) == 0 || c.SignalFactor <= 0 || c.SignalFactor > 1 {
		return nil, fmt.Errorf("capacity %s: need classes and 0 < signal_factor <= 1", path)
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
