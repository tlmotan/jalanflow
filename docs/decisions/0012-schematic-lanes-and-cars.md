# 0012 — Lanes are drawn schematically; cars appear when zoomed in

- Status: accepted · 2026-11-24
- Context: The flow dots (decision 0011) show how much each road can carry, but a 5-lane road and a 2-lane road both look like one string of dots. Tim wants to see the lanes, with cars in each. OSM gives a lane **count** per road and one centre line per carriageway; it does not say where each lane runs. At the whole-corridor view a 5-lane road is about 7 px wide, so a car per lane would be one pixel.
- Decision:
  - **Zoomed out (below zoom 13)** nothing changes: one line per road, dots on it.
  - **Zoomed in (from zoom 14)** the road is drawn `lanes × lane width` wide with a dark edge and white dashed lane lines, and the dots become small cars, one stream per lane. Zoom 13–14 cross-fades between the two.
  - Lanes are **equal stripes either side of the OSM centre line**. Lane width in pixels comes from one table (`web/src/lanes.ts`) used by the road width, the lane lines and the car positions. It is about 3× a real 3.5 m lane at zoom 14 and close to true scale from zoom 17.
  - Cars are the **same vehicles** as the dots: the exported max flow is split into gate-to-gate routes that add up to `flow_vph` on every road (`web/src/routes.ts`), and vehicles are placed in proportion to flow × length. More are shown as you zoom in; the share shown is the same for every road, so density always follows flow.
  - Each car keeps one lane share for its whole trip (lane 0 is the left lane; Malaysia drives on the left). Where the lane count changes it slides across on the wider road. Cars in one lane are spaced so they never overlap. This is drawing only.
  - **Decision 0011 still holds:** one speed everywhere, no queues, no slowing at lane drops. The note under the button says so.
  - Guessed lane counts stay marked: dashed line when zoomed out, dashed road edge when lanes are drawn.
- Consequences: web only; no output format change. A road's lanes on screen can sit a few metres off the real carriageway and off each other at junctions. Cars are drawn as short line features, not icons, so they update as cheaply as the dots (about 28 updates a second measured with ~500 cars in view). Two-way roads are drawn centred on the OSM line, not on their own side.
