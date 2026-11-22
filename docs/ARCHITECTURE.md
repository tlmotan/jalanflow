# ARCHITECTURE — JalanFlow

> Status: v0.2 (revised after REVIEW.md) · Read ARCHITECTURE-ESSENTIALS.md first if you are an agent.

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
                                     static host (Cloudflare Pages / Vercel)
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
| Hosting | Cloudflare Pages or Vercel (static) | Free tier, no server |
| CI | GitHub Actions: `go test`, `go vet`, `npm run build` | Keeps main green |
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
- Run per **direction** (`inbound`: Kota Kemuning → PJ; `outbound`: reverse) by swapping gate roles.
- Max flow therefore = the most vehicles/hour the network can move between the gate sets. The min cut = the set of road segments that together cap that number.

### 3.4 Algorithms
| Step | Algorithm | Complexity | Notes |
|---|---|---|---|
| Max flow | Dinic's | O(V²E), fast in practice | Integer capacities (veh/h) |
| Min cut | BFS on residual graph from source | O(V+E) | Cut = arcs from reachable to unreachable |
| Bottleneck score | `flow / capacity` per edge, plus `is_cut` | O(E) | Saturated non-cut edges shown amber |
| Fix ranking | For each cut edge: +1 lane, re-run Dinic, record gain + new cut | O(k · Dinic) | k = cut size, small |

Min cuts are **not unique**. We report the cut nearest the source (from the residual BFS) and say so. Phase 2 may also compute the cut nearest the sink.

### 3.5 Fix difficulty (Phase 1 heuristic)
| Signal (from OSM) | Effect |
|---|---|
| `bridge=yes` or `tunnel=yes` | hard |
| `toll=yes` or known concessionaire `operator` | medium+, reason "needs concessionaire agreement" |
| `highway` = motorway/trunk with no above | medium |
| primary and below with no above | easy (re-stripe / junction change) |

Reasons are stored as a list of strings and shown to the user verbatim.

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

type FlowResult struct {
    Direction  Direction
    MaxFlowVPH int
    EdgeFlow   []int    // indexed by EdgeID
    CutEdges   []EdgeID
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
}
```

### 4.3b Lane drops (`internal/lanedrop`) — the site headline (decision 0004)

```go
type Drop struct {
    Rank          int     // 1 = biggest squeeze
    Key           string  // "node:<osm node>" (merge) | "way:<from>><to>" (narrowing); stable across rebuilds
    Kind          Kind    // "merge" | "narrowing"
    Pos           LatLon
    LanesIn, LanesOut, LanesLost int
    Squeeze       float64 // LanesOut / LanesIn
    From, To      []Road  // {osm_way_id, name, ref, class, lanes, lanes_inferred}
    LanesInferred bool    // any lane count here is a class default
    Confirmed     *Confirmation // {name, source} or null
}
```
- **Merge:** node without signals, all roads in `Classes`, one on `RouteRefs`, Σ lanes in > Σ lanes out. Lanes are taken from the part of each edge that touches the node, not the edge minimum.
- **Narrowing:** consecutive `Edge.Parts` where lanes fall; edge on the route or a ramp touching it. Position = `Parts[k-1].End`.
- Skip within `TollPlazaRadiusM` of a toll booth. Order: squeeze asc, lanes lost desc, key.
- Lanes only: no demand, so rank ≠ how bad the jam is. Said on the site.

### 4.4 Output files (`web/public/data/<corridor>/`)

**`edges.geojson`** — FeatureCollection, one LineString per edge **per direction analysed**:
```json
{ "type": "Feature",
  "geometry": { "type": "LineString", "coordinates": [[101.5, 3.07], ...] },
  "properties": {
    "id": 42, "dir": "inbound", "name": "Lebuhraya Shah Alam", "ref": "E5",
    "class": "trunk", "lanes": 3, "lanes_inferred": false,
    "capacity_vph": 5400, "flow_vph": 5400, "utilisation": 1.0, "is_cut": true
  } }
```

**`fixes.json`** — `{ "inbound": Fix[], "outbound": Fix[] }`, sorted by `GainVPH` desc.

**`lane_drops.json`** — `{ corridor, osm_date, generated_at, ranking, assumptions[], drops: Drop[] }` (`export.LaneDropsFile`), drops in rank order.

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

**Dependency rule:** `flow` depends on nothing in the repo. `graph` depends on `osm` and `config`. `fixes` depends on `flow` and `graph`. `lanedrop` depends on `graph` and `config`. `export` depends on `graph`, `flow`, `fixes`, `lanedrop`. `cmd/*` wire things together. No package imports `cmd`.

## 6. Testing

| Layer | How |
|---|---|
| `flow` | Table tests on hand-made graphs with known max flow (CLRS example, parallel paths, single pinch). Property test: max flow == min cut capacity |
| `osm` tag parsing | Table tests for every lanes/oneway combination |
| `graph` simplify | Test that a 2-lane pinch inside a 4-lane chain survives merging |
| End-to-end | `testdata/mini.osm.pbf` (a few junctions) → golden `edges.geojson` |
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
- **Route choice (Phase 3):** Frank–Wolfe user equilibrium with BPR travel-time functions; needs an OD demand estimate.
- **Layers (Phase 4):** GTFS for Rapid KL; crowdsourced bus stops go to OSM, not our repo.
