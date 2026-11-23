# DESIGN — how the site looks, moves and behaves

Read this (and `CONTENT.md` for wording) before any change under `web/`. Design decisions with a story behind them are ADRs: 0004 (lane drops headline), 0010 (no landing page), 0011 (animation honesty).

## 1. Page structure
- **One page**: full-screen map + side panel. No separate landing page until Phase 2 adds several corridors; then a "pick your commute" page sits in front (decision 0010).
- **Finding first.** Panel order: headline → run-the-analysis controls → confirmed jam spots → top fixes → all lane drops → "How this works". Caveats live in "How this works", not at the top.

## 2. Ten-second rule
A commuter opening the site cold must get *where it jams and why* without scrolling or clicking. Anything that delays that (intro screens, autoplay, modals) is out.

## 3. Honesty
- Every visual says what it measures: **lanes**, **capacity** (max flow) or, from Phase 1.5, **simulated traffic**. Never live traffic.
- **No animation shows something the model didn't compute** (decision 0011). Max flow is a steady state: nothing queues, so the site never draws pile-ups from it. Queues appear only when the CTM simulator computes them.
- Guessed lane counts are always visibly marked (dashed lines, hollow dots).
- Phase 1.5 results always carry their confidence label.

## 4. Colour
One meaning per colour, used the same way on map, legend, dots and panel. Defined once: CSS variables in `web/src/style.css` (`:root`) and the `COLORS` block in `web/src/colors.ts` for MapLibre paint. Never hard-code a colour elsewhere.

| Token | Hex | Meaning |
|---|---|---|
| `--spare` | `#4a90a4` | road with spare capacity |
| `--full` | `#ff9f1c` | nearly full (≥ 90 % utilisation); lane drop |
| `--cut` | `#d62728` | bottleneck (min cut); locally confirmed jam |
| `--idle` | `#9aa5b1` | not carrying flow yet (replay start) |
| `--path` | `#1f3b57` | route being pushed right now (replay), drawn as a soft glow; also the "Run the analysis" controls and counter. Not white: the basemap is light |
| `--ink` / `--muted` | `#222` / `#555` | body text / notes |

## 5. Type and layout
- `system-ui`. Sizes 18 / 15 / 13 / 12 px for h1 / h2 / body / notes.
- Desktop: panel 340 px wide, top-left, 12 px from the edges. Phones (≤ 600 px): bottom sheet, at most 45 % of the height.
- Map padding always accounts for the panel (left 380 px on desktop, sheet height on phones) when fitting or flying.

## 6. Motion
- Nothing moves until the user asks. No autoplay.
- Every animation has **Skip** and finishes in **≤ 15 s** whatever the data size.
- `prefers-reduced-motion: reduce` → jump straight to the end state; no moving dots.
- Background loops (flow dots) pause when the tab is hidden or the toggle is off.

## 7. Copy
Plain language, local road names, "veh/h". Short sentences; caveats once. Full rules: `CONTENT.md`.

## 8. Accessibility
- Every control is a real `<button>` with a visible `:focus-visible` ring.
- Text contrast at least WCAG AA.
- Colour is never the only signal: the bottleneck also gets width and a pulse; guessed lanes also get dashes.
- Status changes during the replay (counter, captions) go in an `aria-live="polite"` region.

## 9. Performance
- First meaningful map in < 2 s on 4G. Data files are compact JSON; only flow-carrying edges are exported.
- Animations stay smooth on a mid-range phone: ≤ ~3k dots, `setData` at ≤ 30 fps.

## 10. Constraints
- Plain DOM + MapLibre GL, TypeScript strict. No framework, no new dependencies.
- OSM attribution always visible.
- Display-only constants (dot density, timings) may live in TS, named and commented. Capacity, speed and traffic numbers never do (they live in `config/`).

## 11. Shareability (planned)
Deep links per finding (`#drop-21`) and a title / preview image so links look good on WhatsApp and X.
