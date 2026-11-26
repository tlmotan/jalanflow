# 0011 — Animations show only what the model computed

- Status: accepted · 2026-11-23
- Context: We want visitors to *watch* the bottleneck being found and *feel* it, via a "Run the analysis" button and moving dots. Max flow is a steady state: every road carries a constant flow and nothing queues. Drawing cars piling up behind the bottleneck would show something the model never computed, which a journalist could rightly call out.
- Decision:
  - The replay draws Dinic's **real augmenting paths** in the order the algorithm found them (`flow.MaxFlowTrace` → `replay.json`). Roads fill as flow is pushed; the min cut is highlighted when no route has room left.
  - The dots show **max-flow capacity**: density follows each road's flow, labelled "how much the roads can carry, not live traffic". No queues, no slow-downs.
  - Queues and spillback are drawn only from the Phase 1.5 CTM simulation, which computes them, and only after its validation gate passes.
  - Button only, no autoplay; Skip always available; reduced motion jumps to the end.
  - Update 2026-11-25 (Tim): the running panel shows progress as one segment per real step and says each step's own amount in a sentence, exact rather than rounded while it runs. A step with a negative edge delta is described as moving part of an earlier route. Nothing is interpolated: no percentage, no counter values between steps, no fill inside a segment.
- Consequences: `replay.json` is a new output file (ARCHITECTURE §4.4). The replay is as long as the real trace; the frontend scales its speed to finish in ≤ 15 s.
