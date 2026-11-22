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
