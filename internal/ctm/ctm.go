// Package ctm is a Cell Transmission Model (Daganzo 1994, 1995): a
// macroscopic traffic simulator that cuts roads into short cells and moves
// vehicles between them each time step, limited by what the upstream cell
// can send and what the downstream cell can receive.
//
// It adds three things to the textbook model:
//   - capacity drop: once a queue forms at a node, discharge falls to a
//     fraction of capacity (Node.CapDrop);
//   - ramp metering with ALINEA (Papageorgiou et al. 1991) on origins;
//   - fixed-time traffic signals on nodes, stepped green/red (not averaged).
//
// Like internal/flow it is pure: no I/O, no logging, no imports from other
// internal packages. Every parameter comes from the caller (config), so this
// package holds no traffic numbers of its own.
package ctm

import (
	"errors"
	"fmt"
	"math"
)

// Cell is one short stretch of road with uniform lanes and a triangular /
// trapezoidal fundamental diagram.
type Cell struct {
	LengthM           float64 // cell length, metres
	Lanes             float64 // lanes in the direction of travel
	CapPerLaneVph     float64 // capacity (saturation flow) per lane, veh/h
	FreeSpeedKph      float64 // free-flow speed, km/h
	WaveSpeedKph      float64 // backward (congestion) wave speed, km/h, > 0
	JamDensityPerLane float64 // jam density per lane, veh/km
}

// CapacityVph is the cell's total capacity in veh/h.
func (c Cell) CapacityVph() float64 { return c.Lanes * c.CapPerLaneVph }

// CriticalDensity is the density (veh/km, all lanes) at which flow peaks.
func (c Cell) CriticalDensity() float64 { return c.CapacityVph() / c.FreeSpeedKph }

// JamDensity is the total jam density in veh/km.
func (c Cell) JamDensity() float64 { return c.Lanes * c.JamDensityPerLane }

// Node joins cells. Allowed shapes: 1 in → 1 out (series), n in → 1 out
// (merge), 1 in → n out (diverge). A cell with no downstream node is an exit
// (it discharges freely); a cell with no upstream node can be fed by an Origin.
type Node struct {
	In  []int // upstream cell indices
	Out []int // downstream cell indices

	// Split is, for a diverge, the share of the In cell's traffic going to
	// each Out cell (same order, sums to 1). Ignored otherwise.
	Split []float64

	// Priority is, for a merge, each In cell's share of the downstream
	// supply when it cannot take everyone (Daganzo 1995). Nil means
	// proportional to lanes.
	Priority []float64

	// CapDrop is the fraction (0..1) by which discharge falls when any In
	// cell is queued (density above critical). 0 = no capacity drop.
	CapDrop float64

	// Signal, if set, lets an In cell send only during its green.
	Signal *Signal
}

// Origin feeds traffic into a cell from a point queue (vehicles that cannot
// enter wait at the origin, e.g. on a ramp or side road).
type Origin struct {
	Cell      int
	IntervalS float64   // length of each demand interval, seconds
	DemandVph []float64 // arrival rate per interval; zero after the last
	Meter     *Meter    // optional ramp meter
}

// Network is the whole model.
type Network struct {
	Cells   []Cell
	Nodes   []Node
	Origins []Origin
	DtS     float64 // time step, seconds
}

const eps = 1e-9

