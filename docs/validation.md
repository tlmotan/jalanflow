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

## Phase 1.5 simulation gate (CTM baseline)

Decision 0007, PRD goal 8, ARCHITECTURE §3.6. **No fix is published in `scenarios.json` until every row below passes.** Until then the file carries `validated: false` and an empty fix list. The measured inputs live in `config/observed/kemuning-pj.json` (with `source` and date read); this table says what has to match. Status: **not run yet**, no simulator output exists.

How "reproduces a jam" is judged (proposed; confirm before the first run, and put the numbers in config with a `source` once a test checks them):
- **Queue head**: the most downstream congested cell (speed below the critical speed of its class) is within one cell (≤ 500 m) of the target point.
- **Queue tail**: reaches at least the observed tail point at the observed time ± 15 min.
- **Not present off-peak**: the same queue does not form with the off-peak demand profile, so the jam comes from demand, not a broken cell.

### Jams the baseline must reproduce

| # | Jam | Direction / time | Queue head (bottleneck) | Queue tail (observed) | Source | Result |
|---|---|---|---|---|---|---|
| J1 | Sri Muda queue on KESAS E5 eastbound | Inbound, weekday AM peak | Lane drop **#21**, node 33987957, `3.04031,101.55728`: KESAS 3 + exit-507 ramp 2 → 3 (`drop:node:33987957`) | At or behind Tim's Sri Muda pin, `3.0245507704006007,101.54281308204921` (≈1.6 km upstream); time of day still to record | Tim, 2026-11-23: "slow from Sri Muda until just after exit 507" | TODO |
| J2 | Exit-510 Kewajipan ramp into Subang | Inbound, weekday AM peak | Ramp narrowing 3 → 2, `way:148018437>1116586907`, Tim's pin `3.063249022948336,101.59109636839865` | Back along the 3-lane exit and onto KESAS E5 eastbound mainline (diverge spillback); extent still to record | Tim, Street View 2026-11-22: ramp 2 lanes, heavily congested | TODO |

### Travel times (≤ 20 % error)

Error = |simulated − observed| ÷ observed, where observed is the midpoint of Google Maps' "typical" range for that departure time (Directions → Depart at, weekday). Every row must be ≤ 20 %. Read each value manually and record the date read in `config/observed/kemuning-pj.json`.

| # | Section | Direction | Depart | Google Maps typical (min) | Date read | Simulated (min) | Error | Result |
|---|---|---|---|---|---|---|---|---|
| T1 | Kota Kemuning gate (KESAS exit 506B) → PJ gate (LDP exit 1108), whole commute | Inbound | Tue 07:30 | TODO | | | | TODO |
| T2 | KESAS E5: Sri Muda → just after exit 507 (through J1) | Inbound | Tue 07:30 | TODO | | | | TODO |
| T3 | KESAS E5: exit 507 → exit 510 (through J2 diverge) | Inbound | Tue 07:30 | TODO | | | | TODO |
| T4 | PJ gate → Kota Kemuning gate, whole commute | Outbound | Tue 18:00 | TODO | | | | TODO |
| T5 | Any section, off-peak sanity check (should be ≈ free flow) | Inbound | Tue 11:00 | TODO | | | | TODO |

### Open items
- `config/observed/kemuning-pj.json` is still a placeholder. Its J1 queue now has head = drop #21 (`3.04031,101.55728`) and tail = Tim's Sri Muda pin (fixed 2026-11-23; it had them swapped).
- Record J1/J2 tail positions and times (where you first meet the queue, what time).
- Demand inputs (JKR/HPU, LLM) must be filled before a run; a gate pass on placeholder demand does not count.

**Phase 1.5 first real-graph run (2026-11-23, `internal/corridor`, placeholder demand):** inbound route Kota Kemuning → PJ = 56 edges, 19.1 km, 123 mainline cells, free-flow 11.5 min; 16 on-ramps and 14 off-ramps touch it, of which only 2 have (placeholder) numbers. With every unknown off-ramp at 0, all traffic is forced through the 1-lane KESAS → LDP ramp (way 31657409, `3.0563,101.6155`) and the queue grows without limit, so the output is not a finding yet. One OSM gap: way 697934435 (72 m of motorway at `3.05917,101.60778`) has no `ref`; it should be tagged `ref=E5`.

