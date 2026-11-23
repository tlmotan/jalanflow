// Package corridor turns the route through the road graph (KESAS then LDP,
// with their ramps) into a Cell Transmission Model network (internal/ctm).
//
// It is a fixed-route model (decision 0007): traffic enters at the route
// start and at on-ramps, and leaves at off-ramps by measured split ratios.
// Nobody re-routes. Side roads that join or leave the route are modelled
// only as the ramp edge itself: an on-ramp's own approach (and any traffic
// light on it) is outside the model for now.
//
// Package rule: corridor ← graph, config, ctm.
package corridor

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/ctm"
	"github.com/tlmotan/jalanflow/internal/graph"
)

// Options say which roads make up the route and how fine the model is.
type Options struct {
	// RouteRefs are the road refs of the route, e.g. ["E5", "E11"].
	RouteRefs []string
	// MainClasses are mainline classes; a mainline edge must carry one of
	// RouteRefs or have no ref at all (an OSM tagging gap); one with another
	// road's ref is never used. RampClasses (e.g. motorway_link) need no ref,
	// so the route can take the interchange ramp from one highway to the next.
	MainClasses []string
	RampClasses []string
	// DtS is the simulation time step. Cells are cut to at least the
	// distance the fastest class covers in one step (CFL condition).
	DtS float64
}

// Kind says what part of the model a cell or ramp is.
type Kind string

// Kinds of cells and ramps.
const (
	Mainline Kind = "mainline"
	OnRamp   Kind = "on_ramp"
	OffRamp  Kind = "off_ramp"
)

// CellInfo ties a ctm cell back to the road it stands for.
type CellInfo struct {
	Kind          Kind
	EdgeID        graph.EdgeID // edge at the cell's midpoint
	OSMWayID      int64        // OSM way at the cell's midpoint
	StartM        float64      // from the route start (mainline) or ramp start
	LengthM       float64
	Lanes         int
	LanesInferred bool
	Ramp          int // index into Model.Ramps; -1 on the mainline
}

// Ramp is one road joining or leaving the route.
type Ramp struct {
	Kind      Kind
	Name      string
	EdgeID    graph.EdgeID
	OSMWayIDs []int64
	AtNode    graph.NodeID // where it meets the route
	AtM       float64      // distance along the route of that node
	Cells     []int
	Class     graph.HighwayClass
	// Node is the index in Model.Net.Nodes of the merge or diverge where
	// the ramp meets the route.
	Node  int
	Lanes int // narrowest point of the ramp
	// MainLanes is the route's lane count just past the point where the
	// ramp meets it (the lanes that carry on, for an off-ramp).
	MainLanes int
	// DemandKey is the config key that supplied its demand (on-ramp) or
	// split ratio (off-ramp); "" when none matched and a zero was used.
	DemandKey string
	PeakVPH   float64 // on-ramps
	Split     float64 // off-ramps
}

// Model is the CTM network for one direction plus the map back to roads.
type Model struct {
	Net      ctm.Network
	Route    []graph.EdgeID
	Mainline []int // mainline cell indices in travel order
	Cells    []CellInfo
	Ramps    []Ramp
	// MainKey is the demand key used for traffic entering at the route
	// start; "" if none matched.
	MainKey string
	// ProfileStart is the clock time ("HH:MM") of demand interval 0.
	ProfileStart string
	// cellClass is each cell's road class (for per-class settings).
	cellClass []graph.HighwayClass
	// LaneDropNodes are the indices in Net.Nodes of mainline nodes where
	// the route itself loses lanes (no ramp involved).
	LaneDropNodes []int
	// Estimated counts demand figures and split ratios that were fitted
	// (source "estimated - ..."), not counted on the road.
	Estimated int
	// Gaps lists data the model had to assume (zero demand, zero split)
	// or config it could not place. Shown to the user; calibration should
	// not be trusted while it is long.
	Gaps []string
}

