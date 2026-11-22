// Package graph holds the road network data model and builds it from OSM.
package graph

// NodeID is an OpenStreetMap node id.
type NodeID int64

// EdgeID is a dense index into Graph.Edges.
type EdgeID int32

// LatLon is a WGS84 coordinate.
type LatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Node is a junction (or graph endpoint) in the road network.
type Node struct {
	ID      NodeID `json:"id"`
	Pos     LatLon `json:"pos"`
	Signals bool   `json:"signals"` // highway=traffic_signals
}

// HighwayClass is the OSM highway=* value, e.g. "trunk" or "primary_link".
type HighwayClass string

// Edge is one directed arc between two junctions.
type Edge struct {
	ID            EdgeID       `json:"id"`
	From          NodeID       `json:"from"`
	To            NodeID       `json:"to"`
	OSMWayIDs     []int64      `json:"osm_way_ids"`
	Name          string       `json:"name"`
	Ref           string       `json:"ref"`
	Class         HighwayClass `json:"class"`
	Lanes         int          `json:"lanes"` // per direction
	LanesInferred bool         `json:"lanes_inferred"`
	LengthM       float64      `json:"length_m"`
	MaxSpeedKPH   int          `json:"maxspeed_kph"` // 0 = unknown
	CapacityVPH   int          `json:"capacity_vph"` // min over Parts
	Toll          bool         `json:"toll"`
	Operator      string       `json:"operator"`
	Bridge        bool         `json:"bridge"`
	Tunnel        bool         `json:"tunnel"`
	Geometry      []LatLon     `json:"geometry"`
	// Parts are the OSM segments merged into this edge, in travel order.
	// Lanes, LanesInferred and CapacityVPH come from the part with the
	// lowest capacity (the bottleneck). Fix ranking needs the other parts:
	// after +1 lane at the bottleneck, the next-narrowest part may cap it.
	Parts []Segment `json:"parts"`
}

// Segment is one OSM way piece between two graph nodes, before merging.
type Segment struct {
	OSMWayID      int64   `json:"osm_way_id"`
	Lanes         int     `json:"lanes"` // per direction, class default if untagged
	LanesInferred bool    `json:"lanes_inferred"`
	Signalised    bool    `json:"signalised"` // ends at traffic signals; signal factor applied
	LengthM       float64 `json:"length_m"`
	CapacityVPH   int     `json:"capacity_vph"`
}

// Graph is the full directed road network for one corridor.
type Graph struct {
	Corridor string           `json:"corridor"`
	OSMDate  string           `json:"osm_date"`
	Nodes    map[NodeID]*Node `json:"nodes"`
	Edges    []Edge           `json:"edges"`
}
