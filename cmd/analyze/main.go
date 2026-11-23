// Command analyze finds lane drops, runs max flow / min cut per direction,
// ranks "+1 lane" fixes and writes site data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/export"
	"github.com/tlmotan/jalanflow/internal/fixes"
	"github.com/tlmotan/jalanflow/internal/graph"
	"github.com/tlmotan/jalanflow/internal/lanedrop"
)

func main() {
	graphPath := flag.String("graph", "", "graph.json from build-graph")
	corridor := flag.String("corridor", "", "corridor config json")
	out := flag.String("out", "", "output dir, e.g. web/public/data/kemuning-pj")
	capacity := flag.String("capacity", "config/capacity-defaults.json", "capacity defaults, hashed into manifest.json")
	flag.Parse()
	if *graphPath == "" || *corridor == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*graphPath, *corridor, *capacity, *out); err != nil {
		fmt.Fprintln(os.Stderr, "analyze:", err)
		os.Exit(1)
	}
}

func run(graphPath, corridorPath, capacityPath, out string) error {
	c, err := config.LoadCorridor(corridorPath)
	if err != nil {
		return err
	}
	if c.LaneDrops == nil {
		return fmt.Errorf("%s: no lane_drops block", corridorPath)
	}
	g, err := loadGraph(graphPath)
	if err != nil {
		return err
	}

	drops, err := lanedrop.Find(g, *c.LaneDrops)
	if err != nil {
		return err
	}
	f := export.LaneDropsFile{
		Corridor:    c.ID,
		OSMDate:     g.OSMDate,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Ranking:     "Biggest squeeze first: lanes out ÷ lanes in, smallest first; ties go to the drop losing more lanes.",
		Assumptions: []string{
			"Counts lanes only. It does not know how many cars use each road, so a small drop with heavy traffic can jam worse than a big one.",
			"Only highways and their ramps (" + fmt.Sprint(c.LaneDrops.Classes) + ") on or next to " + fmt.Sprint(c.LaneDrops.RouteRefs) + ".",
			"Junctions with traffic signals are skipped: movements take turns there, so more lanes in than out is normal.",
			fmt.Sprintf("Drops within %.0f m of a toll booth are skipped: plazas widen briefly and narrow back.", c.LaneDrops.TollPlazaRadiusM),
			"Lane counts come from OpenStreetMap; untagged roads use a class default and are flagged lanes_inferred.",
		},
		Drops: drops,
	}
	if err := export.WriteLaneDrops(out, f); err != nil {
		return err
	}

	inferred := 0
	for _, d := range drops {
		if d.LanesInferred {
			inferred++
		}
	}
	fmt.Fprintf(os.Stderr, "analyze: %d lane drops (%d with inferred lanes) → %s/lane_drops.json\n", len(drops), inferred, out)
	for _, d := range drops {
		if d.Confirmed != nil {
			fmt.Fprintf(os.Stderr, "  #%d %s: %d → %d lanes (%s) %.5f,%.5f\n",
				d.Rank, d.Confirmed.Name, d.LanesIn, d.LanesOut, d.Key, d.Pos.Lat, d.Pos.Lon)
		}
	}

	results, err := analyzeFlow(g, c)
	if err != nil {
		return err
	}
	if err := export.WriteEdges(out, g, results); err != nil {
		return err
	}
	if err := export.WriteFixes(out, results); err != nil {
		return err
	}
	if err := export.WriteReplay(out, results); err != nil {
		return err
	}
	capBytes, err := os.ReadFile(capacityPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", capacityPath, err)
	}
	m := export.Manifest{
		Corridor:           c.ID,
		GeneratedAt:        f.GeneratedAt,
		OSMDate:            g.OSMDate,
		PipelineVersion:    export.PipelineVersion,
		CapacityConfigHash: export.ConfigHash(capBytes),
		MaxFlowVPH:         map[string]int{},
		Assumptions: []string{
			"Junctions are treated as unlimited capacity; only road segments limit flow.",
			"Capacity = lanes x per-lane veh/h by road class x signal factor; per-lane values are placeholders until calibrated.",
			"Max flow is the most traffic the road network could carry between the gates, not how much actually travels.",
			"Min cuts are not unique; the cut nearest the source is shown.",
			"A +1 lane fix only helps if its road is in every min cut; others gain nothing on their own.",
		},
	}
	for _, r := range results {
		m.MaxFlowVPH[string(r.Direction)] = r.MaxFlowVPH
		helps := 0
		for _, fx := range r.Fixes {
			if fx.Helps {
				helps++
			}
		}
		phases := 0
		if n := len(r.Steps); n > 0 {
			phases = r.Steps[n-1].Phase
		}
		fmt.Fprintf(os.Stderr, "analyze: %s max flow %d veh/h, %d cut edges, %d fixes help on their own, replay %d paths in %d phases\n",
			r.Direction, r.MaxFlowVPH, len(r.CutEdges), helps, len(r.Steps), phases)
		for _, fx := range r.Fixes {
			e := g.Edges[fx.EdgeID]
			fmt.Fprintf(os.Stderr, "  edge %d %s %s (%d lanes, %d veh/h): helps=%v gain=%d %s\n",
				e.ID, e.Ref, e.Name, e.Lanes, e.CapacityVPH, fx.Helps, fx.GainVPH, fx.WhyNoGain)
		}
	}
	return export.WriteManifest(out, m)
}

