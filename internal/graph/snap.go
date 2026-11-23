package graph

import (
	"fmt"
	"slices"
)

// GateEnd says which end of the flow a gate is.
type GateEnd int

const (
	// SourceEnd: traffic enters here and drives toward the far gate.
	SourceEnd GateEnd = iota
	// SinkEnd: traffic arrives here, coming from the far gate.
	SinkEnd
)

// SnapGate returns the node nearest pos that sits on a road running the
// right way (ARCHITECTURE §3.3). A gate between two carriageways must snap
// onto the one heading in the direction of travel:
//   - SourceEnd: the node starts an edge of an allowed class whose far end
//     is nearer to `far` (the other gate) than its start.
//   - SinkEnd: the node ends an edge of an allowed class whose start is
//     nearer to `far` than its end.
//
// Nodes further than maxM from pos are ignored. Ties go to the lowest id.
func SnapGate(g *Graph, pos, far LatLon, end GateEnd, classes []string, maxM float64) (NodeID, error) {
	best, bestD := NodeID(0), maxM
	found := false
	for _, e := range g.Edges {
		if !slices.Contains(classes, string(e.Class)) || e.CapacityVPH <= 0 {
			continue
		}
		from, okF := g.Nodes[e.From]
		to, okT := g.Nodes[e.To]
		if !okF || !okT {
			continue
		}
		towardFar := haversineM(to.Pos, far) < haversineM(from.Pos, far)
		var n *Node
		switch {
		case end == SourceEnd && towardFar:
			n = from
		case end == SinkEnd && !towardFar:
			n = to
		default:
			continue
		}
		d := haversineM(pos, n.Pos)
		if d < bestD || (d == bestD && (!found || n.ID < best)) {
			best, bestD, found = n.ID, d, true
		}
	}
	if !found {
		return 0, fmt.Errorf("snap gate at %.5f,%.5f: no %v road running the right way within %.0f m", pos.Lat, pos.Lon, classes, maxM)
	}
	return best, nil
}
