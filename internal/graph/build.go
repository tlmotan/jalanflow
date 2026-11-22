package graph

import (
	"fmt"
	"math"
	"sort"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/osm"
)

// Build turns filtered OSM ways into a simplified directed Graph.
//
//  1. Split each way at junctions: nodes used twice or more (shared by ways,
//     or revisited by one way), way endpoints, and traffic-signal nodes
//     (so the signal factor only hits the segment ending at the signal).
//  2. One Edge per allowed direction (osm.Oneway) per segment.
//  3. Lanes via osm.LanesPerDirection; class default if untagged (flagged).
//  4. Capacity via Capacity(); signal factor if the edge ends at a signal.
//  5. Simplify: merge pass-through chains, keeping the MIN capacity.
//
// Segments that start and end at the same node (closed ways with no other
// junction) are dropped: they cannot carry flow between two places.
func Build(d *osm.Data, cfg *config.Capacity, corridor string) (*Graph, error) {
	if d == nil || cfg == nil {
		return nil, fmt.Errorf("build: nil osm data or capacity config")
	}
	split := splitNodes(d)

	var edges []Edge
	for _, w := range d.Ways {
		class := HighwayClass(w.Tags["highway"])
		if _, ok := cfg.Classes[baseClass(class)]; !ok {
			return nil, fmt.Errorf("build: way %d: highway class %q not in capacity config", w.ID, class)
		}
		fwd, bwd := osm.Oneway(w.Tags)
		start := 0
		for i := 1; i < len(w.Nodes); i++ {
			if !split[w.Nodes[i]] && i != len(w.Nodes)-1 {
				continue
			}
			ids := w.Nodes[start : i+1]
			start = i
			if ids[0] == ids[len(ids)-1] {
				continue
			}
			geom, err := positions(d, ids)
			if err != nil {
				return nil, fmt.Errorf("build: way %d: %w", w.ID, err)
			}
			if fwd {
				edges = append(edges, newArc(d, cfg, w, class, ids, geom, osm.Forward))
			}
			if bwd {
				edges = append(edges, newArc(d, cfg, w, class, reversed(ids), reversed(geom), osm.Backward))
			}
		}
	}

	edges = simplify(edges)

	g := &Graph{Corridor: corridor, Nodes: map[NodeID]*Node{}, Edges: edges}
	if !d.Timestamp.IsZero() {
		g.OSMDate = d.Timestamp.UTC().Format("2006-01-02")
	}
	g.TollBooths = tollBooths(d)
	for i := range g.Edges {
		e := &g.Edges[i]
		e.ID = EdgeID(i)
		for _, id := range []NodeID{e.From, e.To} {
			if _, ok := g.Nodes[id]; !ok {
				n := d.Nodes[int64(id)]
				g.Nodes[id] = &Node{ID: id, Pos: LatLon{Lat: n.Lat, Lon: n.Lon}, Signals: n.Signals}
			}
		}
	}
	return g, nil
}

// tollBooths returns toll-booth positions in a stable order.
func tollBooths(d *osm.Data) []LatLon {
	res := []LatLon{}
	for _, n := range d.Nodes {
		if n.TollBooth {
			res = append(res, LatLon{Lat: n.Lat, Lon: n.Lon})
		}
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Lat != res[j].Lat {
			return res[i].Lat < res[j].Lat
		}
		return res[i].Lon < res[j].Lon
	})
	return res
}

// HaversineM returns the great-circle distance between two points in metres.
func HaversineM(a, b LatLon) float64 { return haversineM(a, b) }

// splitNodes returns the nodes where ways must be cut into segments.
func splitNodes(d *osm.Data) map[int64]bool {
	uses := map[int64]int{}
	split := map[int64]bool{}
	for _, w := range d.Ways {
		split[w.Nodes[0]] = true
		split[w.Nodes[len(w.Nodes)-1]] = true
		for _, id := range w.Nodes {
			uses[id]++
		}
	}
	for id, n := range uses {
		if n >= 2 || d.Nodes[id].Signals {
			split[id] = true
		}
	}
	return split
}

func positions(d *osm.Data, ids []int64) ([]LatLon, error) {
	geom := make([]LatLon, len(ids))
	for i, id := range ids {
		n, ok := d.Nodes[id]
		if !ok {
			return nil, fmt.Errorf("node %d missing", id)
		}
		geom[i] = LatLon{Lat: n.Lat, Lon: n.Lon}
	}
	return geom, nil
}

// newArc builds one directed, single-segment edge. ids and geom are already
// in travel order.
func newArc(d *osm.Data, cfg *config.Capacity, w osm.Way, class HighwayClass, ids []int64, geom []LatLon, dir osm.Direction) Edge {
	lanes, inferred := osm.LanesPerDirection(w.Tags, dir)
	e := Edge{
		From:          NodeID(ids[0]),
		To:            NodeID(ids[len(ids)-1]),
		OSMWayIDs:     []int64{w.ID},
		Name:          w.Tags["name"],
		Ref:           w.Tags["ref"],
		Class:         class,
		Lanes:         lanes,
		LanesInferred: inferred,
		LengthM:       pathLength(geom),
		MaxSpeedKPH:   osm.MaxSpeedKPH(w.Tags),
		Toll:          osm.Flag(w.Tags, "toll"),
		Operator:      w.Tags["operator"],
		Bridge:        osm.Flag(w.Tags, "bridge"),
		Tunnel:        osm.Flag(w.Tags, "tunnel"),
		Geometry:      geom,
	}
	signal := d.Nodes[int64(e.To)].Signals
	e.CapacityVPH, e.Lanes, e.LanesInferred = Capacity(e, signal, cfg)
	e.Parts = []Segment{{
		OSMWayID:      w.ID,
		Lanes:         e.Lanes,
		LanesInferred: e.LanesInferred,
		Signalised:    signal,
		LengthM:       e.LengthM,
		CapacityVPH:   e.CapacityVPH,
		End:           geom[len(geom)-1],
	}}
	return e
}

func reversed[T any](s []T) []T {
	r := make([]T, len(s))
	for i, v := range s {
		r[len(s)-1-i] = v
	}
	return r
}

// earthRadiusM is the WGS84 mean Earth radius (IUGG), a physical constant.
const earthRadiusM = 6371008.8

// haversineM returns the great-circle distance between two points in metres.
func haversineM(a, b LatLon) float64 {
	rad := math.Pi / 180
	dLat := (b.Lat - a.Lat) * rad
	dLon := (b.Lon - a.Lon) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}

func pathLength(geom []LatLon) float64 {
	var m float64
	for i := 1; i < len(geom); i++ {
		m += haversineM(geom[i-1], geom[i])
	}
	return m
}