// analyzeFlow snaps gates per direction and runs max flow + fix ranking.
// Inbound is west → east (Kota Kemuning → PJ); outbound swaps the roles.
func analyzeFlow(g *graph.Graph, c *config.Corridor) ([]fixes.Result, error) {
	var west, east []config.Gate
	for _, gt := range c.Gates {
		if gt.Role == "west" {
			west = append(west, gt)
		} else {
			east = append(east, gt)
		}
	}
	dirs := []struct {
		dir      fixes.Direction
		from, to []config.Gate
	}{
		{fixes.Inbound, west, east},
		{fixes.Outbound, east, west},
	}
	var results []fixes.Result
	for _, d := range dirs {
		sources, err := snapAll(g, c, d.from, d.to, graph.SourceEnd)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.dir, err)
		}
		sinks, err := snapAll(g, c, d.to, d.from, graph.SinkEnd)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.dir, err)
		}
		r, err := fixes.Rank(g, d.dir, sources, sinks)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.dir, err)
		}
		results = append(results, r)
	}
	return results, nil
}

// snapAll snaps each gate toward (or from) the centre of the far gates.
func snapAll(g *graph.Graph, c *config.Corridor, gates, far []config.Gate, end graph.GateEnd) ([]graph.NodeID, error) {
	var lat, lon float64
	for _, f := range far {
		lat += f.Pos.Lat / float64(len(far))
		lon += f.Pos.Lon / float64(len(far))
	}
	farPos := graph.LatLon{Lat: lat, Lon: lon}
	var ids []graph.NodeID
	for _, gt := range gates {
		id, err := graph.SnapGate(g, graph.LatLon{Lat: gt.Pos.Lat, Lon: gt.Pos.Lon}, farPos, end, c.GateSnap.Classes, c.GateSnap.MaxM)
		if err != nil {
			return nil, fmt.Errorf("gate %q: %w", gt.Name, err)
		}
		n := g.Nodes[id]
		fmt.Fprintf(os.Stderr, "analyze: gate %q (%s end) → node %d at %.5f,%.5f\n", gt.Name, map[graph.GateEnd]string{graph.SourceEnd: "source", graph.SinkEnd: "sink"}[end], id, n.Pos.Lat, n.Pos.Lon)
		ids = append(ids, id)
	}
	return ids, nil
}

func loadGraph(path string) (*graph.Graph, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var g graph.Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &g, nil
}
