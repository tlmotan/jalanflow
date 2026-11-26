# User flow: every screen and state of the site

The list `screenshots/flow.mjs` (`make shots`) must walk. One row per screen a
reader can reach. When a change to `web/` adds, removes or renames a screen,
update this file and `flow.mjs` in the same change.

Source of truth: `web/index.html`, `web/src/main.ts`, `web/src/links.ts`.
Checked against them on 2026-11-26.

**Shot** column: ✅ `flow.mjs` captures it today, ⬜ it does not yet.
Shot names are the ones `flow.mjs` uses (without the number prefix).

## 1. How the page describes itself

The page puts its state on `<body>`, so the script can wait on these instead of timers.

| Attribute | Values | Set by |
|---|---|---|
| `data-load` | `loading` → `ready`, or `failed` | the data fetch at start |
| `data-state` | `home` (nothing computed), `running` (the replay), `result` | Run, Skip, the search ending, a link |
| `data-view` | `summary`, `list` (lane drops), `detail` (one road or drop), `how` | the panel's links, Back, Escape, a link |
| `data-more` | `closed`, `open` | phones only: More detail, a drag of the handle |
| `data-sheet` | `collapsed`, `summary`, `expanded` | phones: the sheet's height (decision 0018). The handle (a tap or a drag), More detail, a view change. The list, a detail, How this works and a peek have no `summary` |
| `data-settle` | `on` or absent | phones: while the sheet eases to its new height |
| `data-peek` | `on` or absent | phones / small maps: a card shown in the sheet instead of on the map |
| `data-drag` | `on` or absent | phones: while the handle is dragged (not on a tap) |

Direction is not on `<body>`: read `.seg button[aria-pressed=true]` (`inbound` = to Petaling Jaya, the default; `outbound` = to Kota Kemuning).

Layouts (`PHONE` in `main.ts`, `@media` in `style.css`):
- **Desktop**: panel beside the map. Anything wider than 760 px, or 561–760 px in landscape.
- **Phone**: `(max-width: 760px) and (orientation: portrait), (max-width: 560px)`. Floating sheet over a full-height map (decision 0017) with three heights (decision 0018), a handle that is a button, More detail button. The sheet never moves the map.
- **Card or peek**: independent of the layout. A card sits on the map only when the visible map is at least 320 × 420 px (`CARD_MIN_MAP_W/H`); otherwise it goes into the sheet/panel as a peek. So a short desktop window gets peeks too.

## 2. Viewports

| Viewport | Size | Why | Shot |
|---|---|---|---|
| desktop | 1440 × 900 | main desktop | ✅ |
| mobile | 390 × 844 | main phone | ✅ |
| mobile-short | 375 × 667 | short phone, sheet covers most of the map | ✅ |
| small phone | 320 × 568 | narrowest width MOBILE.md asks for | ✅ short walk |
| tablet portrait | 768 × 1024 | just above the phone breakpoint: desktop layout on a narrow screen | ✅ short walk |
| desktop short | 1280 × 640 | DESKTOP.md "short viewport"; map may be under 420 px tall, so cards become peeks | ✅ short walk |
| phone landscape | 844 × 390 | desktop layout with a very short map: peeks on desktop layout | ✅ short walk |
| reduced motion | 390 × 844 and 1440 × 900 with `reducedMotion: "reduce"` | Run goes straight to the result, no camera moves, no lane-diagram draw | ✅ short walk |

## 3. Screens

### A. Loading and failure

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| A1 | Loading: "Loading the road data…" | first paint, before `data-load=ready` | ✅ `failure-*/loading` |
| A2 | Load failed: "The road data did not load…" + Try again | `page.route('**/edges.geojson', r => r.abort())` → `data-load=failed` | ✅ `failure-*/road-data-failed` |
| A3 | Lane drops missing: counts say "The lane-drop data did not load.", no See all lane drops | abort `**/lane_drops.json` only | ✅ `failure-*/lane-drops-failed` |
| A4 | Basemap missing: our roads carry the OSM credit | abort the basemap style request (`basemap.ts`) | ✅ `failure-*/basemap-failed` |

