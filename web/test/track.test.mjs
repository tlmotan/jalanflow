// Run with `npm test` (Node's built-in test runner, no dependency).
import test from "node:test";
import assert from "node:assert/strict";
import { advance, allocate, pointAt, track } from "../src/track.ts";

// Edge 6739 (Jalan PJU 1A/4F, Ara Damansara): 7.7 m long. Dots on roads
// this short used to fly off the map, because one frame moves a dot
// further than the whole road.
const SHORT = [
  [101.591664, 3.109494],
  [101.591733, 3.109496],
];

test("advance keeps a dot on the road whatever the step", () => {
  const cases = [
    { name: "step shorter than the road", km: 1, step: 0.008, frames: 1000 },
    { name: "step longer than the road (60 fps)", km: 0.0077, step: 0.0083, frames: 1000 },
    { name: "step many times the road (slow frame)", km: 0.0059, step: 0.05, frames: 1000 },
    { name: "zero-length road", km: 0, step: 0.008, frames: 10 },
  ];
  for (const c of cases) {
    let at = 0;
    for (let i = 0; i < c.frames; i++) {
      at = advance(at, c.step, c.km);
      assert.ok(at >= 0 && at <= c.km, `${c.name}: frame ${i} at ${at} km, road is ${c.km} km`);
    }
  }
});

test("a dot on a 7.7 m road stays between its two ends", () => {
  const t = track(SHORT);
  assert.ok(t.km > 0.007 && t.km < 0.009, `length ${t.km}`);
  let at = 0;
  for (let i = 0; i < 600; i++) {
    at = advance(at, 0.5 / 60, t.km); // 0.5 km/s at 60 fps
    const [lon, lat] = pointAt(t, at);
    assert.ok(lon >= SHORT[0][0] && lon <= SHORT[1][0], `frame ${i}: lon ${lon}`);
    assert.ok(lat >= SHORT[0][1] && lat <= SHORT[1][1], `frame ${i}: lat ${lat}`);
  }
});

test("pointAt never leaves the line", () => {
  const t = track(SHORT);
  assert.deepEqual(pointAt(t, t.km * 40), SHORT[1]);
  assert.deepEqual(pointAt(t, -1), SHORT[0]);
  const [lon] = pointAt(t, t.km / 2);
  assert.ok(Math.abs(lon - (SHORT[0][0] + SHORT[1][0]) / 2) < 1e-9);
});

test("allocate shares dots in proportion and keeps the total", () => {
  const cases = [
    { name: "even", wanted: [1, 1, 1, 1], total: 4, want: [1, 1, 1, 1] },
    { name: "tiny roads get none", wanted: [10, 0.02, 0.02], total: 10, want: [10, 0, 0] },
    { name: "spare dots go to the largest remainders", wanted: [1.6, 1.6, 0.8], total: 4, want: [2, 1, 1] },
    { name: "scaled down to a cap", wanted: [300, 100], total: 40, want: [30, 10] },
    { name: "nothing wanted", wanted: [0, 0], total: 5, want: [0, 0] },
  ];
  for (const c of cases) assert.deepEqual(allocate(c.wanted, c.total), c.want, c.name);
});
