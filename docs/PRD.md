# PRD — JalanFlow (working name)

> Status: v0.4 (Phase 1.5 simulation added, see decisions/0007 and 0008) · Owner: Tim Tan · Last updated: 2026-11-23

## 1. One-liner

A public website that models Klang Valley roads as a flow network and shows, with evidence, **which road segments limit traffic flow and which cheap changes would raise capacity the most**.

## 2. Problem

- Klang Valley traffic is car-dependent and jams are treated as "just how it is".
- Existing open projects **describe** the network (live jams, ridership, walk-time reach). None **prescribe** a change and test it before it is built.
- The public, journalists and councillors have no simple, evidence-backed way to say "this specific merge is the problem, and this fix is worth it".

## 3. Who it's for

| User | What they want | Phase |
|---|---|---|
| **Curious commuter** | "Why does my drive always jam here?" — a map they can understand in 10 seconds | 1 |
| **Journalist / advocate** | A shareable, defensible finding with the method and assumptions visible | 1 |
| **Councillor / MP staff** | A ranked list of fixes with rough difficulty | 2 |
| **Open-source contributor** | Clear code, reproducible pipeline, easy places to add data | 1 |
| **Recruiter / interviewer** (secondary) | A real Go project with clean architecture and tests | 1 |

## 4. Goals and non-goals

### Goals (Phase 1 — one corridor)
1. Find the **capacity bottleneck** (min cut) on one corridor: **Kota Kemuning ↔ Petaling Jaya via KESAS (E5) and LDP (E11)** plus the parallel roads a driver could realistically use.
2. Show it on a public map: every road coloured by how close it is to being the limiting cut.
3. For each bottleneck segment, test **"+1 lane"** and report the capacity gained, so knock-on bottlenecks (the jam just moves) become visible.
4. Label each fix **easy / medium / hard** from data we actually have (road class, toll/concession, bridge/tunnel).
5. Every number on the site traces back to a stated assumption.
6. **Headline: lane drops on the route** (KESAS + LDP and their ramps), biggest squeeze first, with spots confirmed by local drivers marked. Lanes only; the site says it does not know traffic volumes (decision 0004).

### Goals (Phase 1.5 — same corridor, with traffic demand)
Phase 1 stays as shipped. Phase 1.5 adds demand and a simulator on top of it (decisions 0007, 0008).

