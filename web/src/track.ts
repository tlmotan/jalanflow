// Pure geometry for things that move along a road line (particles.ts).
// No DOM, no MapLibre: tested with `npm test` (web/test/track.test.mjs).

export interface Track {
  cum: number[]; // cumulative km at each vertex
  coords: [number, number][]; // [lon, lat]
  km: number; // total length
}

const KM_PER_DEG = 111.32;

export function track(coords: number[][]): Track {
  const pts = coords.map((c) => [c[0]!, c[1]!] as [number, number]);
  const cum = [0];
  for (let i = 1; i < pts.length; i++) {
    const [x0, y0] = pts[i - 1]!;
    const [x1, y1] = pts[i]!;
    const dx = (x1 - x0) * Math.cos((((y0 + y1) / 2) * Math.PI) / 180);
    cum.push(cum[i - 1]! + Math.hypot(dx, y1 - y0) * KM_PER_DEG);
  }
  return { cum, coords: pts, km: cum[cum.length - 1]! };
}

/**
 * Point `km` along the track. `km` is clamped to the track, so a caller's
 * mistake can never draw anything beyond the ends of the road.
 */
export function pointAt(t: Track, km: number): [number, number] {
  const at = Math.min(Math.max(km, 0), t.km);
  let i = 1;
  while (i < t.cum.length - 1 && t.cum[i]! < at) i++;
  const a = t.cum[i - 1]!;
  const span = t.cum[i]! - a || 1;
  const k = Math.min(Math.max((at - a) / span, 0), 1);
  const [x0, y0] = t.coords[i - 1]!;
  const [x1, y1] = t.coords[i]!;
  return [x0 + (x1 - x0) * k, y0 + (y1 - y0) * k];
}

/**
 * Move `step` km along a looping track of length `km` and wrap back to the
 * start. The step can be many times the track length (a 6 m road, a 16 m
 * step): the result always lies in [0, km).
 */
export function advance(at: number, step: number, km: number): number {
  if (!(km > 0)) return 0;
  const next = (at + step) % km;
  return next < 0 ? next + km : next;
}

/**
 * Share `total` whole items between buckets in proportion to `wanted`
 * (largest remainder), so short roads are not each forced to take one.
 * The counts add up to round(total), or 0 when nothing is wanted.
 */
export function allocate(wanted: number[], total: number): number[] {
  const sum = wanted.reduce((a, b) => a + b, 0);
  if (!(sum > 0) || !(total > 0)) return wanted.map(() => 0);
  const exact = wanted.map((w) => (w / sum) * total);
  const counts = exact.map((x) => Math.floor(x));
  let left = Math.round(total) - counts.reduce((a, b) => a + b, 0);
  const order = exact.map((x, i) => [x - Math.floor(x), i] as const).sort((a, b) => b[0] - a[0] || a[1] - b[1]);
  for (const [, i] of order) {
    if (left <= 0) break;
    counts[i]!++;
    left--;
  }
  return counts;
}
