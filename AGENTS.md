# AGENTS.md — how to work in this repo

This is the canonical agent instruction file. `CLAUDE.md` imports it.

## Read first
1. `docs/ARCHITECTURE-ESSENTIALS.md` — always, before any change.
2. `docs/PRD.md` §4 (goals / non-goals) — before adding any feature.
3. `docs/ARCHITECTURE.md` — only the section relevant to your task.

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
make hooks      # once per clone: pre-commit runs make test / npm run build for staged Go / web files
cd web && npm run dev   # local site
```

## Rules
- **Phase 1 scope only.** If a task needs anything in the PRD non-goals (API server, database, route choice, removing roads, buses/rail, more corridors), stop and ask.
- **No new dependencies** without asking. Go: standard library + `paulmach/osm` only. Web: `maplibre-gl`, `vite`, `typescript` only.
- **`internal/flow` stays pure**: no I/O, no logging, no imports from other internal packages.
- **Respect the package dependency rule** in ARCHITECTURE-ESSENTIALS.md.
- **No magic numbers** for capacity, speeds or factors — they go in `config/capacity-defaults.json` with a `source` field.
- **Merging edges keeps the minimum capacity.** Never average.
- **Output format changes**: update the Go struct, `web/src/types.ts` and ARCHITECTURE.md §4 in the same change.
- **Never commit** `data/raw/` or `data/interim/`. Do commit `web/public/data/`.
- Keep OSM attribution on the site.

## Code style
- Go: `gofmt`, `go vet` clean. Small packages, exported types documented. Errors wrapped with `fmt.Errorf("...: %w", err)`. No panics outside `main`.
- Tests: table-driven, next to the code (`_test.go`). Every bug fix gets a test that failed before the fix.
- TypeScript: `strict: true`. No `any`. Plain DOM, no framework.

## Definition of done
- `make test` passes and `cd web && npm run build` passes.
- New logic has tests.
- If behaviour or a decision changed, the relevant doc is updated (and ARCHITECTURE-ESSENTIALS.md if it's a critical decision). Record bigger decisions as `docs/decisions/NNNN-title.md`.

## When unsure
Prefer the simpler option, write down the assumption in the code comment and the PR description, and ask.