// Validate checks indices, shapes, parameters and the CFL condition
// (a vehicle at free speed may not cross more than one cell per step).
func (n *Network) Validate() error {
	if n.DtS <= 0 {
		return errors.New("ctm: DtS must be > 0")
	}
	if len(n.Cells) == 0 {
		return errors.New("ctm: no cells")
	}
	for i, c := range n.Cells {
		if c.LengthM <= 0 || c.Lanes <= 0 || c.CapPerLaneVph <= 0 || c.FreeSpeedKph <= 0 ||
			c.WaveSpeedKph <= 0 || c.JamDensityPerLane <= 0 {
			return fmt.Errorf("ctm: cell %d: all parameters must be > 0", i)
		}
		if c.FreeSpeedKph/3.6*n.DtS > c.LengthM+eps {
			return fmt.Errorf("ctm: cell %d: %.0f m is shorter than one step at free speed (%.1f m); shrink DtS",
				i, c.LengthM, c.FreeSpeedKph/3.6*n.DtS)
		}
		if c.CriticalDensity() >= c.JamDensity() {
			return fmt.Errorf("ctm: cell %d: critical density must be below jam density", i)
		}
	}
	inUse := make([]bool, len(n.Cells))  // cell already has a downstream node
	outUse := make([]bool, len(n.Cells)) // cell already has an upstream node
	for ni, nd := range n.Nodes {
		if len(nd.In) == 0 || len(nd.Out) == 0 {
			return fmt.Errorf("ctm: node %d needs at least one in and one out cell", ni)
		}
		if len(nd.In) > 1 && len(nd.Out) > 1 {
			return fmt.Errorf("ctm: node %d: n-in/m-out nodes are not supported; split into merge + diverge", ni)
		}
		for _, c := range nd.In {
			if c < 0 || c >= len(n.Cells) {
				return fmt.Errorf("ctm: node %d: in cell %d out of range", ni, c)
			}
			if inUse[c] {
				return fmt.Errorf("ctm: cell %d has two downstream nodes", c)
			}
			inUse[c] = true
		}
		for _, c := range nd.Out {
			if c < 0 || c >= len(n.Cells) {
				return fmt.Errorf("ctm: node %d: out cell %d out of range", ni, c)
			}
			if outUse[c] {
				return fmt.Errorf("ctm: cell %d has two upstream nodes", c)
			}
			outUse[c] = true
		}
		if len(nd.Out) > 1 {
			if len(nd.Split) != len(nd.Out) {
				return fmt.Errorf("ctm: node %d: diverge needs one split per out cell", ni)
			}
			sum := 0.0
			for _, s := range nd.Split {
				if s < 0 {
					return fmt.Errorf("ctm: node %d: negative split", ni)
				}
				sum += s
			}
			if math.Abs(sum-1) > 1e-6 {
				return fmt.Errorf("ctm: node %d: splits sum to %.4f, want 1", ni, sum)
			}
		}
		if nd.Priority != nil {
			if len(nd.Priority) != len(nd.In) {
				return fmt.Errorf("ctm: node %d: one priority per in cell", ni)
			}
			for _, p := range nd.Priority {
				if p <= 0 {
					return fmt.Errorf("ctm: node %d: priorities must be > 0", ni)
				}
			}
		}
		if nd.CapDrop < 0 || nd.CapDrop >= 1 {
			return fmt.Errorf("ctm: node %d: CapDrop must be in [0,1)", ni)
		}
		if nd.Signal != nil {
			if err := nd.Signal.validate(len(nd.In)); err != nil {
				return fmt.Errorf("ctm: node %d: %w", ni, err)
			}
		}
	}
	fed := make([]bool, len(n.Cells))
	for oi, o := range n.Origins {
		if o.Cell < 0 || o.Cell >= len(n.Cells) {
			return fmt.Errorf("ctm: origin %d: cell out of range", oi)
		}
		if outUse[o.Cell] {
			return fmt.Errorf("ctm: origin %d: cell %d already has an upstream node", oi, o.Cell)
		}
		if fed[o.Cell] {
			return fmt.Errorf("ctm: origin %d: cell %d already has an origin", oi, o.Cell)
		}
		fed[o.Cell] = true
		if o.IntervalS <= 0 {
			return fmt.Errorf("ctm: origin %d: IntervalS must be > 0", oi)
		}
		for _, d := range o.DemandVph {
			if d < 0 {
				return fmt.Errorf("ctm: origin %d: negative demand", oi)
			}
		}
		if o.Meter != nil {
			if err := o.Meter.validate(len(n.Cells)); err != nil {
				return fmt.Errorf("ctm: origin %d: %w", oi, err)
			}
		}
	}
	return nil
}
