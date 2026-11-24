# DESIGN_BRIEF.md — project design brief: how the site looks, moves and behaves

> Project-specific rules. Read `../DESIGN.md` (shared foundation) and the platform guide (`../DESKTOP.md`, `../MOBILE.md`) first, then this file, and `../../CONTENT.md` for wording. The order of work (design on the canvas, review, build, verify) is `../WORKFLOW.md`; the editable design document is `design.fig` beside this file. The site is a responsive web page, so both platform guides apply. Where this file is more specific than the shared guides, this file wins: it records decisions already made and built.

Design decisions with a story behind them are ADRs in `docs/decisions/`: 0004 (lane drops headline), 0010 (no landing page), 0011 (animation honesty), 0012 (lanes and cars), 0013 (the look: light page, ink dots, one red), 0014 (red for jams, yellow for nearly full roads), 0015 (map cards, trial only).

**The target look is `docs/design/project/light-look-mockup.html`** (decision 0013). Open it in a browser before building anything visual. Where the code still differs from this document, the document is the target; 0013 lists the build order.

Section numbers are referred to from code comments in `web/` (for example "DESIGN_BRIEF.md §4"). Add new sections at the end; do not renumber.

## Product intent
JalanFlow explains where a road runs out of lanes, and how much traffic a corridor can carry at most, to ordinary drivers in Malaysia's Klang Valley. It is not a live traffic map. The corridor is Kota Kemuning to Petaling Jaya. Map-first clarity, no dashboard clutter.

Visual work never changes route geometry, calculations, sources or what the model says. Numbers on the site come from the published data files, never from an example in a document or a mockup.

## 1. Page structure
- **One page**: full-screen map + one text column. No separate landing page until Phase 2 adds several corridors; then a "pick your commute" page sits in front (decision 0010).
- **No card.** The text sits straight on the page: no panel background, border or shadow. Top-left on desktop, below the map on phones.
- **One view at a time.** The column shows exactly one of:
  - **Landing**: orb and name, headline, one sentence, "Run the analysis", the three-dot key (the confirmed-jam dot is red), links to "See all lane drops" and "How this works".
  - **Running**: the live counter, "Path n of m", "Skip to the result".
  - **Result**: the result sentence, the bottleneck, the tested fix, a one-line key (bottleneck, nearly full, confirmed jam, moving dots, lane drop), "Replay", "Hide flow", and the same two links as the landing view.
  - **All lane drops**, **one lane drop** and **How this works**: each replaces the column and has a Back button. Escape also goes back.
- No tabs and no map popups (the trial page tries small map cards for a quick look: decision 0015, not adopted). Clicking a marker or a road opens its detail in the column. A road's detail gives its name, lanes, capacity and whether the lanes are guessed; what it carries, and why it is the bottleneck, are added only after the analysis has run. Only roads on the drive can be tapped, and a marker wins where both are under the finger.
- **The list and the map point at each other.** Hovering or focusing a row rings its marker and names it on the map; hovering a marker marks its row and, with the list open, scrolls it into view.
- **Landing shows lanes only.** The drive is a grey line and the confirmed jams are red, because a driver saw them. Everything computed (the red bottleneck, the yellow nearly full roads) appears after "Run the analysis", because the lane-drop finding leads (decision 0004).
- **The map stays readable.** Zoomed out, only the confirmed jam spots and the 10 biggest squeezes get a marker. The rest appear from zoom 13.5, or all at once while the lane-drop list is open.

## 2. Ten-second rule
A commuter opening the site cold must get *where it jams and why* without scrolling or clicking. Anything that delays that (intro screens, autoplay, modals) is out.

## 3. Honesty
- Every visual says what it measures: **lanes**, **capacity** (max flow) or, from Phase 1.5, **simulated traffic**. Never live traffic.
- **No animation shows something the model didn't compute** (decision 0011). Max flow is a steady state: nothing queues, so the site never draws pile-ups from it. Queues appear only when the CTM simulator computes them.
- Guessed lane counts are always visibly marked (dashed lines, hollow dots; a dashed road edge once lanes are drawn).
- Lanes on the map are schematic: equal stripes from the OSM lane count, not surveyed lane positions (decision 0012).
- Phase 1.5 results always carry their confidence label.
- **Observed, computed and tested stay apart.** "Confirmed jam" is only for a spot a driver saw; never put that label on something the model worked out. A tested fix ("+1 lane") is a what-if and must read as one, so it cannot be taken for the road as it is today.
- Moving dots and cars show how much the model sends down a road. Never present them as tracked vehicles.
- A missing or guessed value is shown as missing or guessed, never filled in to look complete.

