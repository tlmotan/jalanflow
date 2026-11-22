// Command analyze runs max flow, min cut and fix ranking, and writes site data.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	graphPath := flag.String("graph", "", "graph.json from build-graph")
	corridor := flag.String("corridor", "", "corridor config json")
	out := flag.String("out", "", "output dir, e.g. web/public/data/federal-highway")
	flag.Parse()
	if *graphPath == "" || *corridor == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	// TODO(phase1): for each direction: snap gates → flow.New → MaxFlow →
	// MinCut → fixes.Rank → export.Write*.
	fmt.Fprintf(os.Stderr, "analyze: not implemented (graph=%s corridor=%s out=%s)\n", *graphPath, *corridor, *out)
	os.Exit(1)
}
