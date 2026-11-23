package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tlmotan/jalanflow/internal/fixes"
	"github.com/tlmotan/jalanflow/internal/graph"
)

func TestWriteEdgesAndFixes(t *testing.T) {
	g := &graph.Graph{Edges: []graph.Edge{
		{ID: 0, Class: "motorway", Lanes: 2, CapacityVPH: 4000,
			Geometry: []graph.LatLon{{Lat: 3.1234567, Lon: 101.1234567}, {Lat: 3.2, Lon: 101.2}}},
		{ID: 1, Class: "residential", Lanes: 1, CapacityVPH: 900,
			Geometry: []graph.LatLon{{Lat: 3, Lon: 101}, {Lat: 3.1, Lon: 101}}},
	}}
	results := []fixes.Result{
		{Direction: fixes.Inbound, MaxFlowVPH: 4000, EdgeFlow: []int{4000, 0}, CutEdges: []graph.EdgeID{0},
			Fixes: []fixes.Fix{{EdgeID: 0, Helps: true, GainVPH: 2000}}},
		{Direction: fixes.Outbound, EdgeFlow: []int{0, 0}},
	}
	dir := t.TempDir()
	if err := WriteEdges(dir, g, results); err != nil {
		t.Fatal(err)
	}
	var fc struct {
		Features []struct {
			Geometry struct{ Coordinates [][2]float64 }
			Props    EdgeProps `json:"properties"`
		}
	}
	readJSON(t, filepath.Join(dir, "edges.geojson"), &fc)
	if len(fc.Features) != 1 {
		t.Fatalf("got %d features, want 1 (only edges carrying flow)", len(fc.Features))
	}
	f := fc.Features[0]
	if p := f.Props; p.ID != 0 || p.Dir != "inbound" || !p.IsCut || p.Utilisation != 1 {
		t.Errorf("props = %+v", p)
	}
	if c := f.Geometry.Coordinates[0]; c != [2]float64{101.123457, 3.123457} {
		t.Errorf("first coord = %v, want lon,lat rounded to 6 dp", c)
	}

	if err := WriteFixes(dir, results); err != nil {
		t.Fatal(err)
	}
	var fx map[string][]fixes.Fix
	readJSON(t, filepath.Join(dir, "fixes.json"), &fx)
	if len(fx["inbound"]) != 1 || !fx["inbound"][0].Helps {
		t.Errorf("inbound fixes = %+v", fx["inbound"])
	}
	if fx["outbound"] == nil {
		t.Error("outbound should be [] not null, the site iterates it")
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}
