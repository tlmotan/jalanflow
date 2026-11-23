// Flowing dots: each road gets dots in proportion to the max flow it
// carries, all moving at the same speed. Packed roads are full, sparse
// roads have room. This shows capacity, not live traffic, and never queues
// (decision 0011).
import type maplibregl from "maplibre-gl";
import type { Feature, FeatureCollection, LineString, Point } from "geojson";
import type { EdgeProps } from "./types";
import { COLORS, NEARLY_FULL } from "./colors";
import { advance, allocate, pointAt, track, type Track } from "./track";

// Display constants (docs/DESIGN.md §9-10), not traffic numbers.
const DOTS_PER_KM_PER_1000_VPH = 5; // density: 6,000 veh/h → 30 dots per km
const MAX_DOTS = 3000; // keep phones smooth; density scales down to fit
const SPEED_KM_PER_S = 0.5; // map distance a dot moves per real second
const FRAME_MS = 33; // ~30 fps

interface Dot {
  track: Track;
  at: number; // km along the track
  u: number; // utilisation, for colour
  cut: boolean;
}

export interface FlowDots {
  /** animate false: draw the dots once, still (prefers-reduced-motion). */
  start: (animate: boolean) => void;
  stop: () => void;
  readonly on: boolean;
}

export function initFlowDots(
  map: maplibregl.Map,
  edges: FeatureCollection<LineString, EdgeProps>,
  beforeLayer?: string,
): FlowDots {
  const carrying = edges.features.filter((f) => f.properties.flow_vph > 0 && f.geometry.coordinates.length >= 2);
  const tracks = carrying.map((f) => track(f.geometry.coordinates));
  const wanted = carrying.map((f, i) => (f.properties.flow_vph / 1000) * tracks[i]!.km * DOTS_PER_KM_PER_1000_VPH);
  const total = wanted.reduce((a, b) => a + b, 0);
  // Dots follow flow × length. Roads a few metres long get none rather
  // than one each, which used to clump at junctions.
  const counts = allocate(wanted, Math.min(total, MAX_DOTS));

  const dots: Dot[] = [];
  carrying.forEach((f, i) => {
    const t = tracks[i]!;
    const n = counts[i]!;
    for (let k = 0; k < n; k++) {
      // Even spacing with a little jitter so neighbouring roads don't march in step.
      const at = ((k + 0.3 * Math.random()) / n) * t.km;
      dots.push({ track: t, at, u: f.properties.utilisation, cut: f.properties.is_cut });
    }
  });

  const data = (): FeatureCollection<Point> => ({
    type: "FeatureCollection",
    features: dots.map(
      (d): Feature<Point> => ({
        type: "Feature",
        geometry: { type: "Point", coordinates: pointAt(d.track, d.at) },
        properties: { u: d.u, cut: d.cut },
      }),
    ),
  });

  map.addSource("flow-dots", { type: "geojson", data: { type: "FeatureCollection", features: [] } });
  map.addLayer(
    {
      id: "flow-dots",
      type: "circle",
      source: "flow-dots",
      layout: { visibility: "none" },
      paint: {
        "circle-radius": ["interpolate", ["linear"], ["zoom"], 10, 1.5, 15, 3.5],
        "circle-color": ["case", ["get", "cut"], COLORS.cut, [">=", ["get", "u"], NEARLY_FULL], COLORS.full, COLORS.spare],
        "circle-stroke-color": "#ffffff",
        "circle-stroke-width": 1,
      },
    },
    beforeLayer,
  );
  const src = map.getSource("flow-dots") as maplibregl.GeoJSONSource;

  let on = false;
  let raf = 0;
  let last = 0;
  let lastDraw = 0;
  const tick = (now: number): void => {
    if (!on) return;
    const dt = last ? Math.min(now - last, 100) / 1000 : 0; // clamp after a hidden tab
    last = now;
    // advance wraps by modulo: one frame can be longer than a short road.
    for (const d of dots) d.at = advance(d.at, SPEED_KM_PER_S * dt, d.track.km);
    if (now - lastDraw >= FRAME_MS) {
      lastDraw = now;
      src.setData(data());
    }
    raf = requestAnimationFrame(tick);
  };

  return {
    get on() {
      return on;
    },
    start(animate) {
      if (on) return;
      on = true;
      last = 0;
      src.setData(data());
      map.setLayoutProperty("flow-dots", "visibility", "visible");
      if (animate) raf = requestAnimationFrame(tick);
    },
    stop() {
      on = false;
      cancelAnimationFrame(raf);
      map.setLayoutProperty("flow-dots", "visibility", "none");
    },
  };
}
