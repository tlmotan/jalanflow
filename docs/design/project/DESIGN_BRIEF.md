# DESIGN_BRIEF.md — project design brief: how the site looks, moves and behaves

> Project-specific rules. Read `../DESIGN.md` (shared foundation) and the platform guide (`../DESKTOP.md`, `../MOBILE.md`) first, then this file, and `../../CONTENT.md` for wording. The order of work (design on the canvas, review, build, verify) is `../WORKFLOW.md`; the editable design document is `design.fig` beside this file. The site is a responsive web page, so both platform guides apply. Where this file is more specific than the shared guides, this file wins: it records decisions already made and built.

Design decisions with a story behind them are ADRs in `docs/decisions/`: 0004 (lane drops headline), 0010 (no landing page), 0011 (animation honesty), 0012 (lanes and cars), 0013 (the earlier light look; its map rules still hold), 0014 (red for jams, yellow for nearly full roads), 0015 (map cards), 0016 (the dark shell is the site), 0017 (the phone sheet floats over the map), 0018 (the design refinement: agreed in concept, not built).

**Decision 0018 is partly built.** Built 2026-11-26, straight in `web/` and reviewed in the browser (Tim set the canvas step aside for this pass): the phone sheet's three heights, the sheet no longer moving the camera, and the result's new order with the what-if bars and the two disclosures. Not built yet: list rows opening a card, the road detail's bar and "How this works". Where a rule below is still to change, a note beginning "Changing (decision 0018…)" says how; until that part is built the rule as written is what the site does. When it is built, fold the note into the rule; delete this paragraph when none are left.

**The look is the dark shell around a light map** (decision 0016, Tim, 2026-11-25): the site as built in `web/`, drawn in `design.fig`. `light-look-mockup.html` is the earlier light look (decision 0013), kept for the record; its map still looks much the same.

Section numbers are referred to from code comments in `web/` (for example "DESIGN_BRIEF.md §4"). Add new sections at the end; do not renumber.

## Product intent
JalanFlow explains where a road runs out of lanes, and how much traffic a corridor can carry at most, to ordinary drivers in Malaysia's Klang Valley. It is not a live traffic map. The corridor is Kota Kemuning to Petaling Jaya. Map-first clarity, no dashboard clutter.

Visual work never changes route geometry, calculations, sources or what the model says. Numbers on the site come from the published data files, never from an example in a document or a mockup.

## 1. Page structure
- **One page**: a top bar, the map and one panel. No separate landing page until Phase 2 adds several corridors; then a "pick your commute" page sits in front (decision 0010).
- **Dark shell, light map** (decision 0016). The bar holds the name, the corridor and the direction switch. The map is a rounded light card. The panel sits beside the map on desktop, never over it. On phones the map fills the screen under the bar and the panel floats over its bottom as a sheet (decision 0017).
- **No boxes in the panel.** Text sits straight on the shell, with one hairline between sections. Only controls and the map card have a surface.
- **A direction switch**, "To Petaling Jaya" / "To Kota Kemuning", because the data holds both directions separately (§10).
- **One view at a time.** The panel shows exactly one of:
  - **Start** (design.fig page 6, approved by Tim 2026-11-25): headline, a muted corridor line, two count lines each led by its map mark (lane drops, confirmed jams; numbers in bold, from the data), "Run the analysis" as the one filled control (full width on phones), the not-live-traffic line right under it, then the key and the links, smaller and muted.
  - **Running** (design.fig page 4, "Running v2", approved by Tim 2026-11-25): the live counter, then one thin segment per route in `replay.json` (done white, the route being traced muted, the rest `--edge`) with "Route n of m" beside it, then one sentence for that route from its own amount ("Route 4 has room for 900 vehicles an hour."; "…by moving part of an earlier route." when the step cancels earlier flow), then "Skip to the result". The last beat says "No route has room left, so this is the most." The segments count real steps: no percentage, no fill inside a segment. Screen readers get a progressbar with "Route n of m"; the live region still announces only the start and the result.
  - **Result** (decision 0018): the most the roads can carry as the one big number with its qualifier, the bottleneck (a button that shows it on the map) with "View lane layout" under it (the lane strip of §13, closed until asked for), then the what-if: "What if we added one lane?", a line saying it is a test in the model, and two labelled bars, "Today" and "With one more lane", from the same zero on one scale, each with its number and unit. The gain is the lighter part of the second bar and is also said as a number ("+2,000"). No gain shows equal bars and the reason from `fixes.json`; no baseline shows no bars (`whatIfBars` in `web/src/copy.ts`). How hard it would be to build, and what the test leaves out, sit under "About this estimate". Then the key, "Replay the analysis", "Hide flow", and the same two links.
  - **All lane drops**, **one lane drop or road** and **How this works**: each replaces the panel and has a Back button. Escape also goes back.
