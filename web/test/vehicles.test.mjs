// Routes, vehicles and lanes behind the flow dots and cars. Run with `npm test`.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { decompose } from "../src/routes.ts";
import { laneIndex, laneOffset, lanePx, LANE_STOPS } from "../src/lanes.ts";
import { buildRoutes, lateral, makeVehicles, spaceOut, spanAt } from "../src/vehicles.ts";

/** Flow per input edge once the routes are added back up. */
function perEdge(edges, routes) {
  const sum = edges.map(() => 0);
  for (const r of routes) for (const i of r.edges) sum[i] += r.flow;
  return sum;
}

test("decompose: routes add up to the flow on every road", () => {
  const cases = [
    { name: "one road", edges: [{ from: "a", to: "b", flow: 500 }], routes: 1 },
    {
      name: "two ways round (diamond)",
      edges: [
        { from: "s", to: "a", flow: 400 },
        { from: "s", to: "b", flow: 200 },
        { from: "a", to: "t", flow: 400 },
        { from: "b", to: "t", flow: 200 },
      ],
      routes: 2,
    },
    {
      name: "ramp joins, then some leave",
      edges: [
        { from: "s", to: "m", flow: 600 },
        { from: "m", to: "x", flow: 600 },
        { from: "x", to: "t", flow: 450 },
        { from: "x", to: "y", flow: 150 },
        { from: "y", to: "t", flow: 150 },
      ],
      routes: 2,
    },
    {
      name: "roads with no flow are left out",
      edges: [
        { from: "s", to: "t", flow: 300 },
        { from: "s", to: "z", flow: 0 },
      ],
      routes: 1,
    },
    {
      name: "a loop does not hang",
      edges: [
        { from: "a", to: "b", flow: 100 },
        { from: "b", to: "a", flow: 100 },
      ],
      routes: 1,
    },
  ];
  for (const c of cases) {
    const routes = decompose(c.edges);
    assert.equal(routes.length, c.routes, c.name);
    assert.deepEqual(
      perEdge(c.edges, routes),
      c.edges.map((e) => e.flow),
      c.name,
    );
    for (const r of routes) {
      for (let i = 1; i < r.edges.length; i++) {
        assert.equal(c.edges[r.edges[i - 1]].to, c.edges[r.edges[i]].from, `${c.name}: route is connected`);
      }
    }
  }
});

// The committed site data: every vehicle must drive gate to gate.
const inbound = JSON.parse(
  readFileSync(new URL("../public/data/kemuning-pj/edges.geojson", import.meta.url), "utf8"),
).features.filter((f) => f.properties.dir === "inbound" && f.properties.flow_vph > 0);
const roads = inbound.map((f) => ({
  coords: f.geometry.coordinates,
  lanes: f.properties.lanes,
  flow: f.properties.flow_vph,
  utilisation: f.properties.utilisation,
  cut: f.properties.is_cut,
}));

test("real corridor: routes run gate to gate and match the exported flow", () => {
  const routes = buildRoutes(roads);
  assert.ok(routes.length > 0);
  const sum = roads.map(() => 0);
  const starts = new Set();
  const ends = new Set();
  for (const r of routes) {
    for (const s of r.spans) sum[s.road] += r.flow;
    starts.add(r.track.coords[0].join());
    ends.add(r.track.coords[r.track.coords.length - 1].join());
    // Spans tile the route with no gaps.
    assert.equal(r.spans[0].start, 0);
    assert.ok(Math.abs(r.spans[r.spans.length - 1].end - r.track.km) < 1e-9);
    for (let i = 1; i < r.spans.length; i++) assert.equal(r.spans[i].start, r.spans[i - 1].end);
  }
  assert.deepEqual(
    sum,
    roads.map((r) => r.flow),
  );
  assert.equal(starts.size, 1, "one start gate");
  assert.equal(ends.size, 1, "one end gate");
});

test("vehicles: capped, on their route, and denser where flow is higher", () => {
  const routes = buildRoutes(roads);
  const vehicles = makeVehicles(routes, 15, 3000);
  assert.ok(vehicles.length > 1000 && vehicles.length <= 3000, `${vehicles.length} vehicles`);
  for (const v of vehicles) {
    assert.ok(v.at >= 0 && v.at < v.route.track.km);
    assert.ok(v.share >= 0 && v.share < 1 && v.rank >= 0 && v.rank < 1);
    const s = v.route.spans[spanAt(v.route, v.at)];
    assert.ok(v.at >= s.start && v.at <= s.end);
  }
  assert.equal(makeVehicles(routes, 15, 100).length, 100, "cap is respected");
  // "One in five" by rank really is about a fifth.
  const fifth = vehicles.filter((v) => v.rank < 0.2).length / vehicles.length;
  assert.ok(fifth > 0.17 && fifth < 0.23, `rank < 0.2 picks ${fifth}`);
});

