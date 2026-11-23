// Package export writes edges.geojson, fixes.json, replay.json,
// lane_drops.json and manifest.json.
package export

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/tlmotan/jalanflow/internal/fixes"
	"github.com/tlmotan/jalanflow/internal/graph"
	"github.com/tlmotan/jalanflow/internal/lanedrop"
)

// PipelineVersion is written to manifest.json; bump on output changes.
const PipelineVersion = "0.3.0"

// ConfigHash is the manifest's capacity_config_hash for the bytes of
// config/capacity-defaults.json. The committed-data test uses it to spot
// data that was not regenerated after a capacity change.
func ConfigHash(b []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
}

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
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return writeFile(path, b)
}

func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
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

type feature struct {
	Type     string    `json:"type"`
	Geometry geometry  `json:"geometry"`
	Props    EdgeProps `json:"properties"`
}

type geometry struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

// edgeFeatures builds one GeoJSON feature per edge that carries flow in
// that direction (cut edges always do), plus edges that carried flow at
// some step of the replay and lost it again when a later path cancelled
// it (flow_vph 0), so replay.json never names an edge with no geometry.
// Other roads are left out: the map shows what the max flow uses, and the
// whole bbox twice over is ~19 MB. Coordinates are rounded to 6 decimals.
func edgeFeatures(g *graph.Graph, r fixes.Result) []feature {
	cut := map[graph.EdgeID]bool{}
	for _, id := range r.CutEdges {
		cut[id] = true
	}
	replayed := map[graph.EdgeID]bool{}
	for _, st := range r.Steps {
		for _, d := range st.Edges {
			replayed[d.EdgeID] = true
		}
	}
	var out []feature
	for _, e := range g.Edges {
		f := r.EdgeFlow[e.ID]
		if (f <= 0 && !replayed[e.ID]) || e.CapacityVPH <= 0 || len(e.Geometry) < 2 {
			continue
		}
		coords := make([][2]float64, len(e.Geometry))
		for i, p := range e.Geometry {
			coords[i] = [2]float64{round6(p.Lon), round6(p.Lat)}
		}
		out = append(out, feature{
			Type:     "Feature",
			Geometry: geometry{Type: "LineString", Coordinates: coords},
			Props: EdgeProps{
				ID: int(e.ID), Dir: string(r.Direction), Name: e.Name, Ref: e.Ref,
				Class: string(e.Class), Lanes: e.Lanes, LanesInferred: e.LanesInferred,
				CapacityVPH: e.CapacityVPH, FlowVPH: f,
				Utilisation: float64(f) / float64(e.CapacityVPH), IsCut: cut[e.ID],
			},
		})
	}
	return out
}

// WriteEdges writes dir/edges.geojson with features for every direction.
func WriteEdges(dir string, g *graph.Graph, results []fixes.Result) error {
	fc := featureCollection{Type: "FeatureCollection", Features: []feature{}}
	for _, r := range results {
		fc.Features = append(fc.Features, edgeFeatures(g, r)...)
	}
	b, err := json.Marshal(fc) // compact: GeoJSON is large and not read by people
	if err != nil {
		return fmt.Errorf("encode edges.geojson: %w", err)
	}
	return writeFile(filepath.Join(dir, "edges.geojson"), b)
}

func round6(x float64) float64 { return math.Round(x*1e6) / 1e6 }

// WriteFixes writes dir/fixes.json: {"inbound": Fix[], "outbound": Fix[]}.
func WriteFixes(dir string, results []fixes.Result) error {
	m := map[fixes.Direction][]fixes.Fix{}
	for _, r := range results {
		fs := r.Fixes
		if fs == nil {
			fs = []fixes.Fix{}
		}
		m[r.Direction] = fs
	}
	return writeJSON(filepath.Join(dir, "fixes.json"), m)
}

// ReplayFile is replay.json: the baseline max flow search per direction,
// path by path, for the site's "Run the analysis" replay (decision 0011).
// Mirror in web/src/types.ts.
type ReplayFile map[fixes.Direction]Replay

// Replay is one direction's search.
type Replay struct {
	MaxFlowVPH int          `json:"max_flow_vph"`
	Phases     int          `json:"phases"`
	Steps      []ReplayStep `json:"steps"`
}

// ReplayStep is one augmenting path. Edges are [edge id, delta veh/h]
// pairs, source to sink; delta is negative where the path cancelled
// earlier flow. Summing deltas per edge gives edges.geojson flow_vph.
type ReplayStep struct {
	Phase     int      `json:"phase"`
	AmountVPH int      `json:"amount_vph"`
	Edges     [][2]int `json:"edges"`
}

// Replays converts the analysis results to replay.json's shape.
func Replays(results []fixes.Result) ReplayFile {
	out := ReplayFile{}
	for _, r := range results {
		rp := Replay{MaxFlowVPH: r.MaxFlowVPH, Steps: make([]ReplayStep, 0, len(r.Steps))}
		for _, st := range r.Steps {
			es := make([][2]int, len(st.Edges))
			for i, d := range st.Edges {
				es[i] = [2]int{int(d.EdgeID), d.Delta}
			}
			rp.Steps = append(rp.Steps, ReplayStep{Phase: st.Phase, AmountVPH: st.AmountVPH, Edges: es})
			rp.Phases = max(rp.Phases, st.Phase)
		}
		out[r.Direction] = rp
	}
	return out
}

// WriteReplay writes dir/replay.json, compact like edges.geojson.
func WriteReplay(dir string, results []fixes.Result) error {
	b, err := json.Marshal(Replays(results))
	if err != nil {
		return fmt.Errorf("encode replay.json: %w", err)
	}
	return writeFile(filepath.Join(dir, "replay.json"), b)
}

// WriteManifest writes dir/manifest.json.
func WriteManifest(dir string, m Manifest) error {
	return writeJSON(filepath.Join(dir, "manifest.json"), m)
}
