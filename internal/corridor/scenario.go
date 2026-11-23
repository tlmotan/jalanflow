package corridor

import (
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/tlmotan/jalanflow/internal/config"
	"github.com/tlmotan/jalanflow/internal/ctm"
	"github.com/tlmotan/jalanflow/internal/graph"
)

// Fix scenarios (decision 0008): each is one change to the CTM network,
// run alone against the same baseline and demand, ranked by delay saved.
// Signal-timing fixes are not built: no surveyed timings yet.

// FixType names a kind of fix, as written to scenarios.json.
type FixType string

// Fix types.
const (
	Restripe   FixType = "restripe"
	Channelise FixType = "channelise"
	RampMeter  FixType = "ramp_meter"
	PlusLane   FixType = "plus_lane"
)

// SiteKind says what kind of squeeze a site is.
type SiteKind string

// Site kinds.
const (
	MergeSite    SiteKind = "merge"     // a ramp joins the route
	LaneDropSite SiteKind = "lane_drop" // the route itself loses a lane
)

// Site is a place on the route where a fix can be tried.
type Site struct {
	Kind   SiteKind
	Key    string  // "node:<osm id>" for a merge, "way:<from>><to>" for a lane drop
	Name   string  // plain description
	AtM    float64 // distance along the route
	EdgeID graph.EdgeID
	// LanesIn → LanesOut on the route (plus RampLanes joining, for a merge).
	LanesIn, LanesOut, RampLanes int

	node  int   // index in Net.Nodes
	ramps []int // Model.Ramps joining here
}

// Outcome is what one simulation run produced.
type Outcome struct {
	DelayVehH float64 // vehicle-hours beyond free flow, origin queues included
	// UnservedVeh are vehicles still on the road or waiting to enter when
	// the run ends; their remaining delay is not in DelayVehH.
	UnservedVeh float64
	// CommuteMin is the mean simulated travel time over the observed
	// departures of the longest observed trip.
	CommuteMin float64
	// Queues are the queued stretches of the route [start m, end m] at the
	// moment the baseline holds the most vehicles.
	Queues [][2]float64
}

// Scenario is one fix at one site and how it compares with the baseline.
type Scenario struct {
	ID              string
	Type            FixType
	Site            Site
	Change          string
	Outcome         Outcome
	DelaySavedVehH  float64
	CommuteSavedMin float64
	Helps           bool
	// Calibrated is false when the site lies past the last observation:
	// the model there is not checked against anything (decision 0009).
	Calibrated       bool
	Confidence       string
	ConfidenceReason string
}

// ScenarioSet is the baseline and every fix tried, best first.
type ScenarioSet struct {
	Baseline  Outcome
	Scenarios []Scenario
	// AtClock is the time of day Outcome.Queues refers to.
	AtClock string
}

