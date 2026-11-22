// Package export writes edges.geojson, fixes.json, lane_drops.json and
// manifest.json.
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tlmotan/jalanflow/internal/lanedrop"
)

// LaneDropsFile is lane_drops.json (mirror in web/src/types.ts).
type LaneDropsFile struct {
	Corridor    string `json:"corridor"`
	OSMDate     string `json:"osm_date"`
	GeneratedAt string `json:"generated_at"`
	// Ranking explains the order of Drops in plain words.
	Ranking     string          `json:"ranking"`
	Assumptions []string        `json:"assumptions"`
	Drops       []lanedrop.Drop `json:"drops"`
}

// WriteLaneDrops writes dir/lane_drops.json.
func WriteLaneDrops(dir string, f LaneDropsFile) error {
	return writeJSON(filepath.Join(dir, "lane_drops.json"), f)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Manifest records how a data release was produced.
type Manifest struct {
	Corridor           string         `json:"corridor"`
	GeneratedAt        string         `json:"generated_at"`
	OSMDate            string         `json:"osm_date"`
	PipelineVersion    string         `json:"pipeline_version"`
	CapacityConfigHash string         `json:"capacity_config_hash"`
	MaxFlowVPH         map[string]int `json:"max_flow_vph"`
	Assumptions        []string       `json:"assumptions"`
}

// EdgeProps are the GeoJSON feature properties (mirror in web/src/types.ts).
type EdgeProps struct {
	ID            int     `json:"id"`
	Dir           string  `json:"dir"`
	Name          string  `json:"name"`
	Ref           string  `json:"ref"`
	Class         string  `json:"class"`
	Lanes         int     `json:"lanes"`
	LanesInferred bool    `json:"lanes_inferred"`
	CapacityVPH   int     `json:"capacity_vph"`
	FlowVPH       int     `json:"flow_vph"`
	Utilisation   float64 `json:"utilisation"`
	IsCut         bool    `json:"is_cut"`
}

// TODO(phase1): WriteEdges, WriteFixes, WriteManifest.
