// The site's sentences (src/copy.ts). Run with `npm test`.
import test from "node:test";
import assert from "node:assert/strict";
import {
  bottleneckSentence,
  confirmedSentence,
  dropTitle,
  fixDifficulty,
  fixSentence,
  fmtVph,
  lanesText,
  pathCaption,
  resultSentence,
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
  assert.equal(dropTitle(drop()), "ramp onto E5 + E5 merge into E5");
  assert.equal(dropTitle(drop({ kind: "narrowing" })), "E5 narrows");
  assert.equal(dropTitle(drop({ confirmed: { name: "Sri Muda merge", source: "Tim" } })), "Sri Muda merge");
});

test("pathCaption never counts past the last path", () => {
  assert.equal(pathCaption(1, 8), "Path 1 of 8.");
  assert.equal(pathCaption(9, 8), "Path 8 of 8.");
});