func refs(e *graph.Edge) []string {
	var out []string
	for _, r := range strings.Split(e.Ref, ";") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

func (o Options) allowed(e *graph.Edge) bool {
	c := string(e.Class)
	if slices.Contains(o.RampClasses, c) {
		return true
	}
	if !slices.Contains(o.MainClasses, c) {
		return false
	}
	rs := refs(e)
	// A mainline stretch with no ref tag at all is an OSM tagging gap, not
	// another road (found on the real graph: 72 m of motorway between KESAS
	// and LDP at 3.05917,101.60778). It may be used; Build lists it in Gaps.
	if len(rs) == 0 {
		return true
	}
	for _, r := range rs {
		if slices.Contains(o.RouteRefs, r) {
			return true
		}
	}
	return false
}

func freeSpeed(cap *config.Capacity, c graph.HighwayClass) (float64, error) {
	p, ok := cap.CTM[graph.BaseClass(c)]
	if !ok {
		return 0, fmt.Errorf("no ctm parameters for class %q", c)
	}
	return p.FreeFlowKMH, nil
}

// Route returns the fastest path at free speed from src to dst using only
// route mainline edges and ramps (Options.allowed).
func Route(g *graph.Graph, cap *config.Capacity, src, dst graph.NodeID, opt Options) ([]graph.EdgeID, error) {
	out := map[graph.NodeID][]int{}
	for i := range g.Edges {
		e := &g.Edges[i]
		if opt.allowed(e) {
			out[e.From] = append(out[e.From], i)
		}
	}
	dist := map[graph.NodeID]float64{src: 0}
	prev := map[graph.NodeID]int{}
	pq := &nodeHeap{{src, 0}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(nodeDist)
		if cur.d > dist[cur.n] {
			continue
		}
		if cur.n == dst {
			break
		}
		for _, i := range out[cur.n] {
			e := &g.Edges[i]
			v, err := freeSpeed(cap, e.Class)
			if err != nil {
				return nil, err
			}
			d := cur.d + e.LengthM/v
			if old, ok := dist[e.To]; !ok || d < old {
				dist[e.To] = d
				prev[e.To] = i
				heap.Push(pq, nodeDist{e.To, d})
			}
		}
	}
	if _, ok := dist[dst]; !ok {
		return nil, fmt.Errorf("corridor: no route from node %d to node %d on %v + ramps", src, dst, opt.RouteRefs)
	}
	var path []graph.EdgeID
	for n := dst; n != src; {
		i := prev[n]
		path = append(path, g.Edges[i].ID)
		n = g.Edges[i].From
	}
	slices.Reverse(path)
	return path, nil
}

type nodeDist struct {
	n graph.NodeID
	d float64
}
type nodeHeap []nodeDist

func (h nodeHeap) Len() int { return len(h) }
func (h nodeHeap) Less(i, j int) bool {
	return h[i].d < h[j].d || (h[i].d == h[j].d && h[i].n < h[j].n)
}
func (h nodeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *nodeHeap) Push(x any)   { *h = append(*h, x.(nodeDist)) }
func (h *nodeHeap) Pop() any     { old := *h; x := old[len(old)-1]; *h = old[:len(old)-1]; return x }

// piece is one OSM part of a route edge, laid out along the route.
type piece struct {
	startM, endM float64
	lanes        int
	inferred     bool
	class        graph.HighwayClass
	edge         graph.EdgeID
	way          int64
}

func pieces(g *graph.Graph, route []graph.EdgeID, startM float64) []piece {
	var ps []piece
	m := startM
	for _, id := range route {
		e := &g.Edges[id]
		parts := e.Parts
		if len(parts) == 0 {
			parts = []graph.Segment{{OSMWayID: firstWay(e), Lanes: e.Lanes, LanesInferred: e.LanesInferred, LengthM: e.LengthM}}
		}
		total := 0.0
		for _, p := range parts {
			total += p.LengthM
		}
		for _, p := range parts {
			l := p.LengthM
			if total > 0 {
				l = p.LengthM / total * e.LengthM // parts and edge length agree; rescale away rounding
			}
			ps = append(ps, piece{m, m + l, p.Lanes, p.LanesInferred, e.Class, e.ID, p.OSMWayID})
			m += l
		}
	}
	return ps
}

// pieceAt returns the piece containing distance x (the last one past the end).
func pieceAt(ps []piece, x float64) *piece {
	for j := range ps {
		if x < ps[j].endM {
			return &ps[j]
		}
	}
	return &ps[len(ps)-1]
}

func firstWay(e *graph.Edge) int64 {
	if len(e.OSMWayIDs) > 0 {
		return e.OSMWayIDs[0]
	}
	return 0
}

// cutCells splits [0, total] at the required breakpoints (merges and
// diverges, kept at least minM apart by pushing later ones forward) and,
// where room allows, at lane changes, then into cells of at least minM.
func cutCells(total float64, required, optional []float64, minM float64) (bounds []float64, moved map[float64]float64) {
	moved = map[float64]float64{}
	sort.Float64s(required)
	var req []float64
	last := 0.0
	for _, r := range required {
		p := math.Max(r, last+minM)
		if p > total-minM {
			p = math.Max(last+minM, total-minM)
		}
		if len(req) > 0 && p <= req[len(req)-1] {
			p = req[len(req)-1] + minM
		}
		moved[r] = p
		req = append(req, p)
		last = p
	}
	all := append([]float64{0}, req...)
	for _, o := range optional {
		ok := o > minM && o < total-minM
		for _, b := range all {
			if math.Abs(o-b) < minM {
				ok = false
			}
		}
		if ok {
			all = append(all, o)
		}
	}
	all = append(all, math.Max(total, last+minM))
	sort.Float64s(all)
	for i := 0; i+1 < len(all); i++ {
		a, b := all[i], all[i+1]
		n := int(math.Max(1, math.Floor((b-a)/minM)))
		for k := 0; k < n; k++ {
			bounds = append(bounds, a+(b-a)*float64(k)/float64(n))
		}
	}
	return append(bounds, all[len(all)-1]), moved
}

// minCellM is the shortest cell any class on the model may have.
func minCellM(cap *config.Capacity, classes []graph.HighwayClass, dtS float64) (float64, error) {
	vmax := 0.0
	for _, c := range classes {
		v, err := freeSpeed(cap, c)
		if err != nil {
			return 0, err
		}
		vmax = math.Max(vmax, v)
	}
	return vmax / 3.6 * dtS, nil
}

func ctmCell(cap *config.Capacity, class graph.HighwayClass, lanes int, lengthM float64) (ctm.Cell, error) {
	b := graph.BaseClass(class)
	cl, ok := cap.Classes[b]
	p, ok2 := cap.CTM[b]
	if !ok || !ok2 {
		return ctm.Cell{}, fmt.Errorf("no capacity or ctm parameters for class %q", class)
	}
	qpl := float64(cl.PerLaneVPH)
	// Triangular diagram: the congested branch runs from (kc, q) to (kj, 0).
	kc := qpl / p.FreeFlowKMH
	if kc >= p.JamDensity {
		return ctm.Cell{}, fmt.Errorf("class %q: critical density %.1f ≥ jam density", class, kc)
	}
	return ctm.Cell{
		LengthM:           lengthM,
		Lanes:             float64(lanes),
		CapPerLaneVph:     qpl,
		FreeSpeedKph:      p.FreeFlowKMH,
		WaveSpeedKph:      qpl / (p.JamDensity - kc),
		JamDensityPerLane: p.JamDensity,
	}, nil
}

func capDrop(cap *config.Capacity, class graph.HighwayClass) float64 {
	return cap.CTM[graph.BaseClass(class)].CapacityDrop
}

func rampName(g *graph.Graph, e *graph.Edge, kind Kind) string {
	n := e.Name
	if n == "" && e.Ref != "" {
		n = e.Ref
	}
	if n == "" {
		n = fmt.Sprintf("unnamed %s (way %d)", e.Class, firstWay(e))
	}
	if kind == OnRamp {
		return "on-ramp from " + n
	}
	return "off-ramp to " + n
}

// demandLookup matches config keys to graph elements.
type demandLookup struct {
	d    *config.Demand
	used map[string]bool
}

func (l *demandLookup) entry(nodeID graph.NodeID, e *graph.Edge) (*config.DemandEntry, error) {
	if l.d == nil {
		return nil, nil
	}
	for i := range l.d.Entries {
		en := &l.d.Entries[i]
		if ok, err := keyMatches(en.Key, nodeID, e); err != nil {
			return nil, err
		} else if ok {
			l.used[en.Key] = true
			return en, nil
		}
	}
	return nil, nil
}

func (l *demandLookup) exit(e *graph.Edge) (*config.DemandExit, error) {
	if l.d == nil {
		return nil, nil
	}
	for i := range l.d.Exits {
		ex := &l.d.Exits[i]
		if ok, err := keyMatches(ex.Key, -1, e); err != nil {
			return nil, err
		} else if ok {
			l.used[ex.Key] = true
			return ex, nil
		}
	}
	return nil, nil
}

// keyMatches: "way:<id>" matches an edge built from that way; "node:<id>"
// matches the given node (the route start, or the node a ramp starts at).
func keyMatches(key string, node graph.NodeID, e *graph.Edge) (bool, error) {
	kind, id, err := config.ParseOSMKey(key)
	if err != nil {
		return false, err
	}
	switch kind {
	case config.OSMWay:
		return e != nil && slices.Contains(e.OSMWayIDs, id), nil
	default:
		return graph.NodeID(id) == node, nil
	}
}

func (l *demandLookup) demand(en *config.DemandEntry) ([]float64, string, error) {
	p, ok := l.d.Profiles[en.Profile]
	if !ok {
		return nil, "", fmt.Errorf("demand entry %s: unknown profile %q", en.Key, en.Profile)
	}
	out := make([]float64, len(p.Shape))
	for i, s := range p.Shape {
		out[i] = s * en.PeakVPH
	}
	return out, p.Start, nil
}

// Build lays route out as mainline cells, attaches every other edge that
// joins (on-ramp) or leaves (off-ramp) the route at an inner node, and
// fills demand and split ratios from the demand config (nil = none yet).
func Build(g *graph.Graph, cap *config.Capacity, demand *config.Demand, route []graph.EdgeID, opt Options) (*Model, error) {
	if len(route) == 0 {
		return nil, errors.New("corridor: empty route")
	}
	if opt.DtS <= 0 {
		return nil, errors.New("corridor: DtS must be > 0")
	}
	m := &Model{Route: route, Net: ctm.Network{DtS: opt.DtS}}
	lk := &demandLookup{d: demand, used: map[string]bool{}}

	// Where each route node sits along the route, and the side edges there.
	type event struct {
		node     graph.NodeID
		atM      float64
		ins      []*graph.Edge
		outs     []*graph.Edge
		in, next graph.EdgeID
	}
	onRoute := map[graph.EdgeID]bool{}
	for _, id := range route {
		onRoute[id] = true
	}
	ins := map[graph.NodeID][]*graph.Edge{}
	outs := map[graph.NodeID][]*graph.Edge{}
	for i := range g.Edges {
		e := &g.Edges[i]
		if onRoute[e.ID] || e.CapacityVPH <= 0 {
			continue
		}
		ins[e.To] = append(ins[e.To], e)
		outs[e.From] = append(outs[e.From], e)
	}
	classes := []graph.HighwayClass{}
	var events []event
	at := 0.0
	for i, id := range route {
		e := &g.Edges[id]
		classes = append(classes, e.Class)
		at += e.LengthM
		if i+1 < len(route) {
			v := e.To
			ev := event{node: v, atM: at, ins: ins[v], outs: outs[v], in: id, next: route[i+1]}
			// A two-way side road shows up as both an in and an out edge
			// at v; both are kept (traffic can join and leave there).
			if len(ev.ins)+len(ev.outs) > 0 {
				events = append(events, ev)
			}
			for _, s := range ev.ins {
				classes = append(classes, s.Class)
			}
			for _, s := range ev.outs {
				classes = append(classes, s.Class)
			}
		}
	}
	total := at
	minM, err := minCellM(cap, classes, opt.DtS)
	if err != nil {
		return nil, fmt.Errorf("corridor: %w", err)
	}

	// Required breakpoints: a diverge at the node; a merge at the node, or
	// one cell later when the same node also has a diverge (ctm nodes are
	// merge or diverge, never both).
	type split struct {
		atM   float64
		ev    int
		merge bool
	}
	var splits []split
	var required []float64
	for i, ev := range events {
		if len(ev.outs) > 0 {
			splits = append(splits, split{ev.atM, i, false})
			required = append(required, ev.atM)
		}
		if len(ev.ins) > 0 {
			p := ev.atM
			if len(ev.outs) > 0 {
				p += minM
			}
			splits = append(splits, split{p, i, true})
			required = append(required, p)
		}
	}
	ps := pieces(g, route, 0)
	var optional []float64
	for i := 1; i < len(ps); i++ {
		if ps[i].lanes != ps[i-1].lanes {
			optional = append(optional, ps[i].startM)
		}
	}
	bounds, moved := cutCells(total, required, optional, minM)

	// Mainline cells: each takes the narrowest piece it overlaps (merging
	// keeps the minimum capacity, AGENTS.md), and the edge at its midpoint.
	for i := 0; i+1 < len(bounds); i++ {
		a, b := bounds[i], bounds[i+1]
		mid := pieceAt(ps, (a+b)/2)
		narrow := mid
		for j := range ps {
			p := &ps[j]
			// Pieces overlapping by under 1 m are rounding at a boundary.
			if math.Min(b, p.endM)-math.Max(a, p.startM) > 1 && p.lanes < narrow.lanes {
				narrow = p
			}
		}
		c, err := ctmCell(cap, narrow.class, narrow.lanes, b-a)
		if err != nil {
			return nil, fmt.Errorf("corridor: %w", err)
		}
		m.Mainline = append(m.Mainline, len(m.Net.Cells))
		m.Net.Cells = append(m.Net.Cells, c)
		m.Cells = append(m.Cells, CellInfo{Mainline, mid.edge, mid.way, a, b - a, narrow.lanes, narrow.inferred, -1})
		m.cellClass = append(m.cellClass, narrow.class)
	}

	// Which mainline boundary each split sits on: split k is between
	// mainline cells j-1 and j where bounds[j] == moved position.
	splitAt := map[int][]split{}
	for _, s := range splits {
		p := moved[s.atM]
		j := sort.SearchFloat64s(bounds, p-1e-6)
		if j <= 0 || j >= len(m.Mainline) || math.Abs(bounds[j]-p) > 1e-6 {
			return nil, fmt.Errorf("corridor: internal: split at %.1f m not on a cell boundary", p)
		}
		splitAt[j] = append(splitAt[j], s)
	}

	for _, id := range route {
		e := &g.Edges[id]
		if slices.Contains(opt.MainClasses, string(e.Class)) && len(refs(e)) == 0 {
			m.Gaps = append(m.Gaps, fmt.Sprintf("route uses a %s stretch with no ref tag in OSM (way %d, %.0f m); assumed part of the route", e.Class, firstWay(e), e.LengthM))
		}
	}

	// Route start: through traffic.
	start := g.Edges[route[0]].From
	if en, err := lk.entry(start, &g.Edges[route[0]]); err != nil {
		return nil, fmt.Errorf("corridor: %w", err)
	} else if en != nil {
		d, clock, err := lk.demand(en)
		if err != nil {
			return nil, fmt.Errorf("corridor: %w", err)
		}
		m.MainKey, m.ProfileStart = en.Key, clock
		m.Net.Origins = append(m.Net.Origins, ctm.Origin{Cell: m.Mainline[0], IntervalS: float64(demand.ProfileStepMin) * 60, DemandVph: d})
		if config.IsPlaceholder(en.Source) {
			m.Gaps = append(m.Gaps, fmt.Sprintf("route start demand (%s) is a placeholder", en.Key))
		}
		if config.IsEstimated(en.Source) {
			m.Estimated++
		}
	} else {
		m.Gaps = append(m.Gaps, fmt.Sprintf("no demand for traffic entering at the route start (node:%d); using 0", start))
	}

	for j := 1; j < len(m.Mainline); j++ {
		prev, next := m.Mainline[j-1], m.Mainline[j]
		ss := splitAt[j]
		if len(ss) == 0 {
			nd := ctm.Node{In: []int{prev}, Out: []int{next}}
			if m.Cells[next].Lanes < m.Cells[prev].Lanes {
				nd.CapDrop = capDrop(cap, g.Edges[m.Cells[next].EdgeID].Class) // lane drop: a bottleneck
				m.LaneDropNodes = append(m.LaneDropNodes, len(m.Net.Nodes))
			}
			m.Net.Nodes = append(m.Net.Nodes, nd)
			continue
		}
		if len(ss) > 1 {
			return nil, fmt.Errorf("corridor: internal: two splits on one boundary at %.1f m", bounds[j])
		}
		s := ss[0]
		ev := events[s.ev]
		if !s.merge {
			nd := ctm.Node{In: []int{prev}, Out: []int{next}}
			main := 1.0
			var shares []float64
			for _, e := range ev.outs {
				r, err := m.addRamp(g, cap, lk, demand, e, OffRamp, ev.node, ev.atM, minM)
				if err != nil {
					return nil, err
				}
				m.Ramps[r].MainLanes = m.Cells[next].Lanes
				nd.Out = append(nd.Out, m.Ramps[r].Cells[0])
				shares = append(shares, m.Ramps[r].Split)
				main -= m.Ramps[r].Split
			}
			if main <= 0 {
				return nil, fmt.Errorf("corridor: off-ramp splits at node %d sum to ≥ 1", ev.node)
			}
			nd.Split = append([]float64{main}, shares...)
			for i := len(m.Ramps) - len(ev.outs); i < len(m.Ramps); i++ {
				m.Ramps[i].Node = len(m.Net.Nodes)
			}
			m.Net.Nodes = append(m.Net.Nodes, nd)
			continue
		}
		nd := ctm.Node{In: []int{prev}, Out: []int{next}, CapDrop: capDrop(cap, g.Edges[m.Cells[next].EdgeID].Class)}
		for _, e := range ev.ins {
			r, err := m.addRamp(g, cap, lk, demand, e, OnRamp, ev.node, ev.atM, minM)
			if err != nil {
				return nil, err
			}
			m.Ramps[r].MainLanes = m.Cells[next].Lanes
			cells := m.Ramps[r].Cells
			nd.In = append(nd.In, cells[len(cells)-1])
		}
		for i := len(m.Ramps) - len(ev.ins); i < len(m.Ramps); i++ {
			m.Ramps[i].Node = len(m.Net.Nodes)
		}
		m.Net.Nodes = append(m.Net.Nodes, nd)
	}

	if demand != nil {
		for _, en := range demand.Entries {
			if !lk.used[en.Key] {
				m.Gaps = append(m.Gaps, fmt.Sprintf("demand entry %s (%s) is not on this route; ignored", en.Key, en.Name))
			}
		}
		for _, ex := range demand.Exits {
			if !lk.used[ex.Key] {
				m.Gaps = append(m.Gaps, fmt.Sprintf("demand exit %s (%s) is not on this route; ignored", ex.Key, ex.Name))
			}
		}
	}
	if err := m.Net.Validate(); err != nil {
		return nil, fmt.Errorf("corridor: %w", err)
	}
	return m, nil
}

// addRamp lays one side edge out as its own short chain of cells.
func (m *Model) addRamp(g *graph.Graph, cap *config.Capacity, lk *demandLookup, demand *config.Demand,
	e *graph.Edge, kind Kind, at graph.NodeID, atM, minM float64) (int, error) {
	idx := len(m.Ramps)
	r := Ramp{Kind: kind, Name: rampName(g, e, kind), EdgeID: e.ID, OSMWayIDs: e.OSMWayIDs, AtNode: at, AtM: atM, Class: e.Class}
	ps := pieces(g, []graph.EdgeID{e.ID}, 0)
	total := math.Max(e.LengthM, minM) // a ramp shorter than one step is stretched to one cell
	bounds, _ := cutCells(total, nil, nil, minM)
	for i := 0; i+1 < len(bounds); i++ {
		a, b := bounds[i], bounds[i+1]
		narrow := &ps[0]
		for j := range ps {
			p := &ps[j]
			if p.startM < b && p.endM > a && p.lanes < narrow.lanes {
				narrow = p
			}
		}
		c, err := ctmCell(cap, e.Class, narrow.lanes, b-a)
		if err != nil {
			return 0, fmt.Errorf("corridor: %w", err)
		}
		if i > 0 {
			m.Net.Nodes = append(m.Net.Nodes, ctm.Node{In: []int{len(m.Net.Cells) - 1}, Out: []int{len(m.Net.Cells)}})
		}
		r.Cells = append(r.Cells, len(m.Net.Cells))
		if r.Lanes == 0 || narrow.lanes < r.Lanes {
			r.Lanes = narrow.lanes
		}
		m.Net.Cells = append(m.Net.Cells, c)
		m.Cells = append(m.Cells, CellInfo{kind, e.ID, narrow.way, a, b - a, narrow.lanes, narrow.inferred, idx})
		m.cellClass = append(m.cellClass, e.Class)
	}
	switch kind {
	case OnRamp:
		en, err := lk.entry(e.From, e)
		if err != nil {
			return 0, fmt.Errorf("corridor: %w", err)
		}
		if en == nil {
			m.Gaps = append(m.Gaps, fmt.Sprintf("no demand for %s (way %d, joins at node:%d); using 0", r.Name, firstWay(e), at))
			break
		}
		d, clock, err := lk.demand(en)
		if err != nil {
			return 0, fmt.Errorf("corridor: %w", err)
		}
		if m.ProfileStart != "" && clock != m.ProfileStart {
			return 0, fmt.Errorf("corridor: profiles start at %s and %s; all must share one start", m.ProfileStart, clock)
		}
		m.ProfileStart = clock
		r.DemandKey, r.PeakVPH = en.Key, en.PeakVPH
		m.Net.Origins = append(m.Net.Origins, ctm.Origin{Cell: r.Cells[0], IntervalS: float64(demand.ProfileStepMin) * 60, DemandVph: d})
		if config.IsPlaceholder(en.Source) {
			m.Gaps = append(m.Gaps, fmt.Sprintf("%s demand (%s) is a placeholder", r.Name, en.Key))
		}
		if config.IsEstimated(en.Source) {
			m.Estimated++
		}
	case OffRamp:
		ex, err := lk.exit(e)
		if err != nil {
			return 0, fmt.Errorf("corridor: %w", err)
		}
		if ex == nil {
			m.Gaps = append(m.Gaps, fmt.Sprintf("no split ratio for %s (way %d, leaves at node:%d); using 0", r.Name, firstWay(e), at))
			break
		}
		r.DemandKey, r.Split = ex.Key, ex.SplitRatio
		if config.IsPlaceholder(ex.Source) {
			m.Gaps = append(m.Gaps, fmt.Sprintf("%s split (%s) is a placeholder", r.Name, ex.Key))
		}
		if config.IsEstimated(ex.Source) {
			m.Estimated++
		}
	}
	m.Ramps = append(m.Ramps, r)
	return idx, nil
}