7. **Simulate the corridor** (KESAS E5 eastbound/westbound + LDP E11 and their ramps) with our own **Cell Transmission Model** (CTM, Daganzo) in Go: demand enters at on-ramps, leaves at off-ramps by measured split ratios, queues form and spill back. Includes capacity drop at merges, ramp metering (ALINEA) and stepped traffic signals. Fixed routes: nobody re-routes.
8. **Calibrate to real data, with a validation gate.** Demand from public counts (JKR/HPU Road Traffic Volume Malaysia, LLM highway statistics); calibrated against Google Maps typical peak travel times and local driver queue observations. **No fix is published** until the baseline reproduces both known jams (the Sri Muda queue caused by lane drop #21 at the exit-507 merge, and the exit-510 Subang ramp) and peak travel times within **≤ 20 %** error.
9. **Rank tested fixes by delay saved** (vehicle-hours per peak), with commute time before/after, where the queue moves, difficulty and confidence. Fix types: re-striping, dividers/channelisation, ramp metering, +1 lane, signal timing (decision 0008). Shown in a Solutions panel with a before/after time–space heat strip.

### Non-goals (Phase 1 and 1.5)
- ❌ Predicting real travel times or real-time jams **anywhere outside the simulated corridor**. Phase 1.5 estimates typical peak travel times on the simulated KESAS + LDP chain only, and only after it passes the validation gate (goal 8). Still no real-time or live data.
- ❌ Drivers changing route in response to a fix (route choice, Phase 3). Phase 1.5 is fixed-route; the site says a fix may attract extra traffic.
- ❌ "Remove a road" recommendations — max flow can never show a removal helping. That needs route choice (Phase 3).
- ❌ Live "what if" editing by users (needs a server; Phase 2).
- ❌ Buses, rail, walkability (later layers).
- ❌ All of Malaysia.
- ❌ Land ownership, utilities, construction disruption (no reliable open data).

## 5. Core user stories (Phase 1)

1. As a commuter, I open the site and immediately see the corridor with the bottleneck segments highlighted red.
2. As a commuter, I tap a red road and see: name, lanes, estimated capacity, whether lanes were guessed, and why it's a bottleneck.
3. As a journalist, I open a ranked "Top fixes" list: segment, change (+1 lane), capacity gain (veh/h), difficulty, and where the bottleneck moves next.
4. As a journalist, I open "How this works" and read the method, assumptions and limits in plain language.
5. As a contributor, I run one command and reproduce every file the site shows.

## 6. Functional requirements

| ID | Requirement | Priority |
|---|---|---|
| F1 | Build a directed road graph for a corridor bounding box from an OpenStreetMap extract | Must |
| F2 | Assign each edge a capacity = lanes per direction × per-lane capacity by road class (config file) | Must |
| F3 | Flag every edge whose lane count was inferred, not tagged | Must |
| F4 | Define sources and sinks per corridor in a config file (named "gates") | Must |
| F5 | Compute max flow and the min cut between gates | Must |
| F6 | For each cut edge, re-run with +1 lane and record capacity gain and the new cut | Must |
| F7 | Tag each fix easy / medium / hard with the reasons shown | Must |
| F8 | Export static GeoJSON + JSON + a manifest (OSM date, assumptions, version) | Must |
| F9 | Map site: corridor, bottleneck colours, popups, top-fixes panel, method page | Must |
| F10 | OSM attribution and ODbL notice on the site | Must |
| F11 | Both travel directions analysed separately (AM inbound, PM outbound) | Should |
| F12 | Shareable link to a specific segment (`#edge=123`) | Should |
| F13 | Compare against known jam spots (validation table in repo) | Should |

## 7. Success criteria

**Phase 1 is done when:**
- `make all` reproduces the site data from a raw OSM file with no manual steps.
- The top 3 bottlenecks match places locals already know jam (checked against Tim's list + one public traffic source). If they don't, we fix the model before launch, not after.
- A non-technical friend can explain the map back after 30 seconds.
- Site is live on free static hosting.

**Long-term signal:** a journalist, councillor or MP cites a finding.

## 8. Phases

| Phase | Scope | New capability |
|---|---|---|
| **1** | One corridor, static site | Bottlenecks + "+1 lane" ranking |
| **1.5** | Same corridor, static site, demand + CTM simulator (fixed routes) | Calibrated baseline; tested fixes ranked by delay saved (`scenarios.json`) |
| **2** | Several Klang Valley corridors, small API | User "what if" edits; junction capacity |
| **3** | Route choice (user equilibrium) | Braess's paradox → "remove a road" findings; induced demand |
| **4** | Buses, rail, walking layers | Crowdsourced bus stops (contributed back to OSM) |

## 9. Risks

| Risk | Mitigation |
|---|---|
| Model flags a "bottleneck" locals know is wrong → credibility gone | Validation gate before launch (§7) |
| OSM lane tags missing/wrong | Infer by road class, flag it on the map, invite OSM fixes |
| Capacity ≠ congestion (no demand data) | Say clearly: "this shows where the road can carry the least, not where it is busiest" |
| Public reads "+1 lane" as "build more roads" | Method page explains induced demand; Phase 3 compares with transit |
| Demand data is coarse, old or annual (JKR/HPU counts are AADT at a few stations; LLM stats are per plaza) → simulated jams in the wrong place | Validation gate (goal 8) blocks publishing fixes; show demand source and year on the site; mark low-confidence inputs; calibrate peak factors and split ratios to Google Maps typical times and local observations |
| CTM is 1-D cells: it cannot model divider geometry, lane-level weaving or a physical obstacle's shape | Model dividers/channelisation only as lower capacity drop + zipper priority, always labelled **low confidence** (decision 0008); never claim a geometry is tested |
| Induced demand: a fix that cuts delay attracts more traffic, so real savings are smaller | Fixed-route model says so next to every result; show savings as "with today's traffic"; real treatment is Phase 3 |
| Scope creep (it's a side project) | Non-goals list above is binding; changes go through REVIEW.md |

## 10. Open questions

1. Exact corridor bounding box and gates — Tim to confirm on a map.
2. Tim's list of 5 known jam spots on the corridor (for validation).
3. Project name and domain.
4. Licence: code MIT or Apache-2.0? (Data output is ODbL either way.)
