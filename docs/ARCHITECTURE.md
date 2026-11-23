# ARCHITECTURE — JalanFlow

> Status: v0.3 (Phase 1.5 simulation planned, §3.6) · Read ARCHITECTURE-ESSENTIALS.md first if you are an agent.

## 1. Overview

Phase 1 is a **batch pipeline + static website**. No server, no database.

```
 Overpass API (bbox + road classes) ──► osmium cat ──► data/raw/<corridor>.osm.pbf
 (or: Geofabrik .osm.pbf ──► osmium extract (bbox), OSM_SOURCE=geofabrik)
                                                        │
                                          cmd/build-graph (Go)
                                                        ▼
                                        data/interim/<corridor>.graph.json
                                                        │
                                           cmd/analyze (Go)
                                                        ▼
                         web/public/data/<corridor>/{edges.geojson, fixes.json, manifest.json}
                                                        │
                                     web/ (Vite + TypeScript + MapLibre)
                                                        ▼
                                     GitHub Pages (deployed by GitHub Actions)
```

Why two Go commands, not one: building the graph is slow and changes rarely; analysis is fast and changes often. Splitting them lets you iterate on the algorithm without re-parsing OSM, and gives a clean, testable boundary (`graph.json`).

## 2. Tech stack

| Layer | Choice | Why |
|---|---|---|
| Language (engine) | **Go 1.23+** | Resume goal; fast; single static binaries |
| OSM parsing | `github.com/paulmach/osm` (+ `osmpbf`) | Mature Go OSM library; streams PBF; built with `CGO_ENABLED=0` (set in Makefile) so no C zlib / pkg-config is needed |
| Fetching | Overpass API by default (a few MB: only the corridor bbox and included classes, with `osm_base` date written into the PBF header); Geofabrik extract + `osmium extract` as fallback | The full Geofabrik extract is ~250 MB and was impractically slow to download; Overpass returns complete ways like `--strategy complete_ways` |
| Format conversion / clipping | `osmium-tool` (CLI, external) | XML→PBF and bbox clipping; faster and more correct than doing it in Go |
| Max flow | **Own Dinic's implementation** in `internal/flow` | Small (~150 lines), good interview talking point, no good Go lib |
| Config | JSON files in `config/` | Zero dependencies; std `encoding/json` |
| Output | GeoJSON (RFC 7946) + JSON | Any map library reads it |
| Frontend | **Vite + TypeScript + MapLibre GL JS** | Free, open-source, vector maps, no API key needed with open tiles |
| Base map tiles | OpenFreeMap or other free OSM-based style | Free; verify terms before launch |
| Hosting | **GitHub Pages**, deployed by the CI workflow on every green push to `main` (decision 0006) | Free, no extra account or secrets; site lives at `/jalanflow/`, so the web code builds URLs from `import.meta.env.BASE_URL` |
| CI | GitHub Actions (`.github/workflows/ci.yml`): `gofmt`, `go vet`, `go test` (incl. a check that committed `web/public/data/` matches the output structs and the current capacity config), `npm run build` | Keeps main green; data is not regenerated in CI because raw OSM is not committed |
| Prototyping (optional) | Python + OSMnx/NetworkX notebooks in `notebooks/` | Only to cross-check Go results; never in the pipeline |

**Explicitly not used in Phase 1:** database, API server, Docker, Kubernetes, message queues, scheduled jobs, auth.

## 3. Modelling decisions

