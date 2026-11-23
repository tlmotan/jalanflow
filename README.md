# JalanFlow (working name)

[![ci](https://github.com/tlmotan/jalanflow/actions/workflows/ci.yml/badge.svg)](https://github.com/tlmotan/jalanflow/actions/workflows/ci.yml) · Live: https://tlmotan.github.io/jalanflow/

Finds the roads that limit traffic flow on Klang Valley corridors, using OpenStreetMap data and max flow / min cut, and ranks cheap fixes. Phase 1: Kota Kemuning ↔ Petaling Jaya via KESAS (E5) and LDP (E11).

- What and why: [docs/PRD.md](docs/PRD.md)
- How: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (short version: [ARCHITECTURE-ESSENTIALS](docs/ARCHITECTURE-ESSENTIALS.md))
- Known pitfalls: [docs/REVIEW.md](docs/REVIEW.md)

## Quick start
```bash
# needs Go 1.23+, Node 20+, osmium-tool, jq
make all
cd web && npm run dev
make hooks   # once: checks Go / web changes before each commit
```

## CI / deploy
Every push and PR runs `.github/workflows/ci.yml` (gofmt, `make test`, web build). A green push to `main` deploys the site to GitHub Pages. CI does not rebuild the data: after changing the pipeline or `config/capacity-defaults.json`, run `make graph analyze` and commit `web/public/data/`, or the committed-data test fails.

Map data © OpenStreetMap contributors, ODbL.
