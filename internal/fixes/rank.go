// Package fixes ranks "+1 lane" changes on min-cut edges.
package fixes

import "github.com/tlmotan/jalanflow/internal/graph"

// Direction of analysis.
type Direction string

const (
	Inbound  Direction = "inbound"
	Outbound Direction = "outbound"
)

// Difficulty is a rough feasibility label.
type Difficulty string

const (
	Easy   Difficulty = "easy"
	Medium Difficulty = "medium"
	Hard   Difficulty = "hard"
)

// Fix is one tested change and its effect.
type Fix struct {
	EdgeID        graph.EdgeID   `json:"edge_id"`
	Direction     Direction      `json:"direction"`
	Change        string         `json:"change"`
	GainVPH       int            `json:"gain_vph"`
	NewMaxFlowVPH int            `json:"new_max_flow_vph"`
	NextCutEdges  []graph.EdgeID `json:"next_cut_edges"`
	Difficulty    Difficulty     `json:"difficulty"`
	Reasons       []string       `json:"reasons"`
}

// Rate labels an edge using only OSM tags (ARCHITECTURE.md §3.5).
func Rate(e graph.Edge) (Difficulty, []string) {
	var reasons []string
	d := Easy
	if e.Class == "motorway" || e.Class == "trunk" {
		d = Medium
		reasons = append(reasons, "major highway: widening needs road reserve")
	}
	if e.Toll || e.Operator != "" {
		d = Medium
		reasons = append(reasons, "tolled or concession road: needs concessionaire agreement")
	}
	if e.Bridge || e.Tunnel {
		d = Hard
		reasons = append(reasons, "bridge or tunnel: structural work")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "local road: re-striping or junction change may suffice")
	}
	return d, reasons
}

// TODO(phase1): Rank(g, cfg, dir) []Fix — for each cut edge, +1 lane,
// rebuild the flow network, re-run MaxFlow, record gain and next cut.
