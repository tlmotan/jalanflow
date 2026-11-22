// Command analyze finds lane drops (and, later, max flow, min cut and fix
// ranking) and writes site data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/export"
	"github.com/tlmotan/jalanflow/internal/graph"
	"github.com/tlmotan/jalanflow/internal/lanedrop"
)

func main() {
	graphPath := flag.String("graph", "", "graph.json from build-graph")
	corridor := flag.String("corridor", "", "corridor config json")
	out := flag.String("out", "", "output dir, e.g. web/public/data/kemuning-pj")
	flag.Parse()
	if *graphPath == "" || *corridor == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*graphPath, *corridor, *out); err != nil {
		fmt.Fprintln(os.Stderr, "analyze:", err)
		os.Exit(1)
	}
}

func run(graphPath, corridorPath, out string) error {
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
	// TODO(phase1): for each direction: snap gates → flow.New → MaxFlow →
	// MinCut → fixes.Rank → export edges/fixes/manifest.
	return nil
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