### 3.1 Graph
- **Directed graph.** One OSM way segment between two junctions → one edge per allowed direction.
- `oneway=yes` → forward arc only. `oneway=-1` → reverse arc only. Otherwise two arcs.
- **Lanes per direction:** use `lanes:forward` / `lanes:backward` if tagged. Else if one-way, `lanes`. Else `lanes / 2` (rounded down, min 1). Else infer from `highway` class default. Inferred → `lanes_inferred = true`.
- **Splitting:** ways are cut at nodes used twice or more, at way endpoints, and at traffic-signal nodes (so the signal factor only hits the segment ending at the signal). Segments that start and end at the same node are dropped.
- **Simplification:** OSM splits roads into many tiny ways. Merge consecutive arcs through a pass-through node — one-way: exactly one in-arc and one out-arc; two-way: exactly two neighbours, each with an arc in and out — when the road attributes match (`class, name, ref, toll, operator, bridge, tunnel, maxspeed`). **Lanes and signals may differ**; otherwise the pinch could never be merged. Capacity, lanes and `lanes_inferred` of a merged edge come from its **lowest-capacity part** (that is the point — a short 2-lane pinch inside a 4-lane road must survive merging). The parts are kept in `Edge.Parts` so fix ranking can see the next-narrowest part after a +1 lane fix. Bridge/tunnel are compared so a structure is never hidden inside a longer edge.
- **Reading OSM:** `osm.ReadPBF` makes two passes over the clip: ways first (keep included classes, skip `area=yes` and ways with < 2 nodes), then only the nodes those ways reference. The class list comes from the keys of `capacity-defaults.json`, so there is one source of truth. A referenced node missing from the file is an error (the clip must use `--strategy complete_ways`).
- **Included road classes:** `motorway, trunk, primary, secondary, tertiary` and their `_link` roads. Residential and service roads excluded in Phase 1 (they carry little through-traffic and explode graph size).
- **Junction capacity:** Phase 1 treats junction nodes as unlimited. This is a known limitation (signals and roundabouts are often the real bottleneck) — shown on the method page. Phase 2 adds node splitting.

### 3.2 Capacity
`capacity_vph = lanes_per_direction × per_lane_vph[highway_class] × signal_factor`

- `per_lane_vph` and `signal_factor` live in `config/capacity-defaults.json`.
- Starting values are **placeholders** to be calibrated against the Malaysian Highway Capacity Manual (MHCM) or published local studies. Every value carries a `source` field in the config.
- `signal_factor` applies to edges ending at a node tagged `highway=traffic_signals` (Phase 1: a flat factor, e.g. 0.5, to represent green time share).

### 3.3 Sources and sinks ("gates")
- Each corridor config lists **gates**: named points (lat/lon) snapped to the nearest graph node, each marked `source` or `sink`.
- A **super-source** links to all source gates and a **super-sink** from all sink gates, with infinite capacity.
- **Gate snapping is per direction.** On a dual carriageway the nearest node may be on the wrong side, so a source gate snaps to the nearest node whose outgoing road runs in the direction of travel, and a sink to one whose incoming road does (KESAS eastbound / LDP northbound for inbound). Checked 2026-11-22: naive nearest-node snapping put a gate on a side street and capped max flow at that street.
- Implemented as `graph.SnapGate`: candidates are only nodes on `gate_snap.classes` roads (motorway/trunk) within `gate_snap.max_m`; "runs the right way" = the edge's far end is nearer the other gate than its start. Checked 2026-11-23: the west gate is ~360 m from the nearest KESAS junction nodes with secondary roads nearer, so `max_m` is 500; the east gate sits on the LDP exit-1108 diverge and its inbound sink is the node where the 4-lane mainline arrives.
- With one gate per side the min cut is usually the gate road itself (2026-11-23 OSM, placeholder capacities: 6,000 veh/h each way; inbound cut = KESAS eastbound at the source, outbound cut = LDP southbound at the source). That is a property of single-point gates (decision 0003), not a finding about the network.
- Run per **direction** (`inbound`: Kota Kemuning → PJ; `outbound`: reverse) by swapping gate roles.
- Max flow therefore = the most vehicles/hour the network can move between the gate sets. The min cut = the set of road segments that together cap that number.

### 3.4 Algorithms
| Step | Algorithm | Complexity | Notes |
|---|---|---|---|
| Max flow | Dinic's | O(V²E), fast in practice | Integer capacities (veh/h) |
| Min cut | BFS on residual graph from source | O(V+E) | Cut = arcs from reachable to unreachable |
| Bottleneck score | `flow / capacity` per edge, plus `is_cut` | O(E) | Saturated non-cut edges shown amber |
| Fixes that help | Picard–Queyranne: arc u→v is in **every** min cut iff u reachable from s and v reaches t in the residual graph | O(V+E) | Only these arcs can raise max flow when widened (decision 0005) |
| Fix ranking | For each cut edge: +1 lane on its narrowest part; if it is in every min cut, re-run Dinic and record gain + new cut, else `helps=false`, gain 0, no re-run | O(k · Dinic) | k = cut size, small |

Min cuts are **not unique**. We report the cut nearest the source (from the residual BFS) and say so. Phase 2 may also compute the cut nearest the sink.

