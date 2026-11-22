# JalanFlow (working name)

Finds the roads that limit traffic flow on Klang Valley corridors, using OpenStreetMap data and max flow / min cut, and ranks cheap fixes. Phase 1: Federal Highway, Shah Alam ↔ KL.

- What and why: [docs/PRD.md](docs/PRD.md)
- How: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (short version: [ARCHITECTURE-ESSENTIALS](docs/ARCHITECTURE-ESSENTIALS.md))
- Known pitfalls: [docs/REVIEW.md](docs/REVIEW.md)

## Quick start
```bash
# needs Go 1.23+, Node 20+, osmium-tool, jq
make all
cd web && npm run dev
```

Map data © OpenStreetMap contributors, ODbL.
