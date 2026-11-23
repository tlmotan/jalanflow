# 0013 — The site's look: light page, ink dots, one red

- Status: accepted · 2026-11-24 (Tim: "lock it in for now, there can always be improvements")
- Reference: `docs/design/light-look-mockup.html`. Open it in a browser; it is one file with a snapshot of the real corridor data inside. It is a picture of the target, not code to copy: it draws on a plain canvas with no MapLibre.
- Context: The panel-and-legend design read as a stock layout. Two measures (lanes and capacity) shared red and orange, so red meant both "bottleneck" and "confirmed jam", and five colours needed a legend. Tim liked the "solving" thinking-orb indicator: a cloud of dots that settles into order, which is what the max-flow search does. We tried a dark version and a light one; Tim chose light.
- Decision:
  - **One mark.** Lane drops, flow and the orb are all the same ink dot at different sizes. No fills, shadows or card around the text: the text column sits straight on the page, top-left on desktop, below the map on phones.
  - **Colour.** Page `#f6f7f7`, ink `#15181b`, muted text `#5d646b`, quiet roads `#dcdfe2`, control borders `#c5cace`. **Red `#d7261e` means the bottleneck and nothing else.** Yellow `#ffd100` appears only in route markers. Spare versus nearly full is shown by how many dots flow, not by teal and amber.
  - **Lane-drop markers are ink, not red or orange:** ringed dot = confirmed jam, solid dot = lane drop, hollow dot = lane count guessed.
  - **Type.** Headline, counter, place names and route markers use the DIN-style road-sign face already on the device: `"DIN Alternate", "Bahnschrift", "Roboto Condensed", "Arial Narrow", system-ui`. Body text is `system-ui`. No font files, so no new dependency.
  - **Route markers.** KESAS and LDP carry the yellow hexagon "E5" / "E11" marker used on Malaysian expressway signs.
  - **The orb** is the logo, next to the name. It is ordered at rest, scatters when the analysis starts and tightens as flow is found (disorder = 1 − found ÷ max). It is written in our own canvas code. The `thinking-orbs` package is not used: it needs React.
  - **One text column, one view at a time:** landing → running → result, plus "all lane drops", one lane drop, and "How this works", each replacing the column with a Back button (Escape also goes back). No tabs, no map popups.
  - **Landing shows lanes only.** The drive is a grey line; the bottleneck turns red after "Run the analysis".
  - **The result is a sentence**, not a big number with a caption: "At most 6,000 vehicles an hour can get from Kota Kemuning to PJ." The large counter appears only while the analysis runs.
- What this replaces:
  - `DESIGN.md` §1 (card with three tabs), §4 (colour table) and §5 (type sizes, 340 px panel), rewritten in the same change as this record, with smaller edits to §6 and §8. `CONTENT.md` "Labels" loses the tab rule.
  - PRD user story 1 says the bottleneck is red "immediately". Here it is red after the run, because the lane-drop finding leads (decision 0004).
  - Decisions 0011 (animation shows only computed results) and 0012 (lanes and cars when zoomed in) still hold. The orb is not on the map, so it does not break 0011.
- Not decided yet:
  - **Basemap.** The mockup has none. The real map needs enough context to zoom in and tap a road; a very pale greyscale basemap is the likely answer, not yet checked against the tile source.
  - **Road and car colours when zoomed in** (0012), now that teal and amber are gone.
  - **Which lane drops count.** The mockup shows the 24 that sit on a published road; `lane_drops.json` has 87.
- Needs the pipeline (not to be faked in `web/`):
  - a common road name ("KESAS", "LDP") beside the ref, for titles and the result sentence;
  - something to tell repeated rows apart ("Ramp onto LDP" appears five times), such as the nearest exit;
  - which lane drops are on the published drive, so the count and the headline direction are true.
- Build order (each step ships on its own):
  1. Tokens: CSS `:root` is the only place a colour lives; `colors.ts` reads from it. New palette, road-sign type stack, focus ring.
  2. Text column replaces the panel: landing, running and result views; result as a sentence; orb.
  3. Map restyle: grey drive, ink markers, red bottleneck only, route markers, flow dots in ink.
  4. List, detail and "How this works" views in the column; map and list highlight each other; tap a road.
  5. Phone layout: map above, text below, map sized to the space left.
  6. Deep links and share preview.
- Consequences: web only, no output format change until the pipeline fields above are added. The site loses its at-a-glance utilisation colours; anyone wanting per-road capacity taps the road. The road-sign face differs slightly by device (DIN Alternate on Apple, Bahnschrift on Windows, a condensed fallback elsewhere).
