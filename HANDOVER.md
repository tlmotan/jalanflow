# Handover — JalanFlow, end of session 2026-11-22

Read this first, then `AGENTS.md` → `docs/ARCHITECTURE-ESSENTIALS.md`. Delete or overwrite this file at the end of the next session.

## Where things stand

| Stage | State |
|---|---|
| Fetch (`make fetch`) | ✅ Overpass by default, Geofabrik fallback (`OSM_SOURCE=geofabrik`). Writes OSM date into the PBF header |
| OSM reader (`internal/osm/reader.go`) | ✅ Two-pass `.pbf` read, tested with XML fixtures |
| Graph build + merge (`internal/graph`) | ✅ Split at junctions/signals, per-direction lanes, min-capacity merge with `Edge.Parts`. Pinch test checked by mutation |
| `cmd/build-graph` | ✅ Runs on real data |
| Max flow (`internal/flow`) | ✅ Dinic, tested (existed before this session) |
| `cmd/analyze` | ❌ Stub. **Next task** |
| Fix ranking (`internal/fixes`) | ❌ Stub |
| Export (`internal/export`) | ❌ Stub (structs only) |
| Website (`web/`) | ❌ Stub. `npm run build` never run: `web/node_modules` not installed |

`make test` passes (`go vet` + `go test`). **Not a git repo** — nothing is committed; run `git init` before risky changes.

## Corridor (changed this session)

Phase 1 corridor is now **Kota Kemuning ↔ Petaling Jaya via KESAS (E5) then LDP (E11)** — the user's own commute. See `docs/decisions/0003-corridor-kemuning-pj.md`. Config: `config/corridors/kemuning-pj.json`, bbox `[101.50, 2.98, 101.66, 3.14]`. `config/corridors/federal-highway.json` is unused, kept for reference.

Current data: `data/raw/kemuning-pj.osm(.pbf)`, `data/interim/kemuning-pj.graph.json` — 10,558 ways → 6,553 nodes, 9,963 edges, 4,059 with inferred lanes, OSM date 2026-11-22, 99.7% of nodes in one component.

## Findings so far (placeholder capacities — ranks hold, absolute veh/h don't)

- **Whole network, Kota Kemuning → PJ:** max flow 6,000 veh/h; min cut = KESAS itself at the source gate (3 lanes). Drivers can bypass the Sunway ramp via exit 510 Kewajipan → NPE (E10) → LDP.
- **Route only (KESAS → Sunway → LDP):** bottleneck is the **1-lane Sunway ramp** onto LDP, exit 511A, OSM way 31657409, 2,000 veh/h. Maps: `3.0563,101.6155`
- Verified by: Dinic == independent Edmonds-Karp == cut capacity; flow conservation; lanes matched against raw OSM tags.

## User's jam spots (in `docs/validation.md`, confirmed on Street View)

1. **Sri Muda / Kota Kemuning entry onto KESAS eastbound** (`3.02405,101.54220`): 2-lane on-ramp + KESAS 3 lanes → 4 lanes (squeeze 5 → 4). Way **162530515** (`3.02964,101.55032`) has **no lanes tag**; model guesses 3, **user confirmed 4**.
2. **KESAS eastbound exit 510 Kewajipan, left into Subang** (`3.06322,101.58990`): 3 exit lanes → 2-lane ramp (OSM correct, user confirmed 2, heavily congested), then merges with Persiaran Kewajipan northbound and splits into two 2-lane roads.

## Open decision (waiting on the user)

