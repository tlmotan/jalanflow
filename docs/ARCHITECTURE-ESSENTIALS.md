# ARCHITECTURE ESSENTIALS

> The 2-minute version. Full detail: ARCHITECTURE.md. If this file and ARCHITECTURE.md disagree, ARCHITECTURE.md wins — and fix this file.

## What we're building
Static website whose headline is **lane drops** on the route (merges and narrowings where more lanes come in than carry on, biggest squeeze first, locally confirmed ones marked), plus **capacity bottlenecks (min cut)** on one Klang Valley corridor (Kota Kemuning ↔ Petaling Jaya via KESAS then LDP), plus a ranked list of **"+1 lane" fixes** with capacity gain and difficulty.

## Shape (Phase 1)
```
Overpass (bbox, road classes) → osmium cat → .pbf → cmd/build-graph → graph.json → cmd/analyze → web/public/data/*.json → static MapLibre site
```
**No server. No database. No Docker. No scheduled jobs.**

## Stack
- Go 1.23+ engine (`cmd/`, `internal/`), only external dep: `paulmach/osm`
- Vite + TypeScript + MapLibre GL JS frontend (`web/`)
- JSON config, GeoJSON output, static hosting

## Critical decisions
1. **Directed graph.** One arc per allowed direction. Lanes are **per direction** (`lanes:forward/backward` → `lanes` if oneway → `lanes/2` → class default).
2. **Capacity** = lanes × per-lane veh/h by class × signal factor. Numbers live in `config/capacity-defaults.json`, never hard-coded.
3. **Inferred lanes are flagged** (`lanes_inferred`) and shown on the map.
4. **Merging segments keeps the MIN capacity.** Never average.
5. **Gates** (named points in corridor config) → super-source / super-sink. Each direction analysed separately.
6. **Dinic's max flow**, own implementation in `internal/flow`, pure functions, no I/O.
7. **Min cut** = residual BFS from source. Not unique — we report the source-side cut.
8. **Fix ranking** = for each cut edge, +1 lane, re-run, record gain and the next cut.
9. **Junctions are unlimited capacity** in Phase 1 (known limitation, disclosed).
10. **Go structs are the source of truth** for output formats; `web/src/types.ts` mirrors them in the same commit.
11. **Lane drops count lanes, not traffic.** Signalised junctions and points near toll booths are skipped; confirmed spots are keyed by OSM ids (decision 0004).

## Package rules
`flow` ← imports nothing internal · `graph` ← `osm`, `config` · `fixes` ← `flow`, `graph` · `lanedrop` ← `graph`, `config` · `export` ← all above · `cmd/*` wires only.

## Out of scope (do not build)
Removing roads, route choice, real-time traffic, user what-if editing, API server, buses/rail/walking, other corridors, land ownership data.

## Key files
- `config/corridors/kemuning-pj.json` — bbox + gates
- `config/capacity-defaults.json` — per-lane capacities (with sources)
- `internal/graph/model.go` — data model
- `internal/flow/dinic.go` — algorithm
- `web/src/types.ts` — frontend mirror of outputs
