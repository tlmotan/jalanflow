// Command build-graph turns a clipped OSM PBF into data/interim/<corridor>.graph.json.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/graph"
	"github.com/tlmotan/jalanflow/internal/osm"
)

func main() {
	in := flag.String("in", "", "clipped .osm.pbf")
	out := flag.String("out", "", "output graph.json")
	capCfg := flag.String("capacity", "config/capacity-defaults.json", "capacity config")
	corridor := flag.String("corridor", "", "corridor id (default: input file name without .osm.pbf)")
	flag.Parse()
	if *in == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *corridor == "" {
		*corridor = strings.TrimSuffix(filepath.Base(*in), ".osm.pbf")
	}
	if err := run(*in, *out, *capCfg, *corridor); err != nil {
		fmt.Fprintln(os.Stderr, "build-graph:", err)
		os.Exit(1)
	}
}

func run(in, out, capPath, corridor string) error {
	cfg, err := config.LoadCapacity(capPath)
	if err != nil {
		return err
	}
	// The capacity config's classes are the single list of included roads.
	classes := make([]string, 0, len(cfg.Classes))
	for c := range cfg.Classes {
		classes = append(classes, c)
	}
	sort.Strings(classes)

	d, err := osm.ReadPBF(context.Background(), in, osm.Options{Classes: classes})
	if err != nil {
		return err
	}
	g, err := graph.Build(d, cfg, corridor)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	if err := json.NewEncoder(f).Encode(g); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", out, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", out, err)
	}

	inferred := 0
	for _, e := range g.Edges {
		if e.LanesInferred {
			inferred++
		}
	}
	fmt.Fprintf(os.Stderr, "build-graph: %d ways → %d nodes, %d edges (%d with inferred lanes) → %s\n",
		len(d.Ways), len(g.Nodes), len(g.Edges), inferred, out)
	return nil
}