### 3.5 Fix difficulty (Phase 1 heuristic)
| Signal (from OSM) | Effect |
|---|---|
| `bridge=yes` or `tunnel=yes` | hard |
| `toll=yes` or known concessionaire `operator` | medium+, reason "needs concessionaire agreement" |
| `highway` = motorway/trunk with no above | medium |
| primary and below with no above | easy (re-stripe / junction change) |

Reasons are stored as a list of strings and shown to the user verbatim.

### 3.6 Simulation (Phase 1.5 — planned, decisions 0007 and 0008)

Phase 1 is unchanged. Phase 1.5 adds traffic demand and a **Cell Transmission Model** (Daganzo 1994/95) so fixes can be tested for delay saved, not just capacity gained.

**Packages**
- `internal/ctm` — pure simulator, like `flow`: no I/O, no logging, no internal imports. Input: cells, nodes (merge/diverge/source/sink/signal), demand profiles, controllers. Output: per-cell density/flow per time step, plus totals. Deterministic.
- `internal/corridor` — builds the CTM network from the graph: walks KESAS E5 + LDP E11 per direction along the Phase 1 route, cuts it into cells (~200–500 m, cell length ≥ free-flow speed × Δt so CTM is stable), attaches on-ramps as source cells and off-ramps as sinks, takes lanes per cell from `Edge.Parts` (minimum, as everywhere). Depends on `graph`, `config`, `ctm`.
- `cmd/analyze` runs baseline + every fix and `export` writes `scenarios.json`.

**Model**
- Triangular fundamental diagram per cell: free-flow speed v, capacity Q = lanes × per-lane veh/h, jam density K_j = lanes × per-lane jam density, wave speed w. Sending S = min(v·ρ, Q), receiving R = min(Q, w·(K_j − ρ)). All parameters in config with `source`.
- **Merge** (on-ramp): inflow split by priority (default: proportional to lanes/capacity) when the downstream receiving flow is short. **Diverge** (off-ramp): fixed split ratios from demand data (FIFO, so a full off-ramp blocks the mainline — this is how the exit-510 queue spills back).
- **Capacity drop:** once a cell's upstream queue is active, its discharge falls to Q × (1 − drop); the drop fraction is in config with `source`.
- **ALINEA** ramp metering on chosen on-ramps: r(k) = r(k−1) + K_R · (ρ_target − ρ_downstream), clamped to [r_min, ramp capacity]; ramp queue limited by storage, overflow counted as delay.
- **Stepped signals:** signal node passes capacity × 1 on green and 0 on red, per cycle from config (observed timings) or Webster's formula for the retiming fix.
- **Fixed routes** (decision 0007): demand enters at on-ramps by time-of-day profile and leaves by split ratios. Nobody re-routes; no induced demand. Stated on the site.

**Demand and calibration**
- Demand: JKR/HPU Road Traffic Volume Malaysia (census station AADT + hourly profile), LLM highway statistics (plaza transactions). Converted to 15-min peak profiles at each on-ramp; off-ramp split ratios from the same. Each input carries source and year.
- Calibration targets: Google Maps typical peak travel times per section (recorded manually with date/time) and local queue observations (where the queue head and tail sit).
- **Validation gate** (checked in a test against `docs/validation.md` targets): baseline reproduces (1) the Sri Muda queue with its head at drop #21 (node 33987957, exit-507 merge), (2) the exit-510 Subang ramp queue, (3) peak travel times within ≤ 20 %. If it fails, `scenarios.json` has `validated: false` and no fixes.

**Fix types** (decision 0008, each run alone against the baseline): re-striping (lane-add instead of merge, longer auxiliary lane), dividers/channelisation (lower capacity drop (merge shares unchanged; decision 0008 as amended), low confidence), ramp metering (ALINEA), +1 lane, signal timing (Webster retiming, new-signal scenario). Ranked by delay saved (veh-h per peak).

## 4. Data models

All IDs are stable within a single pipeline run. OSM IDs are kept for traceability.

### 4.1 Go (`internal/graph`)