A2 throws on purpose (`jalanflow: road data did not load`); exclude it from the page-error check.

### B. Home (before the analysis, `data-state=home`)

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| B1 | Home, to Petaling Jaya | open `/` | ✅ `home-to-petaling-jaya` |
| B2 | Home, to Kota Kemuning | `.seg [data-dir=outbound]`, or open `#outbound` | ✅ `home-to-kota-kemuning` |
| B3 | Home, phone sheet open (More detail) | `#more-btn` | ✅ `home-more-detail-open` |
| B4 | Card: confirmed jam (before analysis) | tap beside a "Confirmed jam" tag | ✅ `home-jam-card` |
| B5 | Card: lane drop, solid dot | tap a `drops-key` marker that is not confirmed | ✅ `home-card-lane-drop` |
| B6 | Card: lane drop, hollow dot (lanes guessed, with "Some lane counts here are guessed.") | tap a drop with `lanes_inferred` (4 in the data). None is a key marker, so none shows at the starting zoom | ⬜ (its detail is F7) |
| B7 | Card: road, kind "Road", "Run the analysis to see…" note | tap a drive road, away from markers | ✅ `home-card-road` |
| B8 | Road detail before analysis: "Run the analysis to see how much of this it would use." | B7 → View details | ✅ `home-detail-road` |
| B9 | Lane drop detail from home | B5 → View details | ✅ `home-detail-lane-drop` |
| B10 | Hover preview (desktop): unpinned card on a marker, closes on leave | `page.mouse.move` onto a marker | ✅ `home-hover-preview` |

### C. Running (`data-state=running`)

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| C1 | Running, to Petaling Jaya: count, route segments, caption | `#run-btn`, wait ~1.8 s | ✅ `running-analysis` |
| C2 | Running, first route (count 0, first segment "now") | shoot right after `data-state=running` | ✅ `running-first-route` |
| C3 | Running, near the end / "routes done" caption | wait until the last segment is `done` | ✅ `running-all-routes-done` |
| C4 | Running, a caption where a route takes back earlier flow | outbound only today: steps 10 and 11 of 12 in `replay.json` have a negative delta | ✅ `running-route-moves-earlier-flow` |
| C5 | Running, to Kota Kemuning | switch to outbound, then Run | ✅ `running-to-kota-kemuning` |
| C6 | Skip to the result | `#skip-btn` during C1 → result | ✅ `skipped-to-result` |
| C7 | Direction switched while running → back to home in the other direction | click the other `.seg` button during C1 | ✅ `running-direction-switched-home` |
| C8 | Map tap while running does nothing | tap a marker during C1, expect no `.card` | ✅ check only |

