# 0014 — Red for jams, yellow for nearly full roads

- Status: accepted · 2026-11-24 (Tim, after seeing it on the real site at desktop and phone width)
- Context: Decision 0013 kept one red, for the bottleneck only, and drew everything else in ink. On the built site the result was one short red stretch, and the two jams a driver had confirmed looked like every other lane drop. Tim: "it'll be more impactful if you show which spots are confirmed jams/clogged (red), and high traffic (yellow)".
- Decision:
  - **Red (`--cut`) means where it jams**, in two shapes. A red ringed dot is a jam a driver confirmed. A thick red line is the bottleneck the analysis found. Each keeps its text label, so observed and computed are not confused.
  - **Yellow (`--full`, `#f2b200`) means nearly full**: a road the analysis fills to 90 % or more of what it can carry, the bottleneck excluded (60 of 420 stretches today). It is a band under the ink line, and the road edge when zoomed in. The route-marker yellow `#ffd100` was too faint for a line on the page.
  - **It is called "nearly full", never "high traffic".** The site has no traffic data; the footer still says so.
  - **When each appears.** Confirmed jams are red from the landing view, because they are observations. The yellow roads and the red bottleneck appear only after "Run the analysis" (decisions 0004, 0011).
  - **The result view gets a one-line key**: bottleneck, nearly full, confirmed jam, moving dots ("more dots, more vehicles"), lane drop. Tim noticed the black dots were not explained.
  - Tapping a nearly full road says it is nearly full and that it is not what sets the limit. It used to say "room to spare".
- What this replaces: the colour rule of 0013 and `design/project/DESIGN_BRIEF.md` §4 ("red means the bottleneck and nothing else", "lane-drop markers are ink, never red", "no utilisation colours"). Everything else in 0013 holds.
- Options Tim turned down: yellow from 70 % (77 stretches, a weaker claim per road); yellow lane-drop dots instead of roads; red for confirmed jams only; all colour on the landing view; all colour only after the run.
- Consequences: web only, no output format change. A max flow is one of several ways to fill the roads, so which roads come out nearly full can change when the data is refreshed. The 90 % line is `NEARLY_FULL` in `web/src/copy.ts`: a display threshold, not a capacity factor. The share image was redrawn with red jam dots.