// Sites lists every merge and lane drop on the route, in route order.
func (m *Model) Sites() []Site {
	var out []Site
	byNode := map[int]int{}
	for i, r := range m.Ramps {
		if r.Kind != OnRamp {
			continue
		}
		nd := m.Net.Nodes[r.Node]
		if j, ok := byNode[r.Node]; ok {
			out[j].ramps = append(out[j].ramps, i)
			out[j].RampLanes += r.Lanes
			continue
		}
		byNode[r.Node] = len(out)
		out = append(out, Site{
			Kind: MergeSite, Key: fmt.Sprintf("node:%d", r.AtNode), Name: r.Name + " joins the route",
			AtM: m.Cells[nd.Out[0]].StartM, EdgeID: m.Cells[nd.Out[0]].EdgeID,
			LanesIn: m.Cells[nd.In[0]].Lanes, LanesOut: m.Cells[nd.Out[0]].Lanes, RampLanes: r.Lanes,
			node: r.Node, ramps: []int{i},
		})
	}
	for _, n := range m.LaneDropNodes {
		nd := m.Net.Nodes[n]
		in, o := m.Cells[nd.In[0]], m.Cells[nd.Out[0]]
		out = append(out, Site{
			Kind: LaneDropSite, Key: fmt.Sprintf("way:%d>%d", in.OSMWayID, o.OSMWayID),
			Name: fmt.Sprintf("route narrows from %d to %d lanes", in.Lanes, o.Lanes),
			AtM:  o.StartM, EdgeID: o.EdgeID, LanesIn: in.Lanes, LanesOut: o.Lanes, node: n,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AtM < out[j].AtM })
	return out
}

func cloneNet(n ctm.Network) ctm.Network {
	c := n
	c.Cells = slices.Clone(n.Cells)
	c.Nodes = slices.Clone(n.Nodes)
	c.Origins = slices.Clone(n.Origins)
	return c
}

// widen adds a lane to the mainline cells starting at cell `from` for up
// to lengthM (0 = as far as the road keeps the same width), stopping where
// the road already widens. It returns the length widened and marks the new
// lane drop at its end with capDrop.
func (m *Model) widen(net *ctm.Network, from int, lengthM, capDrop float64) float64 {
	start := slices.Index(m.Mainline, from)
	if start < 0 {
		return 0
	}
	lanes := m.Cells[from].Lanes
	done := 0.0
	last := -1
	for k := start; k < len(m.Mainline); k++ {
		c := m.Mainline[k]
		if m.Cells[c].Lanes != lanes || (lengthM > 0 && done >= lengthM) {
			break
		}
		net.Cells[c].Lanes++
		done += m.Cells[c].LengthM
		last = k
	}
	// Where the extra lane ends and the road is narrower again, a new lane
	// drop appears.
	if last >= 0 && last+1 < len(m.Mainline) {
		next := m.Mainline[last+1]
		if m.Cells[next].Lanes <= lanes {
			for i := range net.Nodes {
				if len(net.Nodes[i].Out) > 0 && net.Nodes[i].Out[0] == next && len(net.Nodes[i].In) == 1 {
					net.Nodes[i].CapDrop = math.Max(net.Nodes[i].CapDrop, capDrop)
				}
			}
		}
	}
	return done
}

// apply returns the network with one fix in place and a plain description,
// or ok = false when the fix does not apply to the site.
func (m *Model) apply(cap *config.Capacity, s Site, t FixType) (net ctm.Network, change string, ok bool) {
	net = cloneNet(m.Net)
	nd := &net.Nodes[s.node]
	first := nd.Out[0]
	class := graph.BaseClass(graphClass(m, first))
	drop := cap.CTM[class].CapacityDrop
	switch t {
	case PlusLane:
		l := m.widen(&net, first, 0, drop)
		if l == 0 {
			return net, "", false
		}
		if s.Kind == LaneDropSite {
			nd.CapDrop = 0 // the road no longer narrows here
		}
		return net, fmt.Sprintf("Add a lane for %.0f m (%d → %d lanes)", l, s.LanesOut, s.LanesOut+1), true
	case Restripe:
		if s.Kind != MergeSite {
			return net, "", false
		}
		l := m.widen(&net, first, cap.Restripe.AuxLaneM, drop)
		if l == 0 {
			return net, "", false
		}
		nd.CapDrop = 0
		return net, fmt.Sprintf("Ramp keeps its own lane for %.0f m before merging (road after the merge %d → %d lanes)", l, s.LanesOut, s.LanesOut+1), true
	case Channelise:
		if s.Kind != MergeSite || nd.CapDrop == 0 {
			return net, "", false
		}
		// Only the capacity drop changes. Decision 0008 first also gave
		// every inflow an equal share ("zipper 1:1"), but taking turns lane
		// by lane is the lane-proportional split the model already uses;
		// 1:1 per road handed a 2-lane ramp half of a 5-lane road and made
		// the real corridor far worse (docs/validation.md, 2026-11-23).
		nd.CapDrop *= 1 - cap.Channelisation.CapacityDropReduction
		return net, fmt.Sprintf("Dividers / solid lines so cars merge in turn (assumed to remove %.0f%% of the flow lost when the merge jams)", cap.Channelisation.CapacityDropReduction*100), true
	case RampMeter:
		if s.Kind != MergeSite {
			return net, "", false
		}
		kj := cap.CTM[class].JamDensity
		metered := 0
		for _, ri := range s.ramps {
			r := m.Ramps[ri]
			for oi := range net.Origins {
				if net.Origins[oi].Cell != r.Cells[0] {
					continue
				}
				storage := 0.0
				for _, c := range r.Cells {
					storage += net.Cells[c].JamDensity() * net.Cells[c].LengthM / 1000
				}
				capVph := net.Cells[r.Cells[0]].CapacityVph()
				if capVph <= cap.ALINEA.MinRateVPH {
					continue
				}
				net.Origins[oi].Meter = &ctm.Meter{
					MeasureCells:         []int{nd.In[0], first},
					TargetDensityPerLane: cap.ALINEA.TargetOccupancyPct / 100 * kj,
					GainVph:              cap.ALINEA.GainVPHPerPct * 100 / kj,
					MinVph:               cap.ALINEA.MinRateVPH,
					MaxVph:               capVph,
					MaxQueueVeh:          storage,
					UpdateS:              cap.ALINEA.UpdateS,
				}
				metered++
			}
		}
		if metered == 0 {
			return net, "", false
		}
		return net, "Traffic light on the ramp that lets cars in at a controlled rate (ALINEA)", true
	}
	return net, "", false
}

func graphClass(m *Model, cell int) graph.HighwayClass {
	return m.cellClass[cell]
}

func confidence(cap *config.Capacity, t FixType) (string, string) {
	switch t {
	case Channelise:
		return cap.Channelisation.Confidence, "The model cannot see a divider's shape or how drivers react to it; it only tests what happens if the divider removes the assumed share of the lost flow."
	case RampMeter:
		return "medium", "A well-studied method, but the gain depends on how accurate the traffic figures are. Waiting time at the ramp light is counted."
	case Restripe:
		return "medium", "The model handles lane counts and merges well, but assumes there is room for the extra lane."
	default:
		return "medium", "The model handles lane counts well. It ignores the extra traffic a wider road attracts."
	}
}

func (m *Model) outcome(net ctm.Network, steps, atStep int, trip []TravelObs) (Outcome, *ctm.Result, error) {
	res, err := ctm.Run(net, steps)
	if err != nil {
		return Outcome{}, nil, err
	}
	o := Outcome{DelayVehH: res.DelayVehH(net)}
	last := len(res.Vehicles) - 1
	for _, v := range res.Vehicles[last] {
		o.UnservedVeh += v
	}
	for _, q := range res.OriginQueue[last] {
		o.UnservedVeh += q
	}
	mm := *m
	mm.Net = net
	fits, err := mm.Compare(res, trip)
	if err != nil {
		return Outcome{}, nil, err
	}
	for _, f := range fits {
		o.CommuteMin += f.SimMinutes / float64(len(fits))
	}
	start := -1.0
	for i, c := range m.Mainline {
		info := m.Cells[c]
		q := res.Queued(net, atStep, c)
		if q && start < 0 {
			start = info.StartM
		}
		if start >= 0 && !q {
			o.Queues = append(o.Queues, [2]float64{start, info.StartM})
			start = -1
		}
		if start >= 0 && i == len(m.Mainline)-1 {
			o.Queues = append(o.Queues, [2]float64{start, info.StartM + info.LengthM})
		}
	}
	return o, res, nil
}

// Scenarios runs the baseline and every fix that applies at every site.
// obs are the observed trips (the longest gives the commute figure);
// calibratedToM is where the last observation ends; extraS is simulated
// time after the demand profile ends.
func (m *Model) Scenarios(cap *config.Capacity, obs []TravelObs, calibratedToM, extraS float64) (*ScenarioSet, error) {
	if len(obs) == 0 {
		return nil, fmt.Errorf("corridor: no observed trips to measure the commute on")
	}
	far := obs[0]
	for _, o := range obs {
		if o.ToM-o.FromM > far.ToM-far.FromM {
			far = o
		}
	}
	var trip []TravelObs
	for _, o := range obs {
		if o.FromM == far.FromM && o.ToM == far.ToM {
			trip = append(trip, o)
		}
	}
	steps := m.Steps(extraS)
	base, res, err := m.outcome(m.Net, steps, 0, trip)
	if err != nil {
		return nil, err
	}
	// Queues are read at the moment the baseline holds the most vehicles.
	at, most := 0, -1.0
	for step := range res.Vehicles {
		n := 0.0
		for _, v := range res.Vehicles[step] {
			n += v
		}
		if n > most {
			at, most = step, n
		}
	}
	if base, _, err = m.outcome(m.Net, steps, at, trip); err != nil {
		return nil, err
	}
	startMin, err := clockMin(m.ProfileStart)
	if err != nil {
		return nil, err
	}
	clock := int(startMin) + int(float64(at)*m.Net.DtS/60)
	set := &ScenarioSet{Baseline: base, AtClock: fmt.Sprintf("%02d:%02d", clock/60, clock%60)}

	for _, s := range m.Sites() {
		for _, t := range []FixType{Restripe, Channelise, RampMeter, PlusLane} {
			net, change, ok := m.apply(cap, s, t)
			if !ok {
				continue
			}
			o, _, err := m.outcome(net, steps, at, trip)
			if err != nil {
				return nil, fmt.Errorf("corridor: %s at %s: %w", t, s.Key, err)
			}
			conf, why := confidence(cap, t)
			saved := base.DelayVehH - o.DelayVehH
			set.Scenarios = append(set.Scenarios, Scenario{
				ID: fmt.Sprintf("%s-%s", t, s.Key), Type: t, Site: s, Change: change, Outcome: o,
				DelaySavedVehH: saved, CommuteSavedMin: base.CommuteMin - o.CommuteMin,
				// Under a vehicle-hour is rounding, not a result.
				Helps:      saved >= 1,
				Calibrated: s.AtM <= calibratedToM, Confidence: conf, ConfidenceReason: why,
			})
		}
	}
	sort.SliceStable(set.Scenarios, func(i, j int) bool {
		return set.Scenarios[i].DelaySavedVehH > set.Scenarios[j].DelaySavedVehH
	})
	return set, nil
}
