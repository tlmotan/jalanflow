# AGENTS.md — how to work in this repo

This is the canonical agent instruction file. `CLAUDE.md` imports it.

## Read first
1. `docs/ARCHITECTURE-ESSENTIALS.md` — always, before any change.
2. `docs/PRD.md` §4 (goals / non-goals) — before adding any feature.
3. `docs/ARCHITECTURE.md` — only the section relevant to your task.
4. The design guides and `WORKFLOW.md` in `docs/design/` (see "UI and design" below) and `docs/CONTENT.md` — before any change under `web/`.

## Project in one line
Go pipeline turns an OpenStreetMap clip of one Klang Valley corridor into a directed flow network, finds the min-cut bottleneck with Dinic's, ranks "+1 lane" fixes, and writes static JSON for a MapLibre site.

## Commands
```bash
make fetch      # Overpass download → data/raw/<corridor>.osm.pbf (needs osmium-tool, jq)
make graph      # cmd/build-graph → data/interim/kemuning-pj.graph.json
make analyze    # cmd/analyze → web/public/data/kemuning-pj/
make test       # go vet ./... && go test ./...
make web        # cd web && npm ci && npm run build
make all        # everything above, in order
make shots      # screenshots of the user flow → screenshots/flow (starts the dev server if needed)
make hooks      # once per clone: pre-commit runs make test / npm test + build for staged Go / web files
cd web && npm run dev   # local site
```

## Rules
- **Phase 1 and Phase 1.5 scope only.** Phase 1.5 = demand + CTM simulator + tested fixes on the same corridor (PRD goals 7–9, decisions 0007–0008): fixed routes, static output, no fix published before the validation gate passes. If a task needs anything in the PRD non-goals (API server, database, route choice or re-routing, removing roads, buses/rail, more corridors, real-time data), stop and ask.
- **`internal/ctm` stays pure** like `flow`: no I/O, no logging, no internal imports. `corridor` may import only `graph`, `config`, `ctm`.
- **No new dependencies** without asking. Go: standard library + `paulmach/osm` only. Web: `maplibre-gl`, `vite`, `typescript` only. `screenshots/` (a dev tool, never shipped): `playwright` only, pinned.
- **`internal/flow` stays pure**: no I/O, no logging, no imports from other internal packages.
- **Respect the package dependency rule** in ARCHITECTURE-ESSENTIALS.md.
- **No magic numbers** for capacity, speeds or factors — they go in `config/capacity-defaults.json` with a `source` field. Phase 1.5 demand, split ratios, signal timings and fix parameters go in JSON under `config/`, also with a `source` field (and year for counts).
- **Merging edges keeps the minimum capacity.** Never average.
- **Output format changes**: update the Go struct, `web/src/types.ts` and ARCHITECTURE.md §4 in the same change.
- **Never commit** `data/raw/` or `data/interim/`. Do commit `web/public/data/`.
- Keep OSM attribution on the site.

## UI and design
Before implementing or modifying any user interface:

1. Always read `docs/design/DESIGN.md` (shared design foundation).
2. For desktop interfaces, read `docs/design/DESKTOP.md`.
3. For mobile interfaces, read `docs/design/MOBILE.md`.
4. For responsive web applications, apply both platform guides where appropriate. The JalanFlow site is one.
5. Follow `docs/design/project/DESIGN_BRIEF.md` for project-specific design requirements. Where it is more specific than the shared guides, it wins. Wording follows `docs/CONTENT.md`.
6. Follow `docs/design/WORKFLOW.md` for the order of work: design on the canvas (`docs/design/project/design.fig`), human review, then implement, then verify in the browser. **No change under `web/` for a new design before Tim approves it in chat.** How to drive the canvas is in `docs/design/integrations/OPENPENCIL.md`.

- Reuse existing design tokens and components. Preserve application functionality and data integrity when making visual changes.
- Evaluate the implementation against the relevant design acceptance checklists (`DESKTOP.md` §7, `MOBILE.md` §9, `DESIGN_BRIEF.md` §12) before considering the task complete.
- Do not introduce unnecessary UI elements, dependencies, or abstractions.

## Code style
- Go: `gofmt`, `go vet` clean. Small packages, exported types documented. Errors wrapped with `fmt.Errorf("...: %w", err)`. No panics outside `main`.
- Tests: table-driven, next to the code (`_test.go`). Every bug fix gets a test that failed before the fix.
- TypeScript: `strict: true`. No `any`. Plain DOM, no framework.

## Definition of done
- `make test` passes and `cd web && npm test && npm run build` passes (CI runs both on every push/PR and must be green).
- Changed the pipeline or capacity config? Re-run `make graph analyze` and commit `web/public/data/`; CI checks it is not stale.
- New logic has tests.
- If behaviour or a decision changed, the relevant doc is updated (and ARCHITECTURE-ESSENTIALS.md if it's a critical decision). Record bigger decisions as `docs/decisions/NNNN-title.md`.

## When unsure
Prefer the simpler option, write down the assumption in the code comment and the PR description, and ask.
