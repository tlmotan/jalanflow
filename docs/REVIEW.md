# REVIEW — what will break, what's over-engineered

> Step 4 review of the v0.1 plan (Go pipeline + engine + API + frontend, weekly refresh, cost model with 6 feasibility factors). Each item says what changed in PRD / ARCHITECTURE v0.2.

## What will break

| # | Problem | Why it breaks | Fix applied |
|---|---|---|---|
| 1 | **"Lane count = capacity"** | A highway lane carries roughly twice a signalised arterial lane. Raw lane counts make every arterial look as good as the highway | Capacity = lanes × per-lane veh/h by class × signal factor, in config with sources |
| 2 | **OSM `lanes` is total, both directions** | A two-way 4-lane road becomes 4 lanes each way → double capacity | Lanes per direction: `lanes:forward/backward` → `lanes` if oneway → `lanes/2` |
| 3 | **Missing lane tags** | Many Malaysian ways have no `lanes` tag; silent defaults produce fake bottlenecks | Infer by class **and flag it** on the map |
| 4 | **Max flow needs sources and sinks** | v0.1 never said where flow starts and ends. Without that, there is no answer | Named gates in corridor config + super-source/sink, per direction |
| 5 | **Capacity ≠ congestion** | Max flow says where the road *can carry* least, not where it's *busiest*. With no demand data, calling it "traffic" is misleading | Site wording: "capacity bottleneck". Validation gate against known jam spots before launch |
| 6 | **Junctions are often the real bottleneck** | Nodes have unlimited capacity in plain max flow; signals and roundabouts aren't modelled | Disclosed limitation in Phase 1; node splitting in Phase 2 |
| 7 | **Segment merging can erase pinches** | Averaging or taking the first segment's lanes hides the short 2-lane merge — the exact thing we're looking for | Merged edge keeps **min** capacity; dedicated test |
| 8 | **Min cut isn't unique** | Two runs or two tools can show different "bottlenecks" with equal capacity; looks buggy | Always report source-side cut; documented |
| 9 | **"Remove a road" is impossible in max flow** | Removing an edge can never raise max flow. The v0.1 goal promised this | Moved to Phase 3 (user equilibrium / Braess) |
| 10 | **Bounding box leaks** | Roads crossing the bbox edge become dead ends or create fake short-cuts | Only flow between gates counts; gates placed inside bbox; visual check step |
| 11 | **"Land ownership" in Phase 1 feasibility** | There is no open dataset for it in OSM. It can't be modelled from data | Dropped from Phase 1; difficulty uses only OSM tags (toll, operator, bridge, tunnel, class) |
| 12 | **Credibility** | One obviously wrong red road and the project is dismissed | Validation table (`docs/validation.md`) is a launch gate |
| 13 | **ODbL** | Publishing derived OSM data without attribution breaks the licence | Attribution + licence notice is a Must requirement |

## What's over-engineered

| # | v0.1 item | Why it's too much now | Change |
|---|---|---|---|
| A | **Go API server** | Results are fixed per OSM snapshot; nothing needs computing on request | Removed from Phase 1. Static JSON only |
| B | **Weekly refresh job** | Roads on one corridor barely change weekly; adds cron, hosting, failure modes | Manual `make all` when needed; manifest records OSM date |
| C | **Min-cost capacity expansion (full optimisation)** | That's a mixed-integer program; overkill when the cut has a handful of edges | Greedy "+1 lane per cut edge, re-run" — exact enough, explainable, cheap |
| D | **Six feasibility factors** | Four have no data source | Three tag-based signals; others listed as "not modelled" |
| E | **Whole-Malaysia download in the pipeline** | Slow, big, unnecessary | `osmium extract` to bbox once; pipeline reads the small clip |
| F | **Framework frontend (React etc.)** | One map, one panel, one page | Plain TypeScript + MapLibre; add a framework only if UI grows |
| G | **Python in the pipeline** | Two runtimes to maintain | Python only in optional cross-check notebooks |

## Kept on purpose
- **Own Dinic's implementation** — small, testable, and the strongest Go interview talking point.
- **Two commands (build-graph / analyze)** — cheap split that makes iteration fast.
- **Committed output data** — lets the static host build without Go.