**Public traffic figures found by web search (2026-11-23).** Whole-highway totals summed over toll plazas, both directions: useful as a ceiling and for daily-to-peak scaling, not as counts at a point.
- KESAS (E5): 321,493 veh/day FY Jul 2019 (RAM Ratings, https://www.ram.com.my/pressrelease/?prviewid=5254); 310,600 in 2019 and 238,400 in 2020 (MARC). Nothing newer found.
- LDP (E11): 435,948 veh/day FY Mar 2019 (RAM Ratings, https://www.ram.com.my/pressrelease/?prviewid=5255); average weekday tollable traffic 356,000 in FY2022 (LITRAK Annual Report 2022, https://litrak.com.my/wp-content/uploads/2022/07/LITRAK-Annual-Report-2022.pdf).
- RTVM (JKR/HPU) station tables exist for Petaling district (2015, 2017, 2018 editions on Scribd/pdfcoffee) but the station rows could not be read from search results.

**Signal timing on hold (2026-11-23, Tim):** no real timings available. The inbound route is motorway end to end with no signals on it, so the baseline does not need them; signal-timing fixes are not simulated until surveyed timings exist. `config/signals/kemuning-pj.json` stays a placeholder.

**First calibration (2026-11-23, `make calibrate`, decision 0009).** Fitted to Tim's 10 Google Maps typical times (Tuesday, 07:00–09:00) and the Sri Muda queue; 31 figures estimated.
- Fitted: 5,500 veh/h entering at the route start at peak (reached 06:45); section 1 (to 7.56 km, exit 510) on-ramps 80 % of capacity, exits 1.00 × lane share; section 2 (to 10.68 km, Sunway ramp) on-ramps 5 %, exits 1.20 × lane share.
- Travel times: mean error **10.9 %**. Sri Muda → Sunway ramp: within 14 % at all five times (13.8/16.0/17.5/19.1/20.6 min simulated vs 16/18/18/20/20). Sri Muda → exit 510: 11.9/14.1 vs 12/14 at 07:00/07:30, then too slow: 15.7/17.3/18.8 vs 12/14/16 (**31 %, 23 %, 18 % off**).
- Sri Muda queue (0.07–2.50 km at 07:45): reproduced, 100 % queued. It was required by the fit, so this is not an independent check.
- The simulation also grows a second queue behind the merges before exit 510 (4.2–7.6 km by 09:00). Tim's jam spot 2 is at exit 510, but his is on the ramp; unconfirmed whether the mainline queues there.
- **Gate (0007): not passed.** 2 of 10 times are more than 20 % off, and three fitted values sit on the edge of their search range (section 1 ramp load at the top; section 2 at "free-flowing"). Section 2 cannot hold the 4–6 min Google shows between exit 510 and the Sunway ramp: any queue the model makes at the fork grows without limit (FIFO diverge). Nothing past 10.68 km (all of LDP) is calibrated.
- Without the queue requirement the best travel-time match was 10.3 % but with 3,000 veh/h at the route start, four ramps at 2,800 veh/h each and the queue at 1.9–5.5 km: wrong place. Travel times alone do not pin down demand.
- Needed next from Tim: Google Maps times for the whole route to PJ (covers LDP), a 03:00 reading for each route (real free-flow time), and the time of day he meets the Sri Muda queue (07:45 is assumed).

**First fix scenarios (2026-11-23, `go run ./cmd/calibrate -dry -fixes ...`, on the draft baseline that has NOT passed the gate).** 66 fix-at-site combinations on the inbound route, each run alone. Baseline: 42,004 veh·h of delay over 06:00–11:30, mean commute Sri Muda → Sunway ramp 17.4 min, 4,020 vehicles still unserved when the run ends; queues at 09:44 at 0.0–2.5 km and 3.2–7.6 km.

| Fix | Where | Delay saved (veh·h) | Commute change | Queue afterwards (km) |
|---|---|---|---|---|
| +1 lane for 1,567 m | route narrows 5 → 3 lanes at 5.48 km (`way:53562541>1116728174`) | 1,877 (4.5 %) | +0.6 min | 0.0–2.5, 5.2–7.6 |
| +1 lane for 1,421 m | exit-507 / Hicom merge, 2.50 km (`node:33987957`) | 699 | **+6.0 min** | 0.0–7.6 |
| Dividers | same merge | 664 | +2.9 min | 0.0–7.6 |
| Ramp meter | on-ramp at 7.05 km (`node:2955482360`) | 242 | −1.0 min | 0.0–2.5, 4.1–7.6 |
| Re-stripe 569 m | exit-507 / Hicom merge | 237 | +1.1 min | 0.0–3.1, 3.2–7.6 |
| Ramp meter | exit-507 / Hicom merge | 225 | −0.2 min | 0.0–2.5, 3.2–7.6 |

- No single fix removes more than 4.5 % of the delay. In the model the corridor is overloaded end to end: fixing the exit-507 merge lets more cars through to the 5 → 3 narrowing at 5.48 km, so the queue joins up from 0 to 7.6 km and the commute gets longer. "The jam just moves" is the main result so far.
- Treat the sizes as unreliable: the baseline failed the gate, on-ramp load in section 1 sits at the top of its search range (80 %), and demand is estimated. The ordering of fixes may still change.
- 55 combinations changed delay by under 1 veh·h, including everything past 8 km (free-flowing in the model, and uncalibrated past 10.68 km).
- Found by this run: decision 0008's "zipper 1:1" merge share for dividers was wrong (it cost 5,900–9,300 veh·h by starving the main road); dividers now only reduce the capacity drop.
- Not built yet: signal-timing fixes (on hold), `scenarios.json` and the Solutions panel.
