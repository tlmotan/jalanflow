# 0001 — Phase 1 is a static site with no server

- Status: accepted · 2026-11-21
- Context: Results only change when the OSM snapshot or config changes.
- Decision: Precompute everything in Go; ship JSON to a static host. No API, no DB.
- Consequence: Users can't run their own "what if" until Phase 2 adds a small Go API.
