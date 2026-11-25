// The lane strip (src/profile.ts) and its wording (src/copy.ts). Run with `npm test`.
import test from "node:test";
import assert from "node:assert/strict";
import { sharedRoads, stretches } from "../src/profile.ts";
import { profileLabel, profileSentence, profileTitle, sharedName } from "../src/copy.ts";

// A road from x to x+1 along the equator-ish line; `y` moves it onto a side road.
let nextId = 1;
const road = (from, to, lanes, flow, over = {}) => ({
  id: nextId++,
  coords: [from, to],
  ref: "E5",
  name: "Lebuhraya Shah Alam",
  class: "motorway",
  lanes,
  lanes_inferred: false,
  capacity_vph: lanes * 2000,
  flow_vph: flow,
  is_cut: false,
  ...over,
});
const A = [101.5, 3], B = [101.51, 3], C = [101.52, 3], D = [101.53, 3], E = [101.54, 3], S = [101.53, 3.01];

test("sharedRoads follows the road carrying everything until the traffic splits", () => {
  const ab = road(A, B, 3, 6000, { is_cut: true });
  const bc = road(B, C, 5, 6000);
  const cd = road(C, D, 3, 6000);
  const de = road(D, E, 3, 5000); // 1,000 have left on the side road
  const ds = road(D, S, 1, 1000);
  // Order in the file must not matter.
  const got = sharedRoads([ds, de, cd, ab, bc], 6000);
  assert.deepEqual(got.map((r) => r.id), [ab.id, bc.id, cd.id]);
});

test("sharedRoads is empty when there is nothing to follow", () => {
  const cases = [
    { name: "no roads", roads: [], max: 6000 },
    { name: "no flow", roads: [road(A, B, 3, 0)], max: 6000 },
    { name: "max flow unknown", roads: [road(A, B, 3, 6000)], max: 0 },
    { name: "traffic splits at the start", roads: [road(A, B, 2, 4000), road(A, S, 1, 2000)], max: 6000 },
  ];
  for (const c of cases) assert.deepEqual(sharedRoads(c.roads, c.max), [], c.name);
});

test("sharedRoads stops rather than going round a loop for ever", () => {
  const got = sharedRoads([road(A, B, 3, 6000), road(B, C, 3, 6000), road(C, B, 3, 6000)], 6000);
  assert.equal(got.length, 3); // each road once, then it is back at a place it has been
});

test("stretches joins neighbours with the same lanes and mark, and keeps the marks apart", () => {
  const path = [
    road(A, B, 3, 6000, { is_cut: true }),
    road(B, C, 3, 6000), // same lanes, but nearly full rather than the bottleneck
    road(C, D, 3, 6000, { lanes_inferred: true }),
    road(D, E, 5, 6000),
  ];
  const got = stretches(path).map((s) => [s.lanes, s.cut, s.full, s.guessed]);
  assert.deepEqual(got, [
    [3, true, false, false],
    [3, false, true, true],
    [5, false, false, false],
  ]);
  const km = stretches(path).map((s) => s.km);
  assert.ok(Math.abs(km[1] - 2 * km[0]) < 1e-6, "a joined stretch is as long as its roads");
});

test("stretches calls a road nearly full from 90 % of what it can carry", () => {
  const cases = [
    { flow: 5400, full: true },
    { flow: 5399, full: false },
  ];
  for (const c of cases) assert.equal(stretches([road(A, B, 3, c.flow)])[0].full, c.full, String(c.flow));
});

const part = (lanes, km, mark = "") => ({ lanes, km, cut: mark === "cut", full: mark === "full" });
// The real inbound strip, 2026-11-24.
const kesas = [part(3, 0.33, "cut"), part(4, 0.63), part(5, 0.47), part(3, 2.5, "full"), part(5, 0.93)];

test("profileTitle gives the length and the road", () => {
  assert.equal(
    profileTitle(kesas, "E5 Lebuhraya Shah Alam"),
    "Lanes along the first 4.9 km of E5 Lebuhraya Shah Alam, which every route uses:",
  );
  assert.equal(profileTitle([part(3, 1)], ""), "Lanes along the first 1.0 km of the drive, which every route uses:");
});

test("profileSentence teaches the lesson only when the data shows it", () => {
  const lesson =
    "Up to 5 lanes wide, but the 3-lane bottleneck decides how much gets through. The wider parts have room to spare, so more lanes there would add nothing.";
  const cases = [
    { name: "bottleneck with wider parts that have room", parts: kesas, want: lesson },
    { name: "a wider part is itself nearly full", parts: [part(3, 1, "cut"), part(4, 1, "full")], want: "Between 3 and 4 lanes wide." },
    { name: "the bottleneck is somewhere else", parts: [part(3, 1), part(5, 1)], want: "Between 3 and 5 lanes wide." },
    { name: "nothing wider than the bottleneck", parts: [part(3, 1, "cut"), part(2, 1)], want: "Between 2 and 3 lanes wide." },
    { name: "one width", parts: [part(3, 1, "cut"), part(3, 1, "full")], want: "3 lanes all the way." },
    { name: "one lane", parts: [part(1, 1)], want: "1 lane all the way." },
    { name: "nothing to show", parts: [], want: "" },
  ];
  for (const c of cases) assert.equal(profileSentence(c.parts), c.want, c.name);
});

test("profileLabel reads the strip out in order", () => {
  assert.equal(profileLabel(kesas), "Lanes in order: 3 (bottleneck), 4, 5, 3 (nearly full), 5.");
});

test("sharedName names the road only when it is one road", () => {
  const e5 = { id: 1, ref: "E5", name: "Lebuhraya Shah Alam", class: "motorway", lanes: 3 };
  const ramp = { id: 2, ref: "", name: "", class: "motorway_link", lanes: 1 };
  assert.equal(sharedName([e5, { ...e5, id: 3, lanes: 5 }]), "E5 Lebuhraya Shah Alam");
  assert.equal(sharedName([e5, ramp]), "");
  assert.equal(sharedName([]), "");
});