```go
type NodeID int64   // OSM node id
type EdgeID int32   // dense index into Graph.Edges

type LatLon struct{ Lat, Lon float64 }

type Node struct {
    ID      NodeID
    Pos     LatLon
    Signals bool // highway=traffic_signals
}
// osm.Node also reads barrier=toll_booth; booths end up in Graph.TollBooths.

type HighwayClass string // "motorway", "trunk", "primary", ... (+ "_link")

type Edge struct {
    ID            EdgeID
    From, To      NodeID
    OSMWayIDs     []int64   // ways merged into this edge
    Name          string
    Ref           string    // e.g. "E5"
    Class         HighwayClass
    Lanes         int       // per direction
    LanesInferred bool
    LengthM       float64
    MaxSpeedKPH   int       // 0 = unknown
    CapacityVPH   int
    Toll          bool
    Operator      string
    Bridge, Tunnel bool
    Geometry      []LatLon  // for the map
    Parts         []Segment // merged OSM pieces, travel order; Lanes/Capacity come from the min part
}

type Segment struct {
    OSMWayID      int64
    Lanes         int
    LanesInferred bool
    Signalised    bool    // ends at traffic signals
    LengthM       float64
    CapacityVPH   int
    End           LatLon  // where this part stops; inside an edge, where the next part begins
}

type Graph struct {
    Corridor    string
    OSMDate     string // from the PBF header / file
    Nodes       map[NodeID]*Node
    Edges       []Edge
    TollBooths  []LatLon // barrier=toll_booth nodes, sorted; lane drops skip points near them
}
```

### 4.2 Corridor config (`config/corridors/<id>.json`)

```go
type Corridor struct {
    ID      string     // "kemuning-pj"
    Name    string
    BBox    [4]float64 // minLon, minLat, maxLon, maxLat
    Gates   []Gate
    GateSnap GateSnap // {classes, max_m, source}: see §3.3
}
type Gate struct {
    Name string
    Pos  LatLon
    Role string // "west" | "east"  (direction decides which is source)
}
// Corridor.LaneDrops (optional; cmd/analyze needs it):
type LaneDrops struct {
    RouteRefs        []string // ["E5","E11"]: only drops on these roads or ramps touching them
    Classes          []string // grade-separated classes checked
    TollPlazaRadiusM float64  // skip drops this close to a toll booth
    Source           string
    Confirmed        []ConfirmedDrop // {key, name, pin, source}; key must match a drop or analyze fails
}
```

### 4.3 Analysis (`internal/flow`, `internal/fixes`)

```go
type Direction string // "inbound" | "outbound"

type FlowResult struct {          // fixes.Result
    Direction  Direction
    MaxFlowVPH int
    EdgeFlow   []int    // indexed by EdgeID
    CutEdges   []EdgeID
    Steps      []Step   // baseline augmenting paths, in the order found
}

type Step struct {                // one augmenting path on road edges
    Phase     int                 // Dinic phase (BFS level graph), from 1
    AmountVPH int
    Edges     []EdgeDelta         // {EdgeID, Delta}: +AmountVPH, or -AmountVPH where flow was cancelled
}

type Difficulty string // "easy" | "medium" | "hard"

type Fix struct {
    EdgeID        EdgeID
    Direction     Direction
    Change        string   // "+1 lane" (Phase 1 only change type)
    GainVPH       int
    NewMaxFlowVPH int
    NextCutEdges  []EdgeID // where the bottleneck moves
    Difficulty    Difficulty
    Reasons       []string
    Helps         bool     // +1 lane alone raises max flow (edge in every min cut)
    WhyNoGain     string   // plain words when !Helps: other equally tight cut, other equally narrow part, or unknown lanes
}
```
- `flow.MaxFlowTrace` is `MaxFlow` that also records every augmenting path (`flow.Augment{Phase, Amount, Arcs, Reverse}`). `fixes.Rank` traces the baseline run only; the +1 lane re-runs use plain `MaxFlow`. Summing a direction's step deltas per edge gives `EdgeFlow` exactly (tested). Written to `replay.json` (§4.4).

### 4.3b Lane drops (`internal/lanedrop`) — the site headline (decision 0004)

```go
type Drop struct {
    Rank          int     // 1 = biggest squeeze
    Key           string  // "node:<osm node>" (merge) | "way:<from>><to>" (narrowing); stable across rebuilds
    Kind          Kind    // "merge" | "narrowing"
    Pos           LatLon
    LanesIn, LanesOut, LanesLost int
    Squeeze       float64 // LanesOut / LanesIn
    From, To      []Road  // {osm_way_id, name, ref, class, lanes, lanes_inferred, leaves, joins}
    LanesInferred bool    // any lane count here is a class default
    Confirmed     *Confirmation // {name, source} or null
}
```
- **Merge:** node without signals, all roads in `Classes`, one on `RouteRefs`, Σ lanes in > Σ lanes out. Lanes are taken from the part of each edge that touches the node, not the edge minimum.
- **Narrowing:** consecutive `Edge.Parts` where lanes fall; edge on the route or a ramp touching it. Position = `Parts[k-1].End`.
- Unnamed ramps get `leaves` / `joins`: the nearest named road (ref, else name) where the ramp starts / ends, found breadth-first through other ramps (ties → lowest edge ID). The site titles ramps "ramp E5 → E11".
- Skip within `TollPlazaRadiusM` of a toll booth. Order: squeeze asc, lanes lost desc, key.
- Lanes only: no demand, so rank ≠ how bad the jam is. Said on the site.

