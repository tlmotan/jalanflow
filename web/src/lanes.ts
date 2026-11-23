// How wide a lane is drawn at each zoom. One table drives the road width,
// the lane lines and the car positions, so they cannot disagree
// (docs/decisions/0012). Display constants, not traffic numbers.
//
// OSM gives a lane count per road, not where each lane runs, so lanes are
// equal stripes either side of the road's centre line. A real lane is
// about 3.5 m: these widths are ~3x that at zoom 14 and close to true
// scale from zoom 17.
import type { ExpressionSpecification } from "maplibre-gl";

/** Below this zoom a road is one line with dots; from LANES_FULL_ZOOM it is lanes with cars. */
export const LANES_START_ZOOM = 13;
export const LANES_FULL_ZOOM = 14;

/** [zoom, pixels per lane], linear in between. */
export const LANE_STOPS: readonly (readonly [number, number])[] = [
  [14, 2.5],
  [15, 4],
  [16, 6],
  [17, 9],
  [18, 14],
  [19, 28],
  [20, 56],
];

/** Piecewise-linear value at x from [x, y] stops, clamped at both ends. */
export function lerpStops(stops: readonly (readonly [number, number])[], x: number): number {
  const first = stops[0]!;
  if (x <= first[0]) return first[1];
  for (let i = 1; i < stops.length; i++) {
    const [x1, y1] = stops[i]!;
    if (x <= x1) {
      const [x0, y0] = stops[i - 1]!;
      return y0 + ((x - x0) / (x1 - x0)) * (y1 - y0);
    }
  }
  return stops[stops.length - 1]![1];
}

/**
 * Pixels per lane at a zoom. 0 up to LANES_START_ZOOM (the road is a single
 * line there), growing in to the first stop at LANES_FULL_ZOOM.
 */
export function lanePx(zoom: number): number {
  return lerpStops([[LANES_START_ZOOM, 0], ...LANE_STOPS], zoom);
}

/** Ground metres per screen pixel (512 px tiles, as MapLibre uses). */
export function metresPerPixel(zoom: number, lat: number): number {
  return (40075016.686 * Math.cos((lat * Math.PI) / 180)) / (512 * 2 ** zoom);
}

/**
 * Sideways position of a vehicle in lane widths, right of the centre line.
 * `share` (0–1) picks the lane; lane 0 is the left lane (Malaysia drives
 * on the left).
 */
export function laneOffset(share: number, lanes: number): number {
  const n = Math.max(1, Math.round(lanes));
  const lane = Math.min(n - 1, Math.floor(share * n));
  return lane + 0.5 - n / 2;
}

/** Lane index (0 = left) for a share, as used by laneOffset. */
export function laneIndex(share: number, lanes: number): number {
  const n = Math.max(1, Math.round(lanes));
  return Math.min(n - 1, Math.floor(share * n));
}

const LANES: ExpressionSpecification = ["get", "lanes"];
// The single-line look used when zoomed out: 2 px for 1 lane, 7 px for 5.
const LINE_WIDTH: ExpressionSpecification = ["interpolate", ["linear"], LANES, 1, 2, 5, 7];

/**
 * A zoom-interpolated paint value. `far` is the value at LANES_START_ZOOM
 * and below; `near` builds the value from the lane width at each stop.
 * (MapLibre wants ["zoom"] at the top of the expression, hence the builder.)
 */
export function byLaneWidth(
  far: number | ExpressionSpecification,
  near: (px: number) => number | ExpressionSpecification,
): ExpressionSpecification {
  const out: unknown[] = ["interpolate", ["linear"], ["zoom"], LANES_START_ZOOM, far];
  for (const [z, px] of LANE_STOPS) out.push(z, near(px));
  return out as ExpressionSpecification;
}

/** Road width in px: one line when far, lanes × lane width when near. */
export function roadWidth(extra = 0): ExpressionSpecification {
  return byLaneWidth(extra ? ["+", LINE_WIDTH, extra] : LINE_WIDTH, (px) => ["+", ["*", LANES, px], extra]);
}

/** line-offset of lane line `k` (between lane k-1 and lane k, counted from the left). */
export function laneLineOffset(k: number): ExpressionSpecification {
  return byLaneWidth(0, (px) => ["*", ["-", k, ["/", LANES, 2]], px]);
}
