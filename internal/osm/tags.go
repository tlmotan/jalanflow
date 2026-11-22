// Package osm reads OSM PBF files and parses road tags.
package osm

import (
	"strconv"
	"strings"
)

// Direction of travel along a way relative to its node order.
type Direction int

const (
	Forward Direction = iota
	Backward
)

// Oneway reports which directions a way allows, from its tags.
func Oneway(tags map[string]string) (forward, backward bool) {
	switch tags["oneway"] {
	case "yes", "true", "1":
		return true, false
	case "-1", "reverse":
		return false, true
	}
	// Motorways and roundabouts are one-way by default in OSM.
	if tags["highway"] == "motorway" || tags["junction"] == "roundabout" {
		return true, false
	}
	return true, true
}

// LanesPerDirection returns lanes for one direction and whether it was inferred.
// Order: lanes:forward/backward → lanes (if one-way) → lanes/2 → 0 (caller uses class default).
func LanesPerDirection(tags map[string]string, dir Direction) (lanes int, inferred bool) {
	key := "lanes:forward"
	if dir == Backward {
		key = "lanes:backward"
	}
	if n, ok := atoi(tags[key]); ok {
		return n, false
	}
	fwd, bwd := Oneway(tags)
	if n, ok := atoi(tags["lanes"]); ok {
		if fwd != bwd { // one-way
			return n, false
		}
		return max(1, n/2), false
	}
	return 0, true
}

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n > 0
}

// MaxSpeedKPH parses maxspeed=*. Plain numbers and "N km/h" are km/h;
// anything else (mph, "none", zone codes like "MY:urban") returns 0 = unknown.
func MaxSpeedKPH(tags map[string]string) int {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(tags["maxspeed"]), "km/h"))
	if n, ok := atoi(s); ok {
		return n
	}
	return 0
}

// Flag reports whether a yes/no style tag is set: any value except "" or "no".
// bridge=viaduct and tunnel=building_passage count as set.
func Flag(tags map[string]string, key string) bool {
	v := tags[key]
	return v != "" && v != "no"
}
