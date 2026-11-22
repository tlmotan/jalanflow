# 0002 — Implement Dinic's max flow ourselves

- Status: accepted · 2026-11-21
- Context: No well-maintained Go max-flow library; graph is small; resume goal.
- Decision: ~150-line Dinic's in `internal/flow`, pure, table-tested, cross-checked against NetworkX.
- Consequence: We own correctness; tests must assert max flow == min cut capacity.
