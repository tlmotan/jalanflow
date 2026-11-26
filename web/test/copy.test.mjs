// The site's sentences (src/copy.ts). Run with `npm test`.
import test from "node:test";
import assert from "node:assert/strict";
import {
  bottleneckSentence,
  confirmedSentence,
  dropSentence,
  dropShields,
  dropTitle,
  expresswayRefs,
  fixDifficulty,
  whatIfBars,
  fixSentence,
  fmtVph,
  lanesText,
  routeCaption,
  routeProgress,
  fmtExact,
  resultSentence,
  roadBody,
  roadNotes,
  roadTitle,
} from "../src/copy.ts";

const kesas = { id: 6641, ref: "E5", name: "Lebuhraya Shah Alam", class: "motorway", lanes: 3 };
const ldp = { id: 7, ref: "E11", name: "Lebuhraya Damansara-Puchong", class: "motorway", lanes: 2 };
const ramp = { id: 9, ref: "", name: "", class: "motorway_link", lanes: 1 };

const fix = (over) => ({
  edge_id: 6641,
  direction: "inbound",
  change: "+1 lane",
  gain_vph: 2000,
  new_max_flow_vph: 8000,
  next_cut_edges: [6641],
  difficulty: "medium",
  reasons: ["major highway: widening needs road reserve"],
  helps: true,
  ...over,
});

test("fmtVph rounds to the nearest 100 with separators", () => {
  for (const [n, want] of [[6000, "6,000"], [5949, "5,900"], [5950, "6,000"], [0, "0"], [12340, "12,300"]]) {
    assert.equal(fmtVph(n), want);
  }
});

test("resultSentence spells the unit out", () => {
  assert.equal(resultSentence(6000), "At most 6,000 vehicles an hour can get from Kota Kemuning to PJ.");
});

test("confirmedSentence", () => {
  const cases = [
    [0, ""],
    [1, "A local driver confirmed one of them as a daily jam."],
    [2, "A local driver confirmed two of them as daily jams."],
    [12, "A local driver confirmed 12 of them as daily jams."],
  ];
  for (const [n, want] of cases) assert.equal(confirmedSentence(n), want);
});

test("bottleneckSentence names the road from the data", () => {
  const cases = [
    { name: "no bottleneck", cut: [], want: "" },
    { name: "one road", cut: [kesas], want: "It runs out at one 3-lane stretch of E5 Lebuhraya Shah Alam." },
    {
      name: "two roads",
      cut: [kesas, ldp],
      want: "It runs out at two stretches taken together: E5 Lebuhraya Shah Alam (3 lanes) and E11 Lebuhraya Damansara-Puchong (2 lanes).",
    },
    {
      name: "two stretches of one road are named once",
      cut: [kesas, { ...kesas, id: 6642 }],
      want: "It runs out at two stretches taken together: E5 Lebuhraya Shah Alam (3 lanes).",
    },
    { name: "unnamed road falls back to its class", cut: [ramp], want: "It runs out at one 1-lane stretch of motorway_link." },
  ];
  for (const c of cases) assert.equal(bottleneckSentence(c.cut), c.want, c.name);
});

test("fixSentence", () => {
  const cases = [
    { name: "no fixes tested", fixes: [], cut: [kesas], want: "" },
    {
      name: "bottleneck stays put",
      fixes: [fix()],
      cut: [kesas],
      want: "One more lane there would raise the limit to 8,000, and that stretch would still be the bottleneck.",
    },
    {
      name: "bottleneck moves",
      fixes: [fix({ next_cut_edges: [7] })],
      cut: [kesas],
      want: "One more lane there would raise the limit to 8,000, and the bottleneck would move to another road.",
    },
    {
      name: "several bottleneck roads: the road is named and the biggest gain leads",
      fixes: [fix({ edge_id: 7, gain_vph: 500, new_max_flow_vph: 6500 }), fix()],
      cut: [kesas, ldp],
      want: "One more lane on E5 Lebuhraya Shah Alam would raise the limit to 8,000, and that stretch would still be the bottleneck.",
    },
    {
      name: "nothing helps alone",
      fixes: [fix({ helps: false, gain_vph: 0 })],
      cut: [kesas, ldp],
      want: "One more lane on any single bottleneck road would gain nothing: another road is just as tight.",
    },
  ];
  for (const c of cases) assert.equal(fixSentence(c.fixes, c.cut).join(""), c.want, c.name);
});

