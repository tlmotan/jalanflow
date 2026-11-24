# OPENPENCIL.md — driving the design canvas from an agent

> Steps 3 and 4 of `../WORKFLOW.md` when the tool is OpenPencil (open-source design editor). Everything here is about the tool; what to design comes from the guides and `../project/DESIGN_BRIEF.md`.

## Setup

- OpenPencil runs as a desktop app; its MCP server (`openpencil`) is registered for this project in local scope. Its tools are named `mcp__openpencil__*`.
- If the tools are missing or cannot reach the app, **stop and tell Tim**. Do not fall back to another design tool or to hand-written HTML mock-ups.
- The document is `../project/design.fig`. Open it with `open_document`, and always save with an explicit path (`save_document` with `path`): a tab can remember an older location.

## The document

| Page | Holds |
|---|---|
| 0 Components and tokens | Map layers, cards, markers, controls, the token sheet with proposed changes |
| 1 Desktop baseline | The current trial page, drawn faithfully |
| 2 Desktop proposed | Proposed desktop frames, with a notes frame |
| 3 Mobile proposed | Proposed phone frames (390 × 844), with a notes frame |
| 4 Run analysis states | Every state of Run the analysis the code supports |

- **Frame names** say status, platform, width and state: `Proposed / Desktop 1440 / B. Confirmed jam card beside its marker`.
- **Tokens** are the `jalanflow` variable collection, imported from `web/src/trial/trial.css` as a W3C design-token file (`import_design_tokens`). Proposed tokens carry `PROPOSED:` in their description. Re-import after `trial.css` changes.
- **Map layers are components built from the real data.** Project `web/public/data/kemuning-pj/edges.geojson` with Web Mercator into the frame size, simplify to whole pixels, and draw each layer (quiet roads, drive, guessed lanes, nearly full, bottleneck, traced routes) as one `<Vector>`. Lane-drop markers sit at their projected `pos`. Screens use instances, so one fix updates every frame.
- **Map controls and credit are a separate "chrome" component**, so a panned map keeps them fixed to the map box.
- Mock-up limits to state in the notes: no basemap streets, no moving flow dots, Inter standing in for `system-ui`, Menlo for `ui-monospace`.

## Working with `render` (JSX)

- `lineHeight` is in **pixels** (`lineHeight={20}`), not a multiplier; `1.3` gives text 1.3 px tall.
- Wrapping text needs `w="fill"` (or a fixed `w`) inside an auto-layout frame.
- `mt`, `autoResize` and `textTransform` are ignored. Use spacer frames (`<Frame w={1} h={16} />`) for gaps, and write capitals into the text itself.
- A `<Vector d="…">` rendered into a frame keeps the path's coordinates. Re-rendering it with `replace_id` keeps the **old** node's position instead, so delete and render fresh.
- Resizing an instance does not scale its contents: make a separate component at the new size.
- `reparent_node` appends at the end of the new parent; reparent the later siblings again to restore order, and use it to bring an overlay to the top.

## Checking (step 4)

1. `lint` per page with the `recommended` preset, then `lint_fix` to bind colours to tokens and round to whole pixels.
2. `lint` with the `accessibility` preset; fix contrast, text under 12 px and touch targets on controls.
3. Known, accepted flags: map tags (they are not interactive), 32 px desktop links and the 40 px desktop direction switch (WCAG 2.2 allows 24 px; both are 44 px on touch, as in `trial.css`), and `999px` pill radii.
4. Look at every frame with `export_image` before calling it done, then write the review PNGs with `export_image` and `path` into `screenshots/openpencil/` (scale 2 for screens).
