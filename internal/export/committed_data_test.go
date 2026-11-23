package export

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tlmotan/jalanflow/internal/fixes"
)

// The committed web/public/data/ is what the live site serves, and CI cannot
// regenerate it (raw OSM data is not committed). These tests check it still
// matches the Go output structs and the current capacity config, so a struct
// or config change without `make graph analyze` fails CI.

const (
	repoRoot                             = "../.."
	capacityConfig                       = repoRoot + "/config/capacity-defaults.json"
	regenerateHint                       = "re-run `make graph analyze` and commit web/public/data/"
	klangValleyMinLat, klangValleyMaxLat = 2.6, 3.6
	klangValleyMinLon, klangValleyMaxLon = 101.2, 102.0
)

// decodeStrict decodes path into v, rejecting fields v does not know.
func decodeStrict(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("%s does not match the Go output struct (%v); %s", path, err, regenerateHint)
	}
}

func corridorDirs(t *testing.T) []string {
	t.Helper()
	manifests, err := filepath.Glob(repoRoot + "/web/public/data/*/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Skip("no committed corridor data under web/public/data/")
	}
	dirs := make([]string, len(manifests))
	for i, m := range manifests {
		dirs[i] = filepath.Dir(m)
	}
	return dirs
}

func TestCommittedData(t *testing.T) {
	capBytes, err := os.ReadFile(capacityConfig)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := ConfigHash(capBytes)

	for _, dir := range corridorDirs(t) {
		corridor := filepath.Base(dir)
		t.Run(corridor, func(t *testing.T) {
			t.Run("manifest", func(t *testing.T) {
				var m Manifest
				decodeStrict(t, filepath.Join(dir, "manifest.json"), &m)
				if m.Corridor != corridor {
					t.Errorf("corridor = %q, want folder name %q", m.Corridor, corridor)
				}
				if m.CapacityConfigHash != wantHash {
					t.Errorf("capacity_config_hash = %s, but %s hashes to %s: config changed since the data was built; %s",
						m.CapacityConfigHash, capacityConfig, wantHash, regenerateHint)
				}
				for _, d := range []fixes.Direction{fixes.Inbound, fixes.Outbound} {
					if m.MaxFlowVPH[string(d)] <= 0 {
						t.Errorf("max_flow_vph[%s] = %d, want > 0", d, m.MaxFlowVPH[string(d)])
					}
				}
			})

			t.Run("edges", func(t *testing.T) {
				var fc featureCollection
				decodeStrict(t, filepath.Join(dir, "edges.geojson"), &fc)
				if fc.Type != "FeatureCollection" || len(fc.Features) == 0 {
					t.Fatalf("type = %q with %d features, want a non-empty FeatureCollection", fc.Type, len(fc.Features))
				}
				for _, f := range fc.Features {
					p := f.Props
					if f.Geometry.Type != "LineString" || len(f.Geometry.Coordinates) < 2 {
						t.Errorf("edge %d: geometry %s with %d points, want LineString with >= 2", p.ID, f.Geometry.Type, len(f.Geometry.Coordinates))
					}
					for _, c := range f.Geometry.Coordinates {
						lon, lat := c[0], c[1]
						if lat < klangValleyMinLat || lat > klangValleyMaxLat || lon < klangValleyMinLon || lon > klangValleyMaxLon {
							t.Errorf("edge %d: point [%v, %v] is outside the Klang Valley (lon/lat swapped?)", p.ID, lon, lat)
							break
						}
					}
					if p.CapacityVPH <= 0 {
						t.Errorf("edge %d: capacity_vph = %d, want > 0", p.ID, p.CapacityVPH)
					}
					if p.FlowVPH > p.CapacityVPH {
						t.Errorf("edge %d: flow_vph %d > capacity_vph %d", p.ID, p.FlowVPH, p.CapacityVPH)
					}
				}
			})

			t.Run("fixes", func(t *testing.T) {
				var fx map[fixes.Direction][]fixes.Fix
				decodeStrict(t, filepath.Join(dir, "fixes.json"), &fx)
				for d := range fx {
					if d != fixes.Inbound && d != fixes.Outbound {
						t.Errorf("unknown direction %q", d)
					}
				}
			})

			t.Run("lane_drops", func(t *testing.T) {
				path := filepath.Join(dir, "lane_drops.json")
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Skip("no lane_drops.json")
				}
				var ld LaneDropsFile
				decodeStrict(t, path, &ld)
				if ld.Corridor != corridor {
					t.Errorf("corridor = %q, want folder name %q", ld.Corridor, corridor)
				}
				for i, d := range ld.Drops {
					if d.Rank != i+1 {
						t.Errorf("drops[%d].rank = %d, want %d", i, d.Rank, i+1)
						break
					}
				}
			})
		})
	}
}