## 4. Colour
Ink on a light page, red for where it jams and yellow for roads that are nearly full (decision 0014). The same dot is the only mark: lane drops, flow and the orb.

Defined once: CSS variables in `web/src/style.css` (`:root`). `web/src/colors.ts` reads them from the page for MapLibre paint; it does not repeat the hex values. Never hard-code a colour elsewhere.

| Token | Hex | Meaning |
|---|---|---|
| `--bg` | `#f6f7f7` | the page; also the knock-out behind markers and map labels |
| `--ink` | `#15181b` | text, the drive, lane-drop markers, flow dots, the route being traced, the orb, the primary button |
| `--muted` | `#5d646b` | secondary text |
| `--road` | `#dcdfe2` | modelled roads that are not on the drive |
| `--line` | `#c5cace` | borders of secondary buttons |
| `--cut` | `#d7261e` | **where it jams.** A ringed dot is a jam a driver confirmed; a thick line is the bottleneck the analysis found (min cut). Each keeps its label |
| `--full` | `#f2b200` | roads the analysis fills to 90 % or more of what they can carry, bottleneck excluded. Shown only after the analysis |
| `--marker` | `#ffd100` | expressway route markers only |

- **One utilisation colour.** A road at 90 % or more (`NEARLY_FULL` in `web/src/copy.ts`) gets a yellow band under its ink line; zoomed in, a yellow road edge. Always call it "nearly full", never "high traffic" or "busy": nothing is measured (CONTENT.md). Below that, spare room is shown by how many dots flow. `--marker` yellow is too faint for a line on this page, so `--full` is a darker amber.
- **Red is observed or computed, told apart by shape.** Confirmed jams are red on every view, because they are observations. The bottleneck is red only after the analysis.
- **Lane-drop markers:** red ringed dot = confirmed jam, solid ink dot = lane drop, hollow ink dot = lane count guessed. After the analysis the ink markers step back; the confirmed jams and their labels stay.
- **Route markers**: KESAS and LDP carry the yellow hexagon "E5" / "E11" used on Malaysian expressway signs, ink outline and ink number.
- **Basemap** (decided 2026-11-24): the OpenFreeMap style drained to pale greys close to the page. Streets are `--map-street` (`#e9ebed`), water is `--map-water` (`#e3e7ea`), and everything else, including every label, is hidden. It gives off-drive lane drops a road to sit on without competing with the ink. If the style cannot be fetched the map falls back to the plain page.
- **Zoomed in** (decided 2026-11-24, decision 0012's lanes and cars): the road surface is `--road` with an ink edge, lane lines are `--bg`, and cars are small ink shapes with a `--bg` knock-out. The bottleneck stretch is `--cut`; a nearly full road has a `--full` edge.
- Labels on the map that use the road-sign face (place names, route markers, "Confirmed jam", "Bottleneck, 3 lanes") are HTML markers, not map text layers: a map text layer can only use the tile server's fonts.
- An expressway gets a route marker once it carries at least 3 km of the drive, so E1 and E6 show one as well as E5 and E11.

## 5. Type and layout
- **Road-sign face** for the headline, result sentence, counter, place names and route markers: `"DIN Alternate", "Bahnschrift", "Roboto Condensed", "Arial Narrow", system-ui`, bold. These ship with the device, so there is no font file and no dependency.
- **Body**: `system-ui`.
- Sizes: headline and result sentence 27 px (22 on phones), running counter 64 px (48 on phones), body 15 px, key and notes 12.5 px, map labels 12 px, place names 14 px.
- **The result is a sentence, not a big number with a caption.** The large counter appears only while the analysis runs, where it is counting.
- Desktop: text column 372 px wide, 44 px from the left, 36 px from the top. Phones (≤ 700 px): map above, text column below with 20 px side margins; the map is sized to the space the text leaves. The column is never taller than 62 % of the screen (lists scroll inside it). Each time the view changes the map is fitted again to what that view is about (the lane drops before the analysis, the drive after), unless the reader has moved the map since. A detail view keeps its point centred. List rows and links are at least 44 px tall, the compass is hidden, the map credit folds into a button, and "not live traffic" sits at the foot of the column.
- Trial page (2026-11-24): every size, space, radius, border and timing is a token at the top of `web/src/trial/trial.css`. Type roles: display 52 px (40 on phones), title 22 px (20), figure 20 px, body 15 px, small 14 px, meta 13 px, label 12 px. Spacing on a 4 px scale, with 24 px between panel sections (16 on phones) and one hairline, no boxes. Radii 6 / 8 / 12 px. Controls share hover (lighter surface), pressed, focus and disabled states.
- Buttons are 44 px tall with an 8 px radius. Primary = ink fill. Secondary = `--line` border. Text links are underlined.
- Map fitting and flying always keep clear of the text column, measured from the column itself, not from constants.

## 6. Motion
- Nothing moves until the user asks. No autoplay.
- Every animation has **Skip** and finishes in **≤ 15 s** whatever the data size.
- `prefers-reduced-motion: reduce` → jump straight to the end state; no moving dots.
- Background loops (flow dots and cars) pause when the tab is hidden or the toggle is off.
- **The orb** (the logo beside the name) is ordered and turning slowly at rest. When the analysis starts it scatters, then tightens as flow is found: disorder = 1 − found ÷ max. It is our own canvas code and never sits on the map, so it cannot be mistaken for a computed result (decision 0011). With reduced motion it is still.
- Flow is dots when zoomed out and cars in lanes from zoom 14 (decision 0012). All vehicles move at one speed, set in screen pixels so it reads the same at every zoom.
- On the trial page, overview dots are quieter (2026-11-24): fewer below zoom 13 (a fifth of the dots at zoom 11, a quarter at 12), smaller, and with a fainter edge, so the red bottleneck and amber nearly-full roads show through. Dots are thinned evenly on every road, so a road with twice the flow still shows twice the dots. The cars in lanes are unchanged.

## 7. Copy
Plain language, local road names, "veh/h". Short sentences; caveats once. Full rules: `docs/CONTENT.md`.

## 8. Accessibility
- Every control is a real `<button>` (or a link) with a visible `:focus-visible` ring in ink. Opening a view moves focus to its Back button; going back returns focus to where it was.
- The lane-drop list is the keyboard route to every marker, since map markers cannot take focus.
- Text contrast at least WCAG AA.
- Colour is never the only signal: the bottleneck also gets width, a pulse and a label; a confirmed jam has its ring and label; a nearly full road says so when tapped; marker types differ by shape (ring, solid, hollow); guessed lanes also get dashes.
- Status changes during the replay (counter, captions) go in an `aria-live="polite"` region.

## 9. Performance
- First meaningful map in < 2 s on 4G. Data files are compact JSON; only flow-carrying edges are exported.
- Animations stay smooth on a mid-range phone: ≤ 3k vehicles, `setData` at ≤ 30 fps, and only vehicles on screen are sent to the map.

## 10. Constraints
- Plain DOM + MapLibre GL, TypeScript strict. No framework, no new dependencies.
- OSM attribution always visible.
- Display-only constants (dot density, timings) may live in TS, named and commented. Capacity, speed and traffic numbers never do (they live in `config/`).
- No control the data cannot back. A direction switch only if the output holds both directions separately; no AM/PM mode just because a mockup had one.

## 11. Shareability
- **Deep links.** The address bar always names what the column shows (`web/src/links.ts`), so the link can be shared as it is:

  | Link ends with | Opens |
  |---|---|
  | nothing | the landing view |
  | `#result` | the result, without replaying the search |
  | `#lane-drops` | the list |
  | `#how` | How this works |
  | `#drop-n33987957` | one lane drop |

  A lane drop's link uses its OpenStreetMap id, not its rank, so it still points at the same place after the data is refreshed. Roads have no lasting id and no link of their own. An unknown link opens the landing view.
- Opening `#result` starts the flow dots without a tap. That is the one exception to "nothing moves until the user asks" (§6): the link is the ask. Reduced motion still gets a still picture.
- **Share preview.** `web/index.html` carries a description and Open Graph / X tags; the image is `web/public/og.png` (1200 × 630). Preview images need a full address, so the tags name `https://tlmotan.github.io/jalanflow/`: change them if the site moves.
- The image shows the landing look (lanes and the red confirmed jams, no bottleneck) and is drawn from the published data by `web/og-card.html`, a page the build ignores. Redo it after the data or the look changes: start the dev server, then

  ```bash
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --hide-scrollbars --window-size=1200,630 --virtual-time-budget=6000 --screenshot=web/public/og.png http://localhost:5173/og-card.html
  ```

## 12. Acceptance checks
Run these as well as the checklists in `DESKTOP.md` §7 and `MOBILE.md` §9 before calling a change under `web/` done.

- A first-time visitor can tell the corridor, where the lanes run out and (after the analysis) the bottleneck and what the corridor can carry, without scrolling (§2).
- The map makes sense with no animation: flow hidden, and with reduced motion on (§6).
- Today's road and a tested fix cannot be confused; observed and computed marks cannot be confused (§3).
- "Not live traffic" is on screen in every view.
- Every finding on the map can also be reached in the text column, on desktop and on a phone (§8).
- Nothing that worked before is lost. Report any view, width or state you did not check.
