// The vehicles behind the flow dots and cars (particles.ts): which routes
// they drive, how many there are and which lane each one keeps. Pure, no
// DOM or MapLibre: tested with `npm test`.
//
// Everything here is drawn from the exported max flow. Vehicles all move at
// one speed and never queue (decisions 0011, 0012).
import { allocate, track, type Track } from "./track.ts";
import { decompose } from "./routes.ts";
import { laneOffset } from "./lanes.ts";

/** One flow-carrying road from edges.geojson. */
export interface Road {
  coords: number[][];
  lanes: number;
  flow: number; // veh/h
  utilisation: number;
  cut: boolean;
}

/** The stretch of a route that runs along one road. */
export interface Span {
  road: number; // index into the roads given to buildRoutes
  start: number; // km along the route
  end: number;
  lanes: number;
  utilisation: number;
  cut: boolean;
}

export interface Route {
  track: Track;
  spans: Span[];
  flow: number; // veh/h
}

export interface Vehicle {
  route: Route;
  at: number; // km along the route
  share: number; // 0–1 across the road, fixed for life: picks the lane
  rank: number; // 0–1: vehicles with rank < x are the ones shown at density x
}

const key = (c: number[]): string => `${c[0]},${c[1]}`;

/** Joins roads end to end into the routes the flow takes (routes.ts). */
export function buildRoutes(roads: Road[]): Route[] {
  const parts = decompose(
    roads.map((r) => ({ from: key(r.coords[0]!), to: key(r.coords[r.coords.length - 1]!), flow: r.flow })),
  );
  return parts.map((p) => {
    const coords: number[][] = [];
    const joints: number[] = [0]; // vertex index where each road starts, then the last vertex
    for (const i of p.edges) {
      const c = roads[i]!.coords;
      coords.push(...(coords.length ? c.slice(1) : c));
      joints.push(coords.length - 1);
    }
    const t = track(coords);
    const spans = p.edges.map((i, n): Span => {
      const r = roads[i]!;
      return {
        road: i,
        start: t.cum[joints[n]!]!,
        end: t.cum[joints[n + 1]!]!,
        lanes: r.lanes,
        utilisation: r.utilisation,
        cut: r.cut,
      };
    });
    return { track: t, spans, flow: p.flow };
  });
}

const frac = (x: number): number => x - Math.floor(x);

/**
 * Vehicles in proportion to flow × length, at most `max`. Spacing along a
 * route is even. `share` and `rank` use irrational steps so that
 * neighbours take different lanes, and so that any "first x %" by rank is
 * itself evenly spread along the route.
 */
export function makeVehicles(routes: Route[], perKmPer1000Vph: number, max: number): Vehicle[] {
  const wanted = routes.map((r) => (r.flow / 1000) * r.track.km * perKmPer1000Vph);
  const total = wanted.reduce((a, b) => a + b, 0);
  const counts = allocate(wanted, Math.min(total, max));
  const out: Vehicle[] = [];
  routes.forEach((route, r) => {
    const n = counts[r]!;
    for (let k = 0; k < n; k++) {
      out.push({
        route,
        at: (frac(k / n + r * 0.137) || 0) * route.track.km,
        share: frac((k + 1) * Math.SQRT2 + r * 0.37),
        rank: frac((k + 1) * 0.6180339887 + r * 0.23),
      });
    }
  });
  return out;
}

/** Index of the span holding `km` on a route. */
export function spanAt(r: Route, km: number): number {
  let lo = 0;
  let hi = r.spans.length - 1;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (r.spans[mid]!.end < km) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

/** Distance over which a vehicle slides into its lane on the next road. */
export const LANE_CHANGE_KM = 0.03;

/**
 * Sideways position in lane widths, right of the centre line. Where the
 * lane count changes the vehicle slides across instead of jumping, always
 * on the wider of the two roads so it never leaves the road surface:
 * before the join when lanes are lost, after it when lanes are gained.
 */
export function lateral(r: Route, span: number, km: number, share: number): number {
  const s = r.spans[span]!;
  const here = laneOffset(share, s.lanes);
  const room = Math.min(LANE_CHANGE_KM, s.end - s.start);
  if (room <= 0) return here;
  const slide = (from: number, to: number, t: number): number => {
    const k = Math.min(1, Math.max(0, t));
    return from + (to - from) * k * k * (3 - 2 * k);
  };
  const next = r.spans[span + 1];
  if (next && next.lanes < s.lanes && s.end - km < room) {
    return slide(here, laneOffset(share, next.lanes), 1 - (s.end - km) / room);
  }
  const prev = r.spans[span - 1];
  if (prev && prev.lanes < s.lanes && km - s.start < room) {
    return slide(laneOffset(share, prev.lanes), here, (km - s.start) / room);
  }
  return here;
}

/**
 * Keeps vehicles in one lane from being drawn on top of each other.
 * `front` holds positions front-first; each is moved back just enough to
 * sit `gap` behind the one ahead. Drawing only: nothing slows down.
 */
export function spaceOut(front: number[], gap: number): number[] {
  let limit = Infinity;
  return front.map((p) => {
    const q = Math.min(p, limit);
    limit = q - gap;
    return q;
  });
}