- On phones the sheet has three heights, each from its content (decisions 0017, 0018). **Summary**, the default, shows the top of the view. **Expanded** ("More detail") shows the rest, up to 64 % of the screen and always leaving 220 px of map; "Less detail" stays at the top of the sheet, as does Back in the other views, and only the sheet scrolls (overscroll contained). **Collapsed** leaves the handle, one line saying what the page shows and "Worked out from lane counts. It is not live traffic.", so the map has nearly the whole screen. The list, a detail, How this works and a quick look in the sheet have no shorter summary: they are expanded or collapsed. Every change has a button: the handle is a real button that hides the findings or brings them back, and More detail / Less detail goes between summary and expanded. Dragging the handle does the same by height: the sheet follows the finger and settles at the nearest height, or the next one on a flick. A drag only changes the height; it never goes back or closes anything. The map never changes size, and the sheet never moves the camera (§5).
- **A tap on a marker or a road opens a small card beside it** (decision 0015); the panel keeps what it was showing. "View details" opens the full detail in the panel. No tabs, and no other popups. A road's detail gives its name, lanes, capacity and whether the lanes are guessed; what it carries, and why it is the bottleneck, are added only after the analysis has run. Only roads on the drive can be tapped, and a marker wins where both are under the finger.
- **The list and the map point at each other.** Hovering or focusing a row rings its marker; hovering a marker marks its row. **Changing (decision 0018, agreed 2026-11-26, not built):** choosing a row goes to the place and opens its card; "View details" on the card opens the detail.
- **The start shows lanes only.** The drive is a grey line and the confirmed jams are red, because a driver saw them. Everything computed (the red bottleneck, the yellow nearly full roads, the capacity) appears after "Run the analysis", because the lane-drop finding leads (decision 0004).
- **The map stays readable.** Zoomed out, only the confirmed jam spots and the 10 biggest squeezes get a marker. The rest appear from zoom 13, or all at once while the lane-drop list is open.

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
A dark shell around a light map (decision 0016). On the map: ink for markers and labels, red for where it jams, yellow for roads that are nearly full (decision 0014).

Defined once: CSS variables in `web/src/style.css` (`:root`). `web/src/colors.ts` reads the map tokens from the page for MapLibre paint; it does not repeat the hex values. Never hard-code a colour elsewhere.

| Token | Hex | Meaning |
|---|---|---|
| `--canvas` | `#111318` | the shell: page, bar and panel |
| `--surface` | `#1c2028` | controls and the map card |
| `--raised` | `#252a34` | a surface under the pointer or pressed |
| `--text` | `#ffffff` | text on the shell; the primary button's fill |
| `--text-muted` | `#a1a7b0` | secondary text |
| `--divider` | `#2b303a` | the hairline between panel sections |
| `--control` | `#6b7480` | edges of controls |
| `--edge` | `#3a3f47` | the map card's hairline |
| `--bg` | `#f7f8f8` | the map background; the knock-out behind markers and map labels; flow dots |
| `--ink` | `#24292f` | lane-drop markers, map labels, the route being traced, edges of dots, cars and roads |
| `--road` | `#dde1e5` | modelled roads that are not on the drive |
| `--route` | `#6b7785` | the drive: roads between the two places, and after the analysis the roads it uses |
| `--cut` | `#ef4035` | **where it jams.** A ringed dot is a jam a driver confirmed; a thick line is the bottleneck the analysis found (min cut). Each keeps its label |
| `--cut-words` | `#f08078` | the same red, lightened so small words pass contrast on the shell |
| `--full` | `#f5b800` | roads the analysis fills to 90 % or more of what they can carry, bottleneck excluded. Shown only after the analysis |
| `--marker` | `#ffd100` | expressway route markers only |

