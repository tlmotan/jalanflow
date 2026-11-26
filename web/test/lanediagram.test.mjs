// The lane diagram on a lane drop's card (src/lanediagram.ts). Run with `npm test`.
import test from "node:test";
import assert from "node:assert/strict";
import { DIAGRAM, joiningSide, laneDiagram } from "../src/lanediagram.ts";

const road = (over) => ({ osm_way_id: 1, name: "", ref: "", class: "motorway", lanes: 3, lanes_inferred: false, leaves: "", joins: "", side: "", ...over });
const e5 = (over) => road({ ref: "E5", name: "Lebuhraya Shah Alam", ...over });
// Drop #21 as published: E5 (3) on the right, exit 507 (2) joining on the left, E5 (3) out.
const sriMuda = (over) => ({
  rank: 21, key: "node:33987957", kind: "merge", pos: { lat: 3.04, lon: 101.56 }, lanes_in: 5, lanes_out: 3, lanes_lost: 2, squeeze: 0.6,
  from: [e5({ side: "right" }), road({ ref: "507", name: "Persimpangan Hicom", class: "motorway_link", lanes: 2, side: "left" })],
  to: [e5()],
  lanes_inferred: false, confirmed: null, ...over,
});
const narrowing = (over) => ({
  rank: 31, key: "way:1>2", kind: "narrowing", pos: { lat: 3, lon: 101 }, lanes_in: 3, lanes_out: 2, lanes_lost: 1, squeeze: 2 / 3,
  from: [road({ class: "motorway_link" })], to: [road({ class: "motorway_link", lanes: 2 })],
  lanes_inferred: false, confirmed: null, ...over,
});

const { top: T, lane: L, width: W } = DIAGRAM;
const through = (dg) => dg.lines.filter((l) => l.x2 === W).map((l) => l.y1).sort((a, b) => a - b);

test("the joining road's side comes from the data, and only when it is clear", () => {
  const cases = [
    { name: "exit 507 joins on the left", d: sriMuda(), want: "left" },
    { name: "mirror: joins on the right", d: sriMuda({ from: [e5({ side: "left" }), road({ class: "motorway_link", lanes: 2, side: "right" })] }), want: "right" },
    { name: "narrowing: OSM does not say which lane ends", d: narrowing(), want: null },
    { name: "sides unknown", d: sriMuda({ from: [e5(), road({ class: "motorway_link", lanes: 2 })] }), want: null },
    { name: "no road carries on by ref", d: sriMuda({ to: [road({ ref: "E11" })] }), want: null },
    { name: "two ramps meeting", d: sriMuda({ from: [road({ class: "motorway_link", side: "right" }), road({ class: "motorway_link", lanes: 2, side: "left" })], to: [road({ class: "motorway_link" })] }), want: null },
  ];
  for (const c of cases) assert.equal(joiningSide(c.d), c.want, c.name);
});

test("a merge from the left: lanes stacked left to right, the left edge tapers", () => {
  const dg = laneDiagram(sriMuda());
  assert.equal(dg.inLabel, "2 + 3");
  assert.equal(dg.outLabel, "3");
  assert.equal(dg.joinSide, "left");
  // 5 lanes in, the top two lost: the road ends between T + 2 lanes and T + 5 lanes.
  assert.match(dg.road, new RegExp(`L${W} ${T + 2 * L} L${W} ${T + 5 * L}`));
  // The two lines between the three E5 lanes run through; the ramp's own line stops.
  assert.deepEqual(through(dg), [T + 3 * L, T + 4 * L]);
  const gore = dg.lines.filter((l) => l.kind === "gore");
  assert.equal(gore.length, 1);
  assert.equal(gore[0].y1, T + 2 * L, "the gore sits between the ramp (2) and E5 (3)");
  assert.ok(dg.lines.find((l) => l.y1 === T + L).x2 < DIAGRAM.taperEnd, "the ramp's lane line ends at the taper");
});

test("a merge from the right tapers the bottom edge", () => {
  const dg = laneDiagram(sriMuda({ from: [e5({ side: "left" }), road({ class: "motorway_link", lanes: 2, side: "right" })] }));
  assert.equal(dg.inLabel, "3 + 2");
  assert.match(dg.road, new RegExp(`L${W} ${T} L${W} ${T + 3 * L}`));
  assert.deepEqual(through(dg), [T + L, T + 2 * L]);
});

test("a narrowing tapers both edges and draws the out lanes fresh", () => {
  const dg = laneDiagram(narrowing());
  assert.equal(dg.inLabel, "3");
  assert.equal(dg.outLabel, "2");
  assert.equal(dg.joinSide, null);
  // Half a lane off each side: out lanes from T + 5 to T + 25.
  assert.match(dg.road, new RegExp(`L${W} ${T + L / 2} L${W} ${T + 2.5 * L}`));
  // No incoming line runs through (none lines up); one out line starts after the taper.
  const out = dg.lines.filter((l) => l.x2 === W);
  assert.equal(out.length, 1);
  assert.equal(out[0].y1, T + 1.5 * L);
  assert.equal(out[0].x1, DIAGRAM.taperEnd);
  assert.ok(dg.lines.every((l) => l.x2 === W || l.x2 <= DIAGRAM.taperStart), "incoming lines stop at the taper");
});

test("guessed lane counts give a dashed edge; a drop to one lane draws no out line", () => {
  assert.equal(laneDiagram(sriMuda({ lanes_inferred: true })).guessed, true);
  const dg = laneDiagram(narrowing({ lanes_in: 2, lanes_out: 1, from: [road({ lanes: 2 })], to: [road({ lanes: 1 })] }));
  assert.equal(dg.lines.filter((l) => l.x2 === W).length, 0);
  assert.equal(dg.height, T + 2 * L + DIAGRAM.bottom);
});
