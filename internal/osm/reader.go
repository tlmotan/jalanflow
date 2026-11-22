package osm

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	posm "github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

// Node is an OSM node referenced by at least one kept way.
type Node struct {
	ID      int64
	Lat     float64
	Lon     float64
	Signals bool // highway=traffic_signals
}

// Way is an OSM way whose highway class is included.
type Way struct {
	ID    int64
	Nodes []int64 // node ids in way order
	Tags  map[string]string
}

// Data is the filtered road network read from one OSM file.
type Data struct {
	Ways  []Way          // sorted by ID
	Nodes map[int64]Node // only nodes referenced by Ways
	// Timestamp is the PBF header's replication timestamp; zero if absent.
	Timestamp time.Time
}

// Options control which ways are kept.
type Options struct {
	// Classes are the base highway classes to keep, e.g. "trunk". Their
	// "_link" variants are kept too. The caller passes the keys of the
	// capacity config so the class list has one source of truth.
	Classes []string
}

// keep reports whether a way with these tags is part of the road network.
func (o Options) keep(tags map[string]string) bool {
	hw := strings.TrimSuffix(tags["highway"], "_link")
	if hw == "" || tags["area"] == "yes" {
		return false
	}
	for _, c := range o.Classes {
		if c == hw {
			return true
		}
	}
	return false
}

// openFunc opens a fresh scanner over the input. Reading takes two passes
// (ways first, then only the nodes they use) so the whole node table of the
// clip never sits in memory.
type openFunc func(ctx context.Context, pass int) (posm.Scanner, error)

const (
	passWays = iota
	passNodes
)

// ReadPBF reads an .osm.pbf file and keeps included highway ways and their nodes.
func ReadPBF(ctx context.Context, path string, opt Options) (*Data, error) {
	var ts time.Time
	open := func(ctx context.Context, pass int) (posm.Scanner, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err // *PathError already names the op and path
		}
		s := osmpbf.New(ctx, f, runtime.GOMAXPROCS(0))
		s.SkipRelations = true
		s.SkipNodes = pass == passWays
		s.SkipWays = pass == passNodes
		if pass == passWays {
			if h, err := s.Header(); err == nil && h != nil {
				ts = h.ReplicationTimestamp
			}
		}
		return &fileScanner{Scanner: s, f: f}, nil
	}
	d, err := read(ctx, open, opt)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	d.Timestamp = ts
	return d, nil
}

// fileScanner closes the underlying file along with the scanner.
type fileScanner struct {
	*osmpbf.Scanner
	f io.Closer
}

func (s *fileScanner) Close() error {
	err := s.Scanner.Close()
	if cerr := s.f.Close(); err == nil {
		err = cerr
	}
	return err
}

func read(ctx context.Context, open openFunc, opt Options) (*Data, error) {
	if len(opt.Classes) == 0 {
		return nil, fmt.Errorf("no highway classes to keep")
	}
	d := &Data{Nodes: map[int64]Node{}}
	need := map[int64]bool{}

	err := scan(ctx, open, passWays, func(o posm.Object) {
		w, ok := o.(*posm.Way)
		if !ok || len(w.Nodes) < 2 {
			return
		}
		tags := w.Tags.Map()
		if !opt.keep(tags) {
			return
		}
		ids := make([]int64, len(w.Nodes))
		for i, n := range w.Nodes {
			ids[i] = int64(n.ID)
			need[ids[i]] = true
		}
		d.Ways = append(d.Ways, Way{ID: int64(w.ID), Nodes: ids, Tags: tags})
	})
	if err != nil {
		return nil, fmt.Errorf("ways pass: %w", err)
	}

	err = scan(ctx, open, passNodes, func(o posm.Object) {
		n, ok := o.(*posm.Node)
		if !ok || !need[int64(n.ID)] {
			return
		}
		d.Nodes[int64(n.ID)] = Node{
			ID:      int64(n.ID),
			Lat:     n.Lat,
			Lon:     n.Lon,
			Signals: n.Tags.Find("highway") == "traffic_signals",
		}
	})
	if err != nil {
		return nil, fmt.Errorf("nodes pass: %w", err)
	}

	// A missing node means the clip cut a way (osmium needs
	// --strategy complete_ways). Fail loudly rather than build a graph
	// with silently broken roads.
	if missing := len(need) - len(d.Nodes); missing > 0 {
		return nil, fmt.Errorf("%d referenced nodes missing from file (clip with --strategy complete_ways)", missing)
	}

	sort.Slice(d.Ways, func(i, j int) bool { return d.Ways[i].ID < d.Ways[j].ID })
	return d, nil
}

func scan(ctx context.Context, open openFunc, pass int, fn func(posm.Object)) (err error) {
	s, err := open(ctx, pass)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := s.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("close: %w", cerr)
		}
	}()
	for s.Scan() {
		fn(s.Object())
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	return nil
}