- **One utilisation colour.** A road at 90 % or more (`NEARLY_FULL` in `web/src/copy.ts`) gets a yellow band under its ink line; zoomed in, a yellow road edge. Always call it "nearly full", never "high traffic" or "busy": nothing is measured (CONTENT.md). Below that, spare room is shown by how many dots flow. `--marker` yellow is too faint for a line on the light map, so `--full` is a darker amber, drawn with an ink edge.
- **Red is observed or computed, told apart by shape.** Confirmed jams are red on every view, because they are observations. The bottleneck is red only after the analysis.
- **Lane-drop markers:** red ringed dot = confirmed jam, solid ink dot = lane drop, hollow ink dot = lane count guessed. After the analysis the ink markers step back; the confirmed jams and their labels stay.
- **Route markers**: KESAS and LDP carry the yellow hexagon "E5" / "E11" used on Malaysian expressway signs, ink outline and ink number.
- **Basemap** (decided 2026-11-24): the OpenFreeMap style drained to pale greys close to the map background. Streets are `--map-street` (`#e9ecee`), water is `--map-water` (`#e3e7ea`), and everything else, including every label, is hidden. It gives off-drive lane drops a road to sit on without competing with the ink. If the style cannot be fetched the map falls back to the plain background.
- **Zoomed in** (decided 2026-11-24, decision 0012's lanes and cars): the road surface is `--road` with an ink edge, lane lines are `--bg`, and cars are small ink shapes with a `--bg` knock-out. The bottleneck stretch is `--cut`; a nearly full road has a `--full` edge.
- Labels on the map (place names, route markers, "Confirmed jam", "Bottleneck, 3 lanes") are HTML markers, not map text layers: a map text layer can only use the tile server's fonts.
- An expressway gets a route marker once it carries at least 3 km of the drive, so E1 and E6 show one as well as E5 and E11.

## 5. Type and layout
Every size, space, radius, border and timing is a token at the top of `web/src/style.css`. Rules use the tokens, not their own numbers.

- **One family**: `system-ui`. Units and data labels ("veh/h") use the device's monospace. No font files, so no dependency. (The road-sign face of decision 0013 is used only on the share image.)
- **Type roles**: display 52 px (40 on phones) for the one big number, title 22 px (20), figure 20 px, body 15 px, small 14 px, meta 13 px, label 12 px.
- **The result leads with the number**: the most the roads can carry, with its unit and "Worked out from lane counts. It is not live traffic." under it. Before the analysis no number is shown.
- **Spacing** on a 4 px scale, with 24 px between panel sections (16 on phones) and one hairline, no boxes. Radii 6 / 8 / 12 px.
- **Desktop**: bar on top, map on the left, panel on the right, `clamp(300px, 32vw, 392px)` wide. **Phones** (portrait up to 760 px wide, or any screen up to 560 px): bar, then the map to the bottom of the screen, with the panel as a sheet floating over the map's lower part (decision 0017).
- **The camera follows the view** (decision 0015): the start, the result and "How this works" frame the whole drive; the list frames every lane drop; a detail frames its feature. Within a view the reader's own pans and zooms are kept. Padding keeps roads clear of the zoom buttons, the credit line and, on phones, the sheet as it stands at that moment.
- **The phone sheet never moves the camera** (decision 0018). The camera moves when the reader asks to go somewhere: a view, the analysis starting or ending, a direction, a picked place, the map's own controls, or the window changing size. Dragging, opening, closing or scrolling the sheet leaves the map exactly where it is, so an expanded sheet may cover part of what was framed; collapsing it shows it again. A card already on the map stays where it is when the sheet grows over it.
- Controls are at least 44 px tall on touch, with an 8 px radius, and share hover (lighter surface), pressed, focus and disabled states. Primary = light fill. Secondary = `--control` border. Text links are underlined.
- The compass is hidden, and on phones the map credit folds into a button.

## 6. Motion
- Nothing moves until the user asks. No autoplay.
- Every animation has **Skip** and finishes in **≤ 15 s** whatever the data size.
- `prefers-reduced-motion: reduce` → jump straight to the end state; no moving dots.
- Background loops (flow dots and cars) pause when the tab is hidden or the toggle is off.
- Flow is dots when zoomed out and cars in lanes from zoom 14 (decision 0012). All vehicles move at one speed, set in screen pixels so it reads the same at every zoom.
- A lane-drop card's lane diagram draws itself once, left to right, in 600 ms (`--dur-draw`) when the card is opened by a tap or click; hover previews show it finished, and it never loops (§14).
- Overview dots are quiet (2026-11-24): fewer below zoom 13 (a fifth of the dots at zoom 11, a quarter at 12), smaller, and with a fainter edge, so the red bottleneck and amber nearly-full roads show through. Dots are thinned evenly on every road, so a road with twice the flow still shows twice the dots. The cars in lanes are unchanged.
- Quiet when informing, livelier when the reader acts (decision 0018). Small changes take `--dur` (150 ms); disclosures open at once. The phone sheet's height follows the finger, a new drag interrupts a settle, and it settles in `--dur-sheet` (220 ms, a token added for this). The replay, the flow and asked-for map moves keep their character.

## 7. Copy
Plain language, local road names, "veh/h". Short sentences; caveats once. Full rules: `docs/CONTENT.md`.

## 8. Accessibility
- Every control is a real `<button>` (or a link) with a visible `:focus-visible` ring. Opening a view moves focus to its Back button; going back returns focus to where it was.
- The lane-drop list is the keyboard route to every marker, and "Show it on the map" to the bottleneck, since map features cannot take focus. Both open the panel detail directly. **Changing (decision 0018, agreed 2026-11-26, not built):** a row opens the place's card and moves focus into it; View details and Close are buttons there, and closing returns focus to the row. No step needs the map canvas.
- Text contrast at least WCAG AA.
- Colour is never the only signal: the bottleneck also gets width and a label; a confirmed jam has its ring and label; a nearly full road says so when tapped; marker types differ by shape (ring, solid, hollow); guessed lanes also get dashes.
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
- **Deep links.** The address bar always names the direction and what the panel shows (`web/src/links.ts`), so the link can be shared as it is:

  | Link ends with | Opens |
  |---|---|
  | nothing | the start, towards Petaling Jaya |
  | `#outbound` | the start, towards Kota Kemuning |
  | `#inbound/result` | the result, without replaying the search |
  | `#outbound/lane-drops` | the list |
  | `#inbound/how` | How this works |
  | `#inbound/drop-n33987957` | one lane drop |

  Links shared before the direction switch (`#result`, `#lane-drops`, `#how`, `#drop-n33987957`) still open, towards Petaling Jaya. A lane drop's link uses its OpenStreetMap id, not its rank, so it still points at the same place after the data is refreshed. Roads have no lasting id and no link of their own. An unknown link opens the start.
- Opening a result link starts the flow dots without a tap. That is the one exception to "nothing moves until the user asks" (§6): the link is the ask. Reduced motion still gets a still picture.
- **Share preview.** `web/index.html` carries a description and Open Graph / X tags; the image is `web/public/og.png` (1200 × 630). Preview images need a full address, so the tags name `https://tlmotan.github.io/jalanflow/`: change them if the site moves.
- The image still shows the earlier light look (lanes and the red confirmed jams, no bottleneck; decision 0016 leaves it to be redrawn). It is drawn from the published data by `web/og-card.html`, a page the build ignores. Redo it after the data or the look changes: start the dev server, then

  ```bash
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --hide-scrollbars --window-size=1200,630 --virtual-time-budget=6000 --screenshot=web/public/og.png http://localhost:5173/og-card.html
  ```

## 12. Acceptance checks
Run these as well as the checklists in `DESKTOP.md` §7 and `MOBILE.md` §9 before calling a change under `web/` done.

- A first-time visitor can tell the corridor, where the lanes run out and (after the analysis) the bottleneck and what the corridor can carry, without scrolling (§2).
- The map makes sense with no animation: flow hidden, and with reduced motion on (§6).
- Today's road and a tested fix cannot be confused; observed and computed marks cannot be confused (§3).
- "Not live traffic" is on screen in every view.
- Every finding on the map can also be reached in the panel, on desktop and on a phone (§8).
- Nothing that worked before is lost. Report any view, width or state you did not check.

## 13. The lane strip
- On the result, under the bottleneck: one bar per stretch of the part of the drive that every route uses, as tall as its lanes and as long as its kilometres, in travel order. It puts a wide road and the narrow stretch that limits it side by side.
- Same marks as the map: pale road, red bottleneck (also named in words), yellow nearly full. A broken bar and a "?" after the number mean the lane count is guessed.
- The title names the road and the length; one sentence below says what the strip shows (`profileTitle`, `profileSentence` in `web/src/copy.ts`). The lesson "more lanes on the wider parts would add nothing" is said only when the data shows it.
- It is drawn for the direction shown. If the traffic splits at once and no road is shared, the strip is left out.
- Everything comes from `edges.geojson` (`web/src/profile.ts`, pure and tested). Nothing about traffic is computed in the browser.
- It sits under the bottleneck behind "View lane layout", closed until asked for, on desktop and phones (decision 0018). It is the lane layout for the bottleneck: a road has only a lane count in the data, so the card's lane diagram (§14) cannot be drawn for it.

## 14. The map cards
Decision 0015, cards v2 (Tim, 2026-11-25; frames on page 5 of `design.fig`). One order on every card, top to bottom:
- **What it is**: the map's own mark and a label in capitals. A ringed red dot for a confirmed jam, a solid dot for a lane drop, a hollow dot where its lanes are guessed, and the road's line for a road (red for the bottleneck, amber for nearly full, after the analysis only). The label is light red where it jams (a confirmed jam or the bottleneck); the mark's shape tells which.
- **Its name**, after a route shield for each exact expressway number in the data (`E5`, `E11`; `expresswayRefs` in `copy.ts`). Exit numbers, federal routes and odd refs get none. With a shield, a road's name follows without repeating the ref.
- **The lane diagram** (lane drops only; `web/src/lanediagram.ts`, pure and tested): the roads coming in and the lanes going out, drawn as the zoomed-in map draws lanes, travel left to right so the driver's left is the top. Each road's lane count and the side it joins on come from `lane_drops.json` (`side`, from the OSM geometry). The lanes are lost on the side of the road that does not carry on; where the data does not say (every narrowing), both edges taper. A dashed edge means a count is guessed. It is decorative for screen readers: the lanes are said in words under it.
- **The lanes** in words ("5 → 3 lanes", "2 → 1 lane"), then a guessed flag with the key's dashed line, then for a road what it can carry ("Can carry about **6,000** veh/h").
- **Where the facts come from**, last: "Seen by a local driver", "From OpenStreetMap lane counts" or "Worked out by the analysis, not measured", with what happens there.
- **View details →**: the arrow points on, because the detail opens on this page, in the panel. ↗ is kept for links that leave the site. No divider above it: space separates one action.
- A card needs at least 320 × 420 px of map showing (on phones, the part above the sheet; the tallest card is about 360 px); less shows it in the sheet (on the desktop layout, in the panel: a short window or a phone on its side) with View details and Close, and a map that shrinks, or a sheet that grows, under an open card moves the card there. A card never sits under the sheet or the credit line.
