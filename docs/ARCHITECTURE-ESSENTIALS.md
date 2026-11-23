# ARCHITECTURE ESSENTIALS

> The 2-minute version. Full detail: ARCHITECTURE.md. If this file and ARCHITECTURE.md disagree, ARCHITECTURE.md wins — and fix this file.

## What we're building
Static website whose headline is **lane drops** on the route (merges and narrowings where more lanes come in than carry on, biggest squeeze first, locally confirmed ones marked), plus **capacity bottlenecks (min cut)** on one Klang Valley corridor (Kota Kemuning ↔ Petaling Jaya via KESAS then LDP), plus a ranked list of **"+1 lane" fixes** with capacity gain and difficulty.

## Shape (Phase 1)
```
Overpass (bbox, road classes) → osmium cat → .pbf → cmd/build-graph → graph.json → cmd/analyze → web/public/data/*.json → static MapLibre site
```
**No server. No database. No Docker. No scheduled jobs.**

## Phase 1.5 (planned, decisions 0007–0008)
Same pipeline and static site. `cmd/analyze` also builds a CTM cell chain of the corridor (`internal/corridor`), simulates the peak with demand (`internal/ctm`), tests fixes and writes `scenarios.json`. Phase 1 outputs are unchanged.

## Stack
- Go 1.23+ engine (`cmd/`, `internal/`), only external dep: `paulmach/osm`
- Vite + TypeScript + MapLibre GL JS frontend (`web/`)
- JSON config, GeoJSON output, static hosting

## Critical decisions
1. **Directed graph.** One arc per allowed direction. Lanes are **per direction** (`lanes:forward/backward` → `lanes` if oneway → `lanes/2` → class default).
2. **Capacity** = lanes × per-lane veh/h by class × signal factor. Numbers live in `config/capacity-defaults.json`, never hard-coded.
3. **Inferred lanes are flagged** (`lanes_inferred`) and shown on the map.
4. **Merging segments keeps the MIN capacity.** Never average.
5. **Gates** (named points in corridor config) → super-source / super-sink. Each direction analysed separately. Gates snap per direction onto `gate_snap.classes` roads running the right way (`graph.SnapGate`), never to the plain nearest node.
6. **Dinic's max flow**, own implementation in `internal/flow`, pure functions, no I/O.
7. **Min cut** = residual BFS from source. Not unique — we report the source-side cut.
8. **Fix ranking** = for each cut edge, +1 lane on its narrowest part. Only edges in **every** min cut (Picard–Queyranne, `flow.InEveryMinCut`) can gain; those are re-run for gain and next cut, the rest are `helps: false` with a reason (decision 0005).
9. **Junctions are unlimited capacity** in Phase 1 (known limitation, disclosed).
10. **Go structs are the source of truth** for output formats; `web/src/types.ts` mirrors them in the same commit.
11. **Lane drops count lanes, not traffic.** Signalised junctions and points near toll booths are skipped; confirmed spots are keyed by OSM ids (decision 0004).
12. **Simulation = own Daganzo CTM** in `internal/ctm`, pure and deterministic, no SUMO/microsim (decision 0007). Extensions: capacity drop, ALINEA ramp metering, stepped signals. Demand, splits, signal timings and fix parameters live in config with a `source`.
13. **Fixed routes.** Demand enters at on-ramps and leaves at off-ramps by measured split ratios; nobody re-routes. No induced demand; the site says so. Route choice stays Phase 3.
14. **Validation gate.** No fix is published in `scenarios.json` until the baseline reproduces both known jams (Sri Muda queue from drop #21 at exit 507; exit-510 Subang ramp) and peak travel times within ≤ 20 %.
15. **Fixes are ranked by delay saved** (veh-h per peak), each run alone against the baseline, with a confidence label; dividers/channelisation are always low confidence (decision 0008).

## Package rules
`flow` ← imports nothing internal · `ctm` ← imports nothing internal (pure, no I/O, no logging) · `graph` ← `osm`, `config` · `fixes` ← `flow`, `graph` · `lanedrop` ← `graph`, `config` · `corridor` ← `graph`, `config`, `ctm` · `export` ← all above · `cmd/*` wires only.

## Out of scope (do not build)
Removing roads, route choice (incl. re-routing in the simulator), real-time traffic, travel-time prediction outside the simulated corridor, user what-if editing, API server, buses/rail/walking, other corridors, land ownership data.

## Key files
- `config/corridors/kemuning-pj.json` — bbox + gates
- `config/capacity-defaults.json` — per-lane capacities (with sources)
- `internal/graph/model.go` — data model
- `internal/flow/dinic.go` — algorithm
- `internal/ctm/` — cell transmission model (Phase 1.5, planned)
- `web/src/types.ts` — frontend mirror of outputs