Way 162530515 should be `lanes=4`. Either the user edits OSM (https://www.openstreetmap.org/way/162530515) and re-runs `make fetch graph`, **or** we add a local lane-override config (must carry a `source` field per AGENTS.md; new feature — ask first). The user had not answered when the session ended.

## Next task: build `cmd/analyze`

Must-haves learned this session:
- **Gate snapping must be edge-based and direction-aware.** Nearest *node* fails: merging removes pass-through nodes, so the nearest node to a point on KESAS was on Jalan Bukit Kemuning. Snap to the nearest edge of the right road whose heading matches the direction of travel; source = its `From`, sink = its `To`. Each gate `pos` sits between carriageways (see the corridor config `_note`, ARCHITECTURE §3.3).
- The **outbound PJ gate** currently snaps 923 m off, just before an LDP exit, so the PJ → Kemuning route leaks through SS7 streets. Move/verify it.
- With one gate per side, the whole-network cut is usually the gate road. Decide with the user what the site's headline finding is before building fixes/export. A "route only" analysis is close to the route-choice non-goal — ask before adding it.
- Then: `fixes.Rank` (+1 lane on the bottleneck **part**, re-check the next-narrowest part via `Edge.Parts`), `export` writers, `web/src/types.ts` in the same change as any output struct.

## Environment gotchas

- **cgo:** `paulmach/osm/osmpbf` needs C zlib via pkg-config when cgo is on. The Makefile sets `CGO_ENABLED=0`; use it for direct `go test`/`go run` too.
- **Network:** the agent sandbox's network is effectively dead (~200 B/s). The user's network is slow and drops connections; `make fetch` (Overpass, ~1.3 MB, with retries) works when run **by the user in their own terminal**. Geofabrik (~250 MB) is impractical for them.
- **Overpass:** default curl User-Agent gets HTTP 406; the script sets one. Fallback mirror: `OVERPASS_URL=https://overpass.kumi.systems/api/interpreter`.
- `data/raw/malaysia-singapore-brunei-latest.osm.pbf.part` is a dead partial download — safe to delete. `data/raw/federal-highway.*` and `data/interim/federal-highway.graph.json` are old-corridor data.
- The Python check scripts used this session lived in a temporary scratchpad and are gone. They were: graph health (classes, inferred %, components, dead ends), Dinic + Edmonds-Karp cross-check with direction-aware gates, route-only subgraph flow, and per-spot lane dumps with raw OSM tags. Rewrite as needed; Python stays out of the pipeline (REVIEW.md G).

## How to talk to this user

- Whenever discussing roads, give **copy-paste Google Maps inputs**: `lat,lon` per place, `https://www.google.com/maps/dir/<a>/<b>/<c>` for routes, `https://www.google.com/maps/@?api=1&map_action=pano&viewpoint=<lat,lon>` for Street View. Coordinates from the data, not memory.
- Explain in plain terms (they asked whether we were "simulating roads"): it's road **capacity**, not traffic or travel time.
- They know the corridor by sight; ask them to confirm spots and lane counts on Street View — that is the validation gate.

## Update — later on 2026-11-22: headline = lane drops (user decided)

User chose **lane drops**, ranked **by size (squeeze = lanes out / lanes in, smallest first; tie → more lanes lost)**, with their two spots marked "confirmed by a local driver".
Prototype rule (validated on real data, 93 drops on/next to E5+E11; both spots found):
- **Junction drop**: unsignalised node, all in/out edges in {motorway, motorway_link, trunk, trunk_link}, at least one edge with ref E5/E11, sum(in lanes) > sum(out lanes). Sri Muda = node at `3.02560,101.54500` (2 + 3 → 4).
- **Narrowing**: consecutive `Edge.Parts` with fewer lanes, edge on E5/E11 or a link touching one. Subang = e2061, way 148018437 → 1116586907 (3 → 2) at `3.06336,101.59283`.
- Skip anything within 400 m of a toll booth (plazas widen to 11–15 lanes).
- Fake drop at `3.02895,101.54961` comes from way 162530515 (inferred 3, really 4); override question still open.

**Done (tests pass):** `osm.Node.TollBooth`, `graph.Graph.TollBooths`, `graph.Segment.End` (where each part ends), `graph.HaversineM`, test `TestBuildTollBoothsAndPartEnd`. **graph.json must be rebuilt** (`make graph`, no network needed).
**Update 2026-11-23 — lane drops built:** `internal/lanedrop` (+ table tests), `lane_drops` block in `config/corridors/kemuning-pj.json` (2 confirmed keys), `export.WriteLaneDrops` → `web/public/data/kemuning-pj/lane_drops.json` (88 drops; Subang #31, Sri Muda #69), `cmd/analyze` writes it, `web/src/types.ts`, decision 0004, PRD goal 6, ARCHITECTURE §4.3b, ESSENTIALS #11. `make test` passes. **`npm run build` not run** (`web/node_modules` not installed).
**Update 2026-11-23 — site shows lane drops:** `web/index.html` + `web/src/main.ts` + `style.css`: dots on the map (red = confirmed, hollow = guessed lanes, size = lanes lost, rank labels from zoom 12), side panel with confirmed spots and all 88 ranked, popups with in/out roads and Street View / Google Maps / OSM links, "How this works" from the file's assumptions. Phone layout = bottom sheet. Edges/fixes layers only draw if those files exist. `npm run build` passes; checked in the browser pane (desktop + mobile). Dev server config: `.claude/launch.json` (`npm --prefix web run dev`, port 5173).
**Ramps named (2026-11-23):** `lanedrop.Road.Leaves/Joins` (nearest named road through other ramps, tested); site titles read e.g. "ramp Persiaran Kewajipan → E5 narrows".
**Left:** ask Tim to Street View ranks #1–2 (untagged ramp lanes); way 162530515 override decision; then flow/fixes in `cmd/analyze`. Nothing is committed.