### 4.4 Output files (`web/public/data/<corridor>/`)

**`edges.geojson`** — FeatureCollection, one LineString per edge **per direction analysed**, only edges carrying flow in that direction (cut edges always do), plus any edge a replay step touched (it may end with `flow_vph` 0 if a later path cancelled its flow). Compact JSON, coordinates rounded to 6 decimals: the whole bbox would be ~19 MB.
```json
{ "type": "Feature",
  "geometry": { "type": "LineString", "coordinates": [[101.5, 3.07], ...] },
  "properties": {
    "id": 42, "dir": "inbound", "name": "Lebuhraya Shah Alam", "ref": "E5",
    "class": "trunk", "lanes": 3, "lanes_inferred": false,
    "capacity_vph": 5400, "flow_vph": 5400, "utilisation": 1.0, "is_cut": true
  } }
```

**`fixes.json`** — `{ "inbound": Fix[], "outbound": Fix[] }`, sorted by `GainVPH` desc (then edge id). One entry per source-side cut edge, including `helps: false` ones so the site can say why they gain nothing.

**`replay.json`** — the baseline max flow search, path by path, for the site's "Run the analysis" replay (decision 0011). Compact JSON (`export.ReplayFile`). Edges are `[edge id, delta veh/h]` pairs, source to sink; delta < 0 where the path cancelled earlier flow. Per direction, summing deltas per edge gives `flow_vph` in `edges.geojson`, and summing `amount_vph` gives `max_flow_vph` (the committed-data test checks both).
```json
{ "inbound": { "max_flow_vph": 6000, "phases": 7,
               "steps": [ { "phase": 1, "amount_vph": 1200, "edges": [[293, 1200], [295, 1200]] } ] },
  "outbound": { "...": "same shape" } }
```

**`lane_drops.json`** — `{ corridor, osm_date, generated_at, ranking, assumptions[], drops: Drop[] }` (`export.LaneDropsFile`), drops in rank order.

**`scenarios.json`** — *planned, Phase 1.5; not produced yet.* Shape (Go struct in `internal/export` will be the source of truth, mirrored in `web/src/types.ts` when built):
```json
{ "corridor": "kemuning-pj", "generated_at": "...", "validated": true,
  "demand_sources": ["JKR/HPU RTVM 2024", "LLM highway statistics 2024"],
  "validation": { "jams_reproduced": ["drop:node:33987957", "way:148018437>1116586907"],
                  "travel_time_error_pct": 12.0 },
  "assumptions": ["Fixed routes: nobody re-routes", "..."],
  "inbound": {
    "baseline": { "delay_veh_h": 0, "commute_min": 0,
                  "heat": { "t0": "07:00", "dt_s": 60, "cell_km": [0.0, 0.3], "speed_kph": [[0]] } },
    "fixes": [ { "id": "restripe-node-33987957", "type": "restripe",
                 "where": { "key": "node:33987957", "pos": [101.55728, 3.04031] },
                 "change": "Exit-507 ramp gets its own lane (3 + 2 → 4)",
                 "delay_saved_veh_h": 0, "commute_min_before": 0, "commute_min_after": 0,
                 "queue_moves_to": "node:...", "difficulty": "easy", "reasons": ["..."],
                 "confidence": "medium", "confidence_reason": "...",
                 "heat": { "...": "same shape as baseline" } } ] },
  "outbound": { "...": "same shape" } }
```
`type` ∈ `restripe | channelise | ramp_meter | plus_lane | signal_timing`; `confidence` ∈ `low | medium | high`. Fixes sorted by `delay_saved_veh_h` desc. When `validated` is false, `fixes` is empty.

