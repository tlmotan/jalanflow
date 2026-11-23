# DESIGN — how the site looks, moves and behaves

Read this (and `CONTENT.md` for wording) before any change under `web/`. Design decisions with a story behind them are ADRs: 0004 (lane drops headline), 0010 (no landing page), 0011 (animation honesty), 0012 (lanes and cars), 0013 (the look: light page, ink dots, one red).

**The target look is `docs/design/light-look-mockup.html`** (decision 0013). Open it in a browser before building anything visual. Where the code still differs from this document, the document is the target; 0013 lists the build order.

## 1. Page structure
- **One page**: full-screen map + one text column. No separate landing page until Phase 2 adds several corridors; then a "pick your commute" page sits in front (decision 0010).
- **No card.** The text sits straight on the page: no panel background, border or shadow. Top-left on desktop, below the map on phones.
- **One view at a time.** The column shows exactly one of:
  - **Landing**: orb and name, headline, one sentence, "Run the analysis", the three-dot key, links to "See all lane drops" and "How this works".
  - **Running**: the live counter, "Path n of m", "Skip to the result".
  - **Result**: the result sentence, the bottleneck, the tested fix, "Replay", "Hide flow", and the same two links as the landing view.
  - **All lane drops**, **one lane drop** and **How this works**: each replaces the column and has a Back button. Escape also goes back.
- No tabs and no map popups. Clicking a marker or a road opens its detail in the column.
- **Landing shows lanes only.** The drive is a grey line. The bottleneck turns red after "Run the analysis", because the lane-drop finding leads (decision 0004).
- **The map stays readable.** Zoomed out, only the confirmed jam spots and the 10 biggest squeezes get a marker. The rest appear from zoom 13.5, or all at once while the lane-drop list is open.

## 2. Ten-second rule
A commuter opening the site cold must get *where it jams and why* without scrolling or clicking. Anything that delays that (intro screens, autoplay, modals) is out.

## 3. Honesty
- Every visual says what it measures: **lanes**, **capacity** (max flow) or, from Phase 1.5, **simulated traffic**. Never live traffic.
- **No animation shows something the model didn't compute** (decision 0011). Max flow is a steady state: nothing queues, so the site never draws pile-ups from it. Queues appear only when the CTM simulator computes them.
- Guessed lane counts are always visibly marked (dashed lines, hollow dots; a dashed road edge once lanes are drawn).
- Lanes on the map are schematic: equal stripes from the OSM lane count, not surveyed lane positions (decision 0012).
- Phase 1.5 results always carry their confidence label.

## 4. Colour
Ink on a light page, and one red. The same dot is the only mark: lane drops, flow and the orb.

Defined once: CSS variables in `web/src/style.css` (`:root`). `web/src/colors.ts` reads them from the page for MapLibre paint; it does not repeat the hex values. Never hard-code a colour elsewhere.

| Token | Hex | Meaning |
|---|---|---|
| `--bg` | `#f6f7f7` | the page; also the knock-out behind markers and map labels |
| `--ink` | `#15181b` | text, the drive, lane-drop markers, flow dots, the route being traced, the orb, the primary button |
| `--muted` | `#5d646b` | secondary text |
| `--road` | `#dcdfe2` | modelled roads that are not on the drive |
| `--line` | `#c5cace` | borders of secondary buttons |
| `--cut` | `#d7261e` | **the bottleneck (min cut), and nothing else** |
| `--marker` | `#ffd100` | expressway route markers only |

- **No utilisation colours.** Spare versus nearly full is shown by how many dots flow along a road. The old `--spare`, `--full`, `--idle` and `--path` tokens are retired.
- **Lane-drop markers are ink, never red:** ringed dot = confirmed jam, solid dot = lane drop, hollow dot = lane count guessed.
- **Route markers**: KESAS and LDP carry the yellow hexagon "E5" / "E11" used on Malaysian expressway signs, ink outline and ink number.
- Not decided yet (0013): the basemap, and road and car colours once lanes are drawn (decision 0012). Both are built as trial options in `web/src/trial.ts`, switched by URL parameter: `?base=none|pale|labels` and `?lanes=pale|ink`. The defaults are `pale` and `pale`. When Tim picks, delete the losing options, the parameters and the `--map-*` tokens.
- Labels on the map that use the road-sign face (place names, route markers, "Confirmed jam", "Bottleneck, 3 lanes") are HTML markers, not map text layers: a map text layer can only use the tile server's fonts.
- An expressway gets a route marker once it carries at least 3 km of the drive, so E1 and E6 show one as well as E5 and E11.

## 5. Type and layout
- **Road-sign face** for the headline, result sentence, counter, place names and route markers: `"DIN Alternate", "Bahnschrift", "Roboto Condensed", "Arial Narrow", system-ui`, bold. These ship with the device, so there is no font file and no dependency.
- **Body**: `system-ui`.
- Sizes: headline and result sentence 27 px (22 on phones), running counter 64 px (48 on phones), body 15 px, key and notes 12.5 px, map labels 12 px, place names 14 px.
- **The result is a sentence, not a big number with a caption.** The large counter appears only while the analysis runs, where it is counting.
- Desktop: text column 372 px wide, 44 px from the left, 36 px from the top. Phones (≤ 700 px): map above, text column below with 20 px side margins; the map is sized to the space the text leaves.
- Buttons are 44 px tall with an 8 px radius. Primary = ink fill. Secondary = `--line` border. Text links are underlined.
- Map fitting and flying always keep clear of the text column, measured from the column itself, not from constants.

## 6. Motion
- Nothing moves until the user asks. No autoplay.
- Every animation has **Skip** and finishes in **≤ 15 s** whatever the data size.
- `prefers-reduced-motion: reduce` → jump straight to the end state; no moving dots.
- Background loops (flow dots and cars) pause when the tab is hidden or the toggle is off.
- **The orb** (the logo beside the name) is ordered and turning slowly at rest. When the analysis starts it scatters, then tightens as flow is found: disorder = 1 − found ÷ max. It is our own canvas code and never sits on the map, so it cannot be mistaken for a computed result (decision 0011). With reduced motion it is still.
- Flow is dots when zoomed out and cars in lanes from zoom 14 (decision 0012). All vehicles move at one speed, set in screen pixels so it reads the same at every zoom.

## 7. Copy
Plain language, local road names, "veh/h". Short sentences; caveats once. Full rules: `CONTENT.md`.

## 8. Accessibility
- Every control is a real `<button>` (or a link) with a visible `:focus-visible` ring in ink. Opening a view moves focus to its Back button; going back returns focus to where it was.
- The lane-drop list is the keyboard route to every marker, since map markers cannot take focus.
- Text contrast at least WCAG AA.
- Colour is never the only signal: the bottleneck also gets width, a pulse and a label; marker types differ by shape (ring, solid, hollow); guessed lanes also get dashes.
- Status changes during the replay (counter, captions) go in an `aria-live="polite"` region.

## 9. Performance
- First meaningful map in < 2 s on 4G. Data files are compact JSON; only flow-carrying edges are exported.
- Animations stay smooth on a mid-range phone: ≤ 3k vehicles, `setData` at ≤ 30 fps, and only vehicles on screen are sent to the map.

## 10. Constraints
- Plain DOM + MapLibre GL, TypeScript strict. No framework, no new dependencies.
- OSM attribution always visible.
- Display-only constants (dot density, timings) may live in TS, named and commented. Capacity, speed and traffic numbers never do (they live in `config/`).

## 11. Shareability (planned)
Deep links per finding (`#drop-21`) and a title / preview image so links look good on WhatsApp and X.