test("fixDifficulty gives the pipeline's reasons", () => {
  assert.equal(fixDifficulty([fix()]), "Rated medium to build: major highway: widening needs road reserve.");
  assert.equal(fixDifficulty([fix({ reasons: [] })]), "Rated medium to build.");
  assert.equal(fixDifficulty([fix({ helps: false })]), "");
});

test("lane drop wording", () => {
  const road = (over) => ({ osm_way_id: 1, name: "", ref: "", class: "motorway", lanes: 3, lanes_inferred: false, leaves: "", joins: "", ...over });
  const drop = (over) => ({
    rank: 1, key: "node:1", kind: "merge", pos: { lat: 3, lon: 101 }, lanes_in: 6, lanes_out: 3, lanes_lost: 3, squeeze: 0.5,
    from: [road({ class: "motorway_link", joins: "E5" }), road({ ref: "E5", name: "Lebuhraya Shah Alam" })],
    to: [road({ ref: "E5", name: "Lebuhraya Shah Alam" })],
    lanes_inferred: false, confirmed: null, ...over,
  });
  assert.equal(lanesText(drop()), "6 → 3 lanes");
  // 15 of the 87 drops end with one lane (2026-11-25): "2 → 1 lanes" read wrong.
  assert.equal(lanesText(drop({ lanes_in: 2, lanes_out: 1 })), "2 → 1 lane");
  assert.equal(dropTitle(drop()), "ramp onto E5 + E5 merge into E5");
  assert.equal(dropTitle(drop({ kind: "narrowing" })), "E5 narrows");
  assert.equal(dropTitle(drop({ confirmed: { name: "Sri Muda merge", source: "Tim" } })), "Sri Muda merge");
});

test("fmtExact keeps every vehicle, with separators", () => {
  for (const [n, want] of [[5450, "5,450"], [150, "150"], [0, "0"], [6000, "6,000"]]) assert.equal(fmtExact(n), want);
});

test("routeProgress names the route being traced, the last one too, then all", () => {
  assert.equal(routeProgress(0, 8), "Route 1 of 8");
  assert.equal(routeProgress(3, 8), "Route 4 of 8");
  assert.equal(routeProgress(7, 8), "Route 8 of 8"); // the last route, still being traced
  assert.equal(routeProgress(8, 8), "8 of 8 routes");
  assert.equal(routeProgress(9, 8), "8 of 8 routes");
});

test("routeCaption gives the route's real amount, and says when it moves an earlier one", () => {
  assert.equal(routeCaption(4, 900, false), "Route 4 has room for 900 vehicles an hour.");
  assert.equal(routeCaption(10, 150, true), "Route 10 has room for 150 vehicles an hour by moving part of an earlier route.");
  assert.equal(routeCaption(1, 1200, false), "Route 1 has room for 1,200 vehicles an hour.");
});