test("lanes: every lane gets cars and offsets stay on the road", () => {
  const cases = [
    { lanes: 1, offsets: [0] },
    { lanes: 2, offsets: [-0.5, 0.5] },
    { lanes: 3, offsets: [-1, 0, 1] },
    { lanes: 5, offsets: [-2, -1, 0, 1, 2] },
  ];
  for (const c of cases) {
    const seen = new Set();
    for (let i = 0; i < 100; i++) {
      const share = i / 100;
      seen.add(laneOffset(share, c.lanes));
      assert.equal(laneOffset(share, c.lanes), laneIndex(share, c.lanes) + 0.5 - c.lanes / 2);
    }
    assert.deepEqual([...seen].sort((a, b) => a - b), c.offsets, `${c.lanes} lanes`);
  }
  assert.equal(lanePx(12), 0, "one line when zoomed out");
  for (const [z, px] of LANE_STOPS) assert.equal(lanePx(z), px);
  assert.ok(lanePx(13.5) > 0 && lanePx(13.5) < LANE_STOPS[0][1]);
});

test("lateral: a car slides into its new lane before a lane drop, never jumps", () => {
  // 3 lanes for 1 km, then 2 lanes for 1 km.
  const route = {
    track: { cum: [0, 1, 2], coords: [[0, 0], [0.01, 0], [0.02, 0]], km: 2 },
    flow: 1000,
    spans: [
      { road: 0, start: 0, end: 1, lanes: 3, utilisation: 1, cut: false },
      { road: 1, start: 1, end: 2, lanes: 2, utilisation: 1, cut: false },
    ],
  };
  const share = 0.9; // right lane of 3 (+1) → right lane of 2 (+0.5)
  assert.equal(lateral(route, 0, 0.5, share), 1);
  assert.equal(lateral(route, 1, 1.5, share), 0.5);
  let prev = 1;
  for (let km = 0.9; km <= 1; km += 0.001) {
    const x = lateral(route, 0, km, share);
    assert.ok(x <= prev + 1e-12 && x >= 0.5 - 1e-12, `at ${km}: ${x}`);
    assert.ok(prev - x < 0.1, `no jump at ${km}`);
    prev = x;
  }
  assert.ok(Math.abs(lateral(route, 0, 1, share) - lateral(route, 1, 1, share)) < 1e-9, "continuous at the join");
});

test("lateral: where a road widens, cars spread out only once they are on the wider road", () => {
  // 2 lanes for 1 km, then 9 lanes (a toll plaza) for 1 km. Cars used to
  // swing out towards the plaza lanes while still on the 2-lane road.
  const route = {
    track: { cum: [0, 1, 2], coords: [[0, 0], [0.01, 0], [0.02, 0]], km: 2 },
    flow: 1000,
    spans: [
      { road: 0, start: 0, end: 1, lanes: 2, utilisation: 1, cut: false },
      { road: 1, start: 1, end: 2, lanes: 9, utilisation: 1, cut: false },
    ],
  };
  for (const share of [0.05, 0.5, 0.95]) {
    for (let km = 0.9; km <= 1; km += 0.001) {
      assert.ok(Math.abs(lateral(route, 0, km, share)) <= 0.5, `share ${share} at ${km}: still within 2 lanes`);
    }
    assert.ok(Math.abs(lateral(route, 0, 1, share) - lateral(route, 1, 1, share)) < 1e-9, "continuous at the join");
    assert.equal(lateral(route, 1, 1.5, share), laneOffset(share, 9));
  }
});

test("spaceOut: cars in one lane never overlap and never move forward", () => {
  const cases = [
    { name: "already apart", front: [0.5, 0.4, 0.1], gap: 0.02, want: [0.5, 0.4, 0.1] },
    { name: "two on top of each other", front: [0.5, 0.5], gap: 0.02, want: [0.5, 0.48] },
    { name: "a bunch", front: [0.3, 0.295, 0.29], gap: 0.01, want: [0.3, 0.29, 0.28] },
    { name: "pushed off the start", front: [0.005, 0.004], gap: 0.01, want: [0.005, -0.005] },
  ];
  for (const c of cases) {
    const got = spaceOut(c.front, c.gap);
    got.forEach((g, i) => assert.ok(Math.abs(g - c.want[i]) < 1e-12, `${c.name}: ${got}`));
  }
});