### D. Result (`data-state=result`, `data-view=summary`)

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| D1 | Result, to Petaling Jaya | run to the end | ✅ `result` |
| D2 | Result, to Kota Kemuning | switch direction on the result | ✅ `result-to-kota-kemuning` |
| D3 | Phone: sheet open (what-if bars, About this estimate, key, links) | `#more-btn` | ✅ `result-more-detail-open` |
| D4 | Phone: sheet open, scrolled | scroll `#panel` | ✅ `result-more-detail-scrolled` |
| D5 | Phone: dragged to summary / collapsed / expanded; the map does not move | handle ±px | ✅ `result-dragged-closed`, `result-dragged-collapsed`, `result-dragged-open` |
| D6 | Phone: handle mid-drag (`data-drag=on`, the sheet's height following the finger) | mouse down, move, shoot, then up | ✅ `result-dragging-down` |
| D7 | Phone: handle tap collapses the sheet (title line and "not live traffic" stay), another brings it back | click `#grab` without moving | ✅ `home-handle-tapped-collapsed` |
| D8 | Flow hidden | `#flow-btn` | ✅ `result-flow-hidden` |
| D9 | Flow shown again (button back to "Hide flow", key row back) | `#flow-btn` twice | ✅ `result-flow-shown-again` |
| D10 | Desktop: lower panel (what-if bars, key, links) | scroll `#panel` on desktop | ✅ `result-panel-scrolled` |
| D11 | Replay the analysis | `#replay-btn` → running again | ✅ `replay-running` |
| D12 | Result, outbound, sheet open (its own what-if and lane strip) | D2 + `#more-btn` | ✅ `result-to-kota-kemuning-detail` |

Data-dependent variants (shoot whichever the current data gives, per direction; note which in the run log): what-if with a gain vs "One more lane there would not raise the limit", lane strip shown vs hidden, `#limit-btn` disabled when there is no cut.

### E. Result cards on the map

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| E1 | Card: bottleneck road (red, "Bottleneck") | tap beside the "Bottleneck, N" tag | ✅ `bottleneck-road-card` |
| E2 | Card: confirmed jam (after analysis) | tap beside "Confirmed jam" | ✅ `jam-card` |
| E3 | Card: road "Nearly full" | tap a road with the `full` style | ✅ `result-card-nearly-full` |
| E4 | Card: road "Room to spare" | tap any other drive road | ✅ `result-card-room-to-spare` |
| E5 | Card: lane drop solid / hollow after analysis | as B5, B6 | ✅ `result-card-lane-drop` (solid only, as B6) |
| E6 | Card's lane diagram mid-draw (pinned card) | shoot ~150 ms after the tap | ✅ `jam-card-drawing` |
| E7 | Phone: card shown in the sheet (peek) | D3 open, tap a jam | ✅ `jam-card-in-sheet` |
| E8 | Phone: sheet collapsed over a peek; the quick look stays open | handle down on E7 | ✅ `jam-card-sheet-collapsed` |
| E9 | Peek closed with its Close button | E7 → `#peek .card button` "Close" | ✅ `jam-card-in-sheet-closed` |
| E10 | Card moved into the sheet when the map shrinks (window resized with a card open) | open E1 on desktop, `page.setViewportSize` to a short height | ✅ `card-moved-into-panel` |
| E11 | Escape closes the card first, then the view | open a card, press Escape | ✅ check only |
| E12 | Second marker replaces the first card | open E2, then tap another marker | ✅ `card-replaced-by-next-tap` |

### F. Detail (`data-view=detail`)

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| F1 | Bottleneck detail, map zoomed to it | `#limit-btn` | ✅ `bottleneck-detail` |
| F2 | Road detail from a card | E1 → View details | ✅ `road-card-view-details` |
| F3 | Road detail, nearly full | E3 → View details | ✅ `result-detail-nearly-full` |
| F4 | Road detail, room to spare | E4 → View details | ✅ `result-detail-room-to-spare` |
| F5 | Lane drop detail, top of the list | list → first row | ✅ `lane-drop-detail` |
| F6 | Lane drop detail, confirmed jam ("A local driver confirmed…") | open `#inbound/drop-n33987957` | ✅ `links-*/drop-jamNode` |
| F7 | Lane drop detail, lanes guessed ("Check on Street View") | list row with "Lanes guessed" | ✅ `links-*/drop-guessed` |
| F8 | Lane drop detail, merge vs lane loss wording | a `kind: merge` drop and a non-merge drop | ✅ `links-*/drop-merge`, `drop-narrowing` |
| F9 | Phone: detail, handle down collapses the sheet; it does not go back | F1 on a phone, drag down | ✅ `detail-dragged-collapsed` |
| F10 | Detail when the direction switches → back to summary | F1, then the other `.seg` button | ✅ `detail-direction-switched` |

### G. Lane drops list (`data-view=list`)

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| G1 | List | `#open-list` | ✅ `lane-drops-list` |
| G2 | Phone: list scrolled, Back stays at the top | scroll `#panel` | ✅ `lane-drops-list-scrolled` |
| G3 | Desktop: row hover highlights its marker | hover `#list button` | ✅ `lane-drops-row-hovered` |
| G4 | Desktop: marker hover highlights its row (`.hot`) | hover a marker while the list is open | ✅ `lane-drops-marker-hovered` |
| G5 | List before the analysis (home state) | `#open-list` from B3 / desktop home | ✅ `home-lane-drops-list` |
| G6 | List, to Kota Kemuning | outbound + `#open-list` | ✅ `lane-drops-list-to-kota-kemuning` |
| G7 | Map showing all markers (`drops-rest` at every zoom) | any list shot, check the map | covered by G1 |

### H. How this works (`data-view=how`)

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| H1 | How this works, top | `[data-open=how]` | ✅ `how-this-works` |
| H2 | How this works, end (OSM date, © link) | scroll to the end | ✅ `how-this-works-end` |
| H3 | Phone: handle down collapses the sheet; it does not go back | drag down on H1 | ✅ `how-dragged-collapsed` |
| H4 | How this works from home | `[data-open=how]` before running | ✅ `home-how-this-works` |

### I. Deep links (`links.ts`)

Each opens straight into a screen, with no clicks. Load each URL fresh.

| ID | Link | Lands on | Shot |
|---|---|---|---|
| I1 | `#outbound` | B2 | ✅ `links-*/home-to-kota-kemuning` |
| I2 | `#inbound/result` | D1 without the replay | ✅ `links-*/result` |
| I3 | `#outbound/result` | D2 without the replay | ✅ `links-*/result-to-kota-kemuning` |
| I4 | `#outbound/lane-drops` | G6 | ✅ `links-*/list-to-kota-kemuning` |
| I5 | `#inbound/how` | H1 | ✅ `links-*/how-this-works` |
| I6 | `#inbound/drop-n33987957` | F6, marker picked once the map loads | ✅ `links-*/drop-jamNode` |
| I7 | `#inbound/drop-w148018437-1116586907` | a way-keyed drop detail | ✅ `links-*/drop-jamWay` |
| I8 | Old links: `#result`, `#lane-drops`, `#how`, `#drop-n33987957` | same as I2, G1, H1, F6, to Petaling Jaya | ✅ `links-*/old-link-*` |
| I9 | Unknown: `#nonsense`, `#inbound/drop-n0` | B1 (home) | ✅ `links-*/unknown-*` |
| I10 | Back/forward after a link change (`hashchange`) | set `location.hash` on a loaded page | ✅ `links-*/hashchange-*` |

Also assert the address bar after each step (`syncLink`): e.g. the result writes `#inbound/result`, a drop detail writes `#…/drop-…`, a road detail writes the summary's hash.

### J. Keyboard and focus

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| J1 | Skip link visible | Tab once from load | ✅ `keyboard/skip-link-focused` |
| J2 | Focus on Back after opening a view | open list / how / detail, shoot focus ring | ✅ `keyboard/how-back-focused` |
| J3 | Focus back on the opener after Back | Back from the list → focus on `#open-list` | ✅ `keyboard/back-focus-returned` |
| J4 | Focus on Skip while running | C1 | ✅ `keyboard/running-skip-focused` |
| J5 | Escape from a view returns to the summary | open how, press Escape | ✅ check only |

### K. Map zoom

| ID | Screen | How to reach | Shot |
|---|---|---|---|
| K1 | Zoom ≥ 13: the other lane-drop markers appear | zoom in with the + control | ✅ `map-zoomed-in` |
| K2 | Zoom ≥ `LANES_FULL_ZOOM`: lanes drawn per lane | zoom further, or a detail view (F1) | ✅ `map-zoomed-to-lanes` |
| K3 | Reader moved the map; a view change keeps their camera | drag the map, then open a card | ⬜ |

## 4. How `flow.mjs` walks it

`make shots` runs every pass; `SHOTS_ONLY=desktop,links-mobile make shots` runs some. Each pass has its own folder under `screenshots/flow/`, with numbered JPGs.

| Pass | Viewport | Covers |
|---|---|---|
| `desktop`, `mobile`, `mobile-short` | 1440 × 900, 390 × 844, 375 × 667 | the full walk: B, C, D, E, F (not F6–F8), G, H, K |
| `links-desktop`, `links-mobile` | 1440 × 900, 390 × 844 | I, and F6–F8 by link |
| `failure-desktop`, `failure-mobile` | 1440 × 900, 390 × 844 | A |
| `keyboard` | 1440 × 900 | J |
| `small-phone`, `tablet`, `desktop-short`, `phone-landscape`, `reduced-desktop`, `reduced-mobile` | §2 | the short walk: start, a card, run, result, bottleneck detail, list |

The full walk goes: start → cards and details before the analysis → list and How this works before it → run, switched mid-way → run to the end → result (sheet, drags, flow off and on) → bottleneck detail → cards and details after → phone sheet peeks / desktop short window → list → How this works → Replay and Skip → the other direction (its run, list, a detail across the switch) → zoom.

Besides screenshots, the script checks: the address after each view, `data-state` / `data-view` / `data-load`, focus (Skip while running, Back in a view, the opener after Back), that a card it opened is visible, and that a map tap while running opens nothing. A failed check, a page error or a pass that stops makes it exit non-zero. Screens it could not reach are listed as "not reached" and do not fail the run.

## 5. How it finds features to tap

Markers and roads are drawn on the map canvas, so they are not in the page.
- Confirmed jams and the bottleneck have HTML labels 14 px beside them (`pin()` in `main.ts`): taps aim there.
- Everything else: the script sweeps the visible map with synthetic mouse moves and keeps the points where the map sets the pointer cursor (over a marker or road), then taps those until a card of the wanted kind opens, reading the kind from the card (`.card-kind`). It probes again if the camera moved. No change to `web/` is needed.
- Guessed-lane markers are not key markers, so none shows at the starting zoom; their detail is shot from a link instead (F7).

## 6. Motion (`screenshots/motion.mjs`)

Stills cannot show what moves, so each motion on the site also has a GIF clip at real speed. When a change adds or changes motion, add or update its clip here and in `motion.mjs`.

Run from `screenshots/` with the dev server up (needs `ffmpeg`): `node motion.mjs http://localhost:5173 <outDir>`. `MOTION_ONLY=card,sheet` records only the clips whose names contain one of those words. It exits non-zero on a page error.

| Clip | What moves | Screens |
|---|---|---|
| `desktop-card-hover` | a hover preview fades in on a marker and goes when the pointer leaves | B10 |
| `desktop-card-open` | a clicked card fades in and its lane diagram draws itself once | B4, E6 |
| `desktop-run-analysis`, `mobile-run-analysis` | the search replay: routes traced one by one, the count, the segments, then the result | C1–C3, D1 |
| `desktop-flow-dots`, `mobile-flow-dots` | flow dots on the roads the analysis uses | D1 |
| `desktop-camera-views` | the camera between views: the bottleneck, back to the drive, every lane drop, back | F1, G1 |
| `desktop-cars-in-lanes` | dots become cars in lanes when zoomed in | K2 |
| `desktop-switch-direction`, `mobile-switch-direction` | the drive redrawn for the other direction | D2 |
| `mobile-sheet-more-detail` | More detail opens the sheet and closes it | B3 |
| `mobile-sheet-drag` | the handle dragged up expands the sheet, down collapses it, up again rests at summary; the height follows the finger and the map stays put | D5, D6 |
| `mobile-card-on-map` | a card fades in on the map above the sheet, its diagram drawing | E2 |
| `mobile-card-in-sheet` | with the sheet open the card takes its place | E7 |
| `mobile-detail-back` | the camera goes to the bottleneck; Back returns to the result | F1 |

Not recorded: reduced motion (nothing moves; the stills in the `reduced-*` passes show it), and the card fade when it closes (120 ms, in `desktop-card-hover`).