test("a tapped road: name, lanes, capacity, and the result only once the analysis has run", () => {
  const road = (over) => ({ ...kesas, lanes_inferred: false, capacity_vph: 6000, flow_vph: 6000, is_cut: false, ...over });
  assert.equal(roadTitle(road()), "E5 Lebuhraya Shah Alam");
  assert.equal(roadTitle(road({ ref: "", name: "", class: "motorway_link" })), "Ramp");
  assert.equal(roadTitle(road({ ref: "", name: "", class: "tertiary" })), "Unnamed road");
  assert.equal(roadBody(road()), "3 lanes, able to carry about 6,000 veh/h.");
  assert.equal(roadBody(road({ lanes: 1, capacity_vph: 1200 })), "1 lane, able to carry about 1,200 veh/h.");

  const cases = [
    { name: "before the run nothing about the result is said", r: road({ is_cut: true }), solved: false, cuts: 1, want: [] },
    { name: "guessed lanes are always flagged", r: road({ lanes_inferred: true }), solved: false, cuts: 1, want: [/guessed/] },
    { name: "the bottleneck says why", r: road({ is_cut: true }), solved: true, cuts: 1, want: [/This is the bottleneck\. Every route/] },
    { name: "one of several bottleneck roads", r: road({ is_cut: true }), solved: true, cuts: 2, want: [/part of the bottleneck/] },
    { name: "a road with room", r: road({ flow_vph: 1100 }), solved: true, cuts: 1, want: [/sends 1,100 veh\/h along it, so it has room to spare/] },
    { name: "a full road that is not the bottleneck", r: road(), solved: true, cuts: 1, want: [/nearly full\. It is not what sets the limit/] },
    { name: "just under nearly full", r: road({ flow_vph: 5399 }), solved: true, cuts: 1, want: [/room to spare/] },
    { name: "exactly nearly full", r: road({ flow_vph: 5400 }), solved: true, cuts: 1, want: [/nearly full/] },
    { name: "a road carrying nothing", r: road({ flow_vph: 0 }), solved: true, cuts: 1, want: [] },
  ];
  for (const c of cases) {
    const got = roadNotes(c.r, c.solved, c.cuts);
    assert.equal(got.length, c.want.length, c.name);
    c.want.forEach((re, i) => assert.match(got[i], re, c.name));
  }
});

test("route shields only for exact expressway numbers", () => {
  assert.deepEqual(expresswayRefs("E5"), ["E5"]);
  assert.deepEqual(expresswayRefs("E1;E5", "E5"), ["E1", "E5"]);
  for (const ref of ["", "507", "2", "E6 I/C-SEA RAMP E", "e5"]) assert.deepEqual(expresswayRefs(ref), [], ref);
});

test("a lane drop's shields: the road it carries on into, else where it comes from", () => {
  const road = (ref) => ({ osm_way_id: 1, name: "", ref, class: "motorway", lanes: 3, lanes_inferred: false, leaves: "", joins: "", side: "" });
  const drop = (from, to) => ({ kind: "merge", from: from.map(road), to: to.map(road) });
  assert.deepEqual(dropShields(drop(["E5", "507"], ["E5"])), ["E5"]);
  assert.deepEqual(dropShields(drop(["E5"], [""])), ["E5"]);
  assert.deepEqual(dropShields(drop([""], [""])), []);
});

test("a lane drop's sentence names the side only when the data says", () => {
  assert.equal(dropSentence({ kind: "merge" }, "left"), "A road joins from the left and lanes are lost.");
  assert.equal(dropSentence({ kind: "merge" }, null), "Roads join here and lanes are lost.");
  assert.equal(dropSentence({ kind: "narrowing" }, null), "The road loses lanes here.");
});

// The what-if bars (decision 0018): two bars from the same zero on one scale.
test("whatIfBars", () => {
  const cases = [
    { name: "a gain", now: 6000, then: 8000, want: { now: 0.75, then: 1, gain: 0.25 } },
    { name: "no gain: equal bars", now: 6000, then: 6000, want: { now: 1, then: 1, gain: 0 } },
    { name: "a loss is drawn shorter, never as a gain", now: 6000, then: 3000, want: { now: 1, then: 0.5, gain: 0 } },
    { name: "no baseline: no bars, no division by zero", now: 0, then: 2000, want: null },
    { name: "a missing number: no bars", now: NaN, then: 2000, want: null },
    { name: "a missing result: no bars", now: 6000, then: NaN, want: null },
  ];
  for (const c of cases) assert.deepEqual(whatIfBars(c.now, c.then), c.want, c.name);
});