**`manifest.json`**
```json
{ "corridor": "kemuning-pj", "generated_at": "2026-11-21T00:00:00Z",
  "osm_date": "2026-11-20", "pipeline_version": "0.1.0",
  "capacity_config_hash": "sha256:...",
  "max_flow_vph": { "inbound": 0, "outbound": 0 },
  "assumptions": ["Junctions treated as unlimited capacity", "..."] }
```

### 4.5 Frontend types (`web/src/types.ts`)
Mirror the JSON above exactly. The Go structs are the source of truth; if a field changes in Go, change `types.ts` in the same commit.

## 5. Repository layout

```
jalanflow/
├── AGENTS.md / CLAUDE.md      agent instructions (AGENTS.md is canonical)
├── Makefile                   fetch, graph, analyze, web, test, all
├── go.mod
├── cmd/
│   ├── build-graph/           OSM PBF → graph.json
│   └── analyze/               graph.json → web/public/data/...
├── internal/
│   ├── config/                load + validate corridor & capacity JSON
│   ├── osm/                   read PBF, filter ways, tag parsing (lanes, oneway)
│   ├── graph/                 data model, build, simplify, capacity
│   ├── flow/                  Dinic, min cut (pure, no I/O)
│   ├── lanedrop/              lane drops: merges and narrowings on the route
│   ├── fixes/                 +1 lane ranking, difficulty heuristic
│   ├── ctm/                   (Phase 1.5, planned) cell transmission model, pure
│   ├── corridor/              (Phase 1.5, planned) graph → CTM cell chain, demand
│   └── export/                GeoJSON / JSON writers
├── config/
│   ├── capacity-defaults.json
│   └── corridors/kemuning-pj.json
├── data/                      git-ignored: raw/, interim/
├── testdata/                  tiny hand-made graphs + OSM snippets for tests
├── scripts/fetch-osm.sh       download + osmium clip
├── web/                       Vite + TS + MapLibre static site
└── docs/                      PRD, ARCHITECTURE(-ESSENTIALS), REVIEW, decisions/
```

**Dependency rule:** `flow` depends on nothing in the repo. `graph` depends on `osm` and `config`. `fixes` depends on `flow` and `graph`. `lanedrop` depends on `graph` and `config`. `ctm` depends on nothing in the repo (pure, like `flow`). `corridor` depends on `graph`, `config` and `ctm`. `export` depends on `graph`, `flow`, `fixes`, `lanedrop` (and `corridor`, `ctm` in Phase 1.5). `cmd/*` wire things together. No package imports `cmd`.

## 6. Testing

| Layer | How |
|---|---|
| `flow` | Table tests on hand-made graphs with known max flow (CLRS example, parallel paths, single pinch). Property test: max flow == min cut capacity |
| `osm` tag parsing | Table tests for every lanes/oneway combination |
| `graph` simplify | Test that a 2-lane pinch inside a 4-lane chain survives merging |
| End-to-end | `testdata/mini.osm.pbf` (a few junctions) → golden `edges.geojson` |
| `ctm` (Phase 1.5) | Table tests: vehicles conserved; queue forms at a known bottleneck and spills back at wave speed w; discharge = Q × (1 − drop) once queued; ALINEA holds downstream density near target; red signal passes 0 |
| Simulation gate (Phase 1.5) | Baseline reproduces both known jams and travel times within ≤ 20 % (§3.6); fixes are not exported otherwise |
| Validation | `docs/validation.md` — known jam spots vs model output (manual, before each release) |
| Cross-check | Optional notebook: same graph in NetworkX gives the same max flow |

## 7. Deployment
- `make all` → `web/dist/` → push to `main` → Cloudflare Pages / Vercel builds `web/`.
- Generated data **is committed** under `web/public/data/` (small, lets the host build without Go). Raw OSM is never committed.

## 8. Licensing
- Code: MIT or Apache-2.0 (open question).
- Output data is derived from OSM → **ODbL**. Site must show "© OpenStreetMap contributors" and link to the licence.

## 9. Phase 2+ (sketch only — do not build yet)
- **Junction capacity:** split each signalised node into in/out nodes with an internal capacity arc.
- **API:** small Go `net/http` server: `POST /scenarios` with edge changes → returns new flow. Graph held in memory; stateless.
- **Phase 1.5 is in §3.6**, not here: it stays static and fixed-route.
- **Route choice (Phase 3):** Frank–Wolfe user equilibrium with BPR travel-time functions; needs an OD demand estimate.
- **Layers (Phase 4):** GTFS for Rapid KL; crowdsourced bus stops go to OSM, not our repo.
