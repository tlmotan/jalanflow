// Package export writes edges.geojson, fixes.json and manifest.json.
package export

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
