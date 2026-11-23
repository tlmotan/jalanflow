# Validation — known jam spots vs model

Launch gate (PRD §7): the model's top 3 bottlenecks should match places locals know jam.
Corridor: Kota Kemuning ↔ PJ via KESAS + LDP. Candidate from the 2026-11-22 check: the 1-lane KESAS→LDP ramp at Sunway (exit 511A, OSM way 31657409) — confirm it jams in practice.

| # | Known jam spot (Tim's list) | Direction / time | Model flags it? | Notes |
|---|---|---|---|---|
| 1 | Sri Muda / Kota Kemuning entry onto KESAS eastbound (3.02405,101.54220): Kemuning Bayu, Kota Kemuning and Sri Muda traffic all joining | Inbound, peak | Partly: 2-lane on-ramp (way 24994898) + KESAS 3 lanes squeeze into 4 lanes (way 142141200), then 4 lanes on way 162530515 (untagged in OSM, model guessed 3; Tim confirmed 4 on Street View 2026-11-22) | Real squeeze is 5 → 4 lanes. Way 162530515 needs `lanes=4` added in OSM, then re-fetch. The rest of the jam is demand, which max flow does not model |
| 2 | KESAS eastbound exit 510 Kewajipan, turning left into Subang (3.06322,101.58990): "three lanes merged into one" | Inbound, peak | Partly: OSM has 3 exit lanes on KESAS, ramp 3 → 2 lanes (way 148018437 → 1116586907), then Persiaran Kewajipan north narrows to 2 (Subang–Kelana Link 2 lanes) | Tim confirmed on Street View 2026-11-22: ramp is 2 lanes (OSM correct) and heavily congested. The squeeze is 3 exit lanes → 2-lane ramp, then merging with Kewajipan's own northbound traffic |
| 3 | TODO | | | |
| 4 | TODO | | | |
| 5 | TODO | | | |

**Tim's exact pins (2026-11-22), the two bottlenecks he hits personally**, snapped to the graph:
- Spot 1, Sri Muda: `3.0245507704006007,101.54281308204921` → 1 m from edge e692, the 2-lane on-ramp (ways 24994898 + 1303695731, 4,000 veh/h) joining KESAS E5 eastbound (3 lanes, 6,000 veh/h).
- Spot 2, left into Subang: `3.063249022948336,101.59109636839865` → 2 m from edge e2061, the exit-510 ramp (way 148018437 3 lanes → way 1116586907 2 lanes, 4,000 veh/h) off KESAS E5 eastbound.
Neither is the whole-network min cut (that is KESAS at the source gate, 6,000 veh/h). Both are **lane drops** (5 → 4 and 3 → 2), where more lanes feed in than carry on.

**Lane-drop check (2026-11-23, `make analyze`, 88 drops on E5/E11 and ramps):** Subang exit-510 ramp = **#31** (`way:148018437>1116586907`, 3 → 2); Sri Muda = **#69** (`node:271726619`, 5 → 4). Both are marked confirmed in `config/corridors/kemuning-pj.json`. Ranks are low because the ranking counts lanes, not traffic. Ranks #1–2 rest on untagged ramp lanes and should be checked on Street View. #44 (`node:1743953924`, 4 → 3 at `3.02895,101.54961`) is the fake drop from untagged way 162530515 (really 4 lanes).

**OSM fix (2026-11-23):** Tim tagged way 162530515 (KESAS E5 eastbound bridge, `3.02964,101.55032`) `lanes=5` in OSM (changeset 192350467, source: survey). Re-fetched and rebuilt: the fake 4 → 3 drop at `3.02895,101.54961` is gone (88 → 87 drops). Sri Muda stays a drop, now **#68** (2 + 3 → 4 on way 142141200, tagged 4, `3.02746,101.54784`, then widens to 5). Open: is way 142141200, just after the merge, really 4 lanes? If it is 5, the Sri Muda merge loses no lanes and the jam there is demand, not a lane drop.

**Sri Muda is not a lane drop (2026-11-23):** Tim confirmed KESAS is 5 lanes right after the merge too (way 142141200, tagged 4 in OSM; needs `lanes=5`). So the merge is 2 + 3 → 5 and loses nothing. Its "confirmed" mark is removed. Likely cause of the queue: drop **#21** about 1.6 km ahead (`3.04031,101.55728`, node 33987957), where exit-507 traffic rejoins: KESAS 3 + ramp 2 → 3 (tagged). Ask Tim whether the Sri Muda queue reaches back from there. #18 (`3.03250,101.55294`, same shape, guessed lanes) also needs a look.

**Sri Muda queue traced (2026-11-23):** Tim: eastbound traffic stays slow from Sri Muda until just after exit 507. So drop **#21** (node 33987957, `3.04031,101.55728`, KESAS 3 + exit-507 ramp 2 → 3) is marked confirmed as the cause of jam spot 1. The pin stays at Tim's Sri Muda point, where he meets the queue.
