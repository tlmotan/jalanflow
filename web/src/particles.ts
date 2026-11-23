// Flowing dots: each road gets dots in proportion to the max flow it
// carries, all moving at the same speed. Packed roads are full, sparse
// roads have room. This shows capacity, not live traffic, and never queues
// (decision 0011).
import type maplibregl from "maplibre-gl";
import type { Feature, FeatureCollection, LineString, Point } from "geojson";
import type { EdgeProps } from "./types";
import { COLORS, NEARLY_FULL } from "./colors";

// Display constants (docs/DESIGN.md §9-10), not traffic numbers.
const DOTS_PER_KM_PER_1000_VPH = 5; // density: 6,000 veh/h → 30 dots per km
const MAX_DOTS = 3000; // keep phones smooth; density scales down to fit
const SPEED_KM_PER_S = 0.5; // map distance a dot moves per real second
const FRAME_MS = 33; // ~30 fps

interface Track {
  cum: number[]; // cumulative km at each vertex
  coords: [number, number][];
  km: number;
}

interface Dot {
  track: Track;
  at: number; // km along the track
  u: number; // utilisation, for colour
  cut: boolean;
}

const KM_PER_DEG = 111.32;

function track(coords: number[][]): Track {
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

function pointAt(t: Track, km: number): [number, number] {
  let i = 1;
  while (i < t.cum.length - 1 && t.cum[i]! < km) i++;
  const a = t.cum[i - 1]!;
  const span = t.cum[i]! - a || 1;
  const k = (km - a) / span;
  const [x0, y0] = t.coords[i - 1]!;
  const [x1, y1] = t.coords[i]!;
  return [x0 + (x1 - x0) * k, y0 + (y1 - y0) * k];
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
  const scale = total > MAX_DOTS ? MAX_DOTS / total : 1;

  const dots: Dot[] = [];
  carrying.forEach((f, i) => {
    const t = tracks[i]!;
    const n = Math.max(1, Math.round(wanted[i]! * scale));
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
    for (const d of dots) {
      d.at += SPEED_KM_PER_S * dt;
      if (d.at > d.track.km) d.at -= d.track.km;
    }
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
