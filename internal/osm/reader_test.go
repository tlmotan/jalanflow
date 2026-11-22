package osm

import (
	"context"
	"reflect"
	"strings"
	"testing"

	posm "github.com/paulmach/osm"
	"github.com/paulmach/osm/osmxml"
)

// xmlOpen serves the same OSM XML document for every pass. The reader's
// filters don't rely on the PBF skip flags, so XML exercises the same logic.
func xmlOpen(doc string) openFunc {
	return func(ctx context.Context, _ int) (posm.Scanner, error) {
		return osmxml.New(ctx, strings.NewReader(doc)), nil
	}
}

var classes = Options{Classes: []string{"motorway", "trunk", "primary", "secondary", "tertiary"}}

const fixture = `<osm version="0.6">
  <node id="1" lat="3.10" lon="101.50"/>
  <node id="2" lat="3.11" lon="101.51"><tag k="highway" v="traffic_signals"/></node>
  <node id="3" lat="3.12" lon="101.52"/>
  <node id="4" lat="3.13" lon="101.53"/>
  <node id="9" lat="3.19" lon="101.59"/>
  <way id="30"><nd ref="3"/><nd ref="4"/><tag k="highway" v="primary_link"/></way>
  <way id="10"><nd ref="1"/><nd ref="2"/><nd ref="3"/><tag k="highway" v="trunk"/><tag k="lanes" v="6"/></way>
  <way id="20"><nd ref="9"/><nd ref="1"/><tag k="highway" v="residential"/></way>
  <way id="40"><nd ref="1"/><nd ref="3"/><nd ref="1"/><tag k="highway" v="primary"/><tag k="area" v="yes"/></way>
  <way id="50"><nd ref="4"/><tag k="highway" v="trunk"/></way>
  <way id="60"><nd ref="1"/><nd ref="4"/><tag k="building" v="yes"/></way>
</osm>`

func TestReadFilters(t *testing.T) {
	d, err := read(context.Background(), xmlOpen(fixture), classes)
	if err != nil {
		t.Fatal(err)
	}

	var gotWays []int64
	for _, w := range d.Ways {
		gotWays = append(gotWays, w.ID)
	}
	// 20 residential, 40 area, 50 single node, 60 not a highway: all dropped.
	// Ways are sorted by id regardless of file order.
	if want := []int64{10, 30}; !reflect.DeepEqual(gotWays, want) {
		t.Fatalf("ways = %v, want %v", gotWays, want)
	}
	if got := d.Ways[0].Nodes; !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Errorf("way 10 nodes = %v", got)
	}
	if got := d.Ways[0].Tags["lanes"]; got != "6" {
		t.Errorf("way 10 lanes tag = %q, want 6", got)
	}

	// Node 9 is only used by the dropped residential way.
	if _, ok := d.Nodes[9]; ok {
		t.Error("node 9 kept, but only a dropped way uses it")
	}
	if len(d.Nodes) != 4 {
		t.Errorf("kept %d nodes, want 4", len(d.Nodes))
	}
	if n := d.Nodes[2]; !n.Signals || n.Lat != 3.11 || n.Lon != 101.51 {
		t.Errorf("node 2 = %+v, want signals at 3.11,101.51", n)
	}
	if d.Nodes[1].Signals {
		t.Error("node 1 should not have signals")
	}
}

func TestOptionsKeep(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want bool
	}{
		{"base class", map[string]string{"highway": "trunk"}, true},
		{"link of included class", map[string]string{"highway": "motorway_link"}, true},
		{"excluded class", map[string]string{"highway": "residential"}, false},
		{"excluded link", map[string]string{"highway": "unclassified_link"}, false},
		{"area", map[string]string{"highway": "primary", "area": "yes"}, false},
		{"no highway", map[string]string{"name": "Jalan X"}, false},
		{"bare _link", map[string]string{"highway": "_link"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classes.keep(tc.tags); got != tc.want {
				t.Fatalf("keep(%v) = %v, want %v", tc.tags, got, tc.want)
			}
		})
	}
}

func TestReadErrors(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		opt  Options
		want string
	}{
		{
			name: "missing node",
			doc: `<osm><node id="1" lat="3" lon="101"/>
				<way id="1"><nd ref="1"/><nd ref="2"/><tag k="highway" v="trunk"/></way></osm>`,
			opt:  classes,
			want: "1 referenced nodes missing",
		},
		{
			name: "no classes",
			doc:  `<osm></osm>`,
			opt:  Options{},
			want: "no highway classes",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := read(context.Background(), xmlOpen(tc.doc), tc.opt)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestReadPBFMissingFile(t *testing.T) {
	_, err := ReadPBF(context.Background(), "testdata/does-not-exist.osm.pbf", classes)
	if err == nil || !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("err = %v, want error naming the file", err)
	}
}
