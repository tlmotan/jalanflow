package graph

import "github.com/tlmotan/jalanflow/internal/config"

// BaseClass maps "primary_link" -> "primary": the key for per-class
// settings in config/capacity-defaults.json.
func BaseClass(c HighwayClass) string { return baseClass(c) }

// baseClass maps "primary_link" -> "primary".
func baseClass(c HighwayClass) string {
	s := string(c)
	if n := len(s); n > 5 && s[n-5:] == "_link" {
		return s[:n-5]
	}
	return s
}

// Capacity returns veh/h for an edge. endsAtSignal applies the signal factor.
// Lanes == 0 means untagged: the class default is used.
func Capacity(e Edge, endsAtSignal bool, cfg *config.Capacity) (capVPH, lanes int, inferred bool) {
	d := cfg.Classes[baseClass(e.Class)]
	lanes, inferred = e.Lanes, e.LanesInferred
	if lanes == 0 {
		lanes, inferred = d.DefaultLanes, true
	}
	c := float64(lanes * d.PerLaneVPH)
	if endsAtSignal {
		c *= cfg.SignalFactor
	}
	return int(c), lanes, inferred
}
