// TRIAL page: the flow dots. Dots in proportion to what the analysis sends
// down each road, all at one speed: a packed road is full, a sparse one has
// room. They show the model's result, not real cars, and never queue
// (decision 0011). The routes and spacing come from the site's own pure
// helpers; only the drawing is the trial's. Zoomed out they are dots on the
// road line; zoomed in they are small cars, each in its own lane (decision
// 0012). The car drawing is a copy of src/particles.ts in the trial's
// colours: fold the two together if the trial is kept.
import type maplibregl from "maplibre-gl";
import type { Feature, FeatureCollection, LineString, Point } from "geojson";
import type { Direction, EdgeProps } from "../types";
import { advance, heading, pointAt, segmentAt, shift } from "../track";
import { byLaneWidth, laneIndex, lanePx, lerpStops, metresPerPixel } from "../lanes";
import { buildRoutes, lateral, makeVehicles, spaceOut, spanAt, type Vehicle } from "../vehicles";

// Display constants, not traffic numbers. Same density and speed as the site.
const PER_KM_PER_1000_VPH = 15;
const MAX_VEHICLES = 3000;
// Share of the dots by zoom. The share is taken by rank, evenly on every road,
// so a road with twice the flow still shows twice the dots at any zoom; zoomed
// out the map just shows fewer of them, so the road colours stay readable.
const SHOWN: [number, number][] = [[11, 0.2], [12, 0.25], [13, 1 / 3], [14, 0.5], [15, 0.75], [16, 1]];
const SPEED_PX: [number, number][] = [[11, 16], [14, 34], [16, 60], [18, 90]]; // screen px a second by zoom
const FRAME_MS = 33; // ~30 fps
const DOTS_MAX_ZOOM = 14.25; // dots fade out …
const CARS_MIN_ZOOM = 13.5; // … as cars fade in
const CAR_LENGTH_LANES = 1.7; // car size in lane widths
const CAR_WIDTH_LANES = 0.7;
const CAR_MIN_PX = 4;
const CAR_GAP = 1.3; // nose-to-nose spacing in car lengths when cars share a lane
const GLASS_MIN_LANE_PX = 5; // draw the windscreen once a car is big enough

interface Seen {
  v: Vehicle;
  span: number;
  local: number; // km from the start of the span
}

const EMPTY: FeatureCollection = { type: "FeatureCollection", features: [] };

export interface Flow {
  /** Dots for one direction. animate false draws them once, still (reduced motion). */
  show: (dir: Direction, animate: boolean) => void;
  hide: () => void;
}

export interface FlowColours {
  dot: string;
  edge: string;
}

export function initFlow(
  map: maplibregl.Map,
  edges: Feature<LineString, EdgeProps>[],
  colour: FlowColours,
  beforeLayer?: string,
): Flow {
  // Built the first time a direction is shown.
  const built = new Map<Direction, Vehicle[]>();
  const vehiclesFor = (dir: Direction): Vehicle[] => {
    let v = built.get(dir);
    if (!v) {
      const carrying = edges.filter(
        (f) => f.properties.dir === dir && f.properties.flow_vph > 0 && f.geometry.coordinates.length >= 2,
      );
      const routes = buildRoutes(
        carrying.map((f) => ({
          coords: f.geometry.coordinates,
          lanes: f.properties.lanes,
          flow: f.properties.flow_vph,
          utilisation: f.properties.utilisation,
          cut: f.properties.is_cut,
        })),
      );
      v = makeVehicles(routes, PER_KM_PER_1000_VPH, MAX_VEHICLES);
      built.set(dir, v);
    }
    return v;
  };

  map.addSource("flow", { type: "geojson", data: EMPTY });
  map.addSource("flow-cars", { type: "geojson", data: EMPTY });
  const fadeIn: maplibregl.ExpressionSpecification = ["interpolate", ["linear"], ["zoom"], CARS_MIN_ZOOM, 0, DOTS_MAX_ZOOM, 1];
  map.addLayer(
    {
      id: "flow",
      type: "circle",
      source: "flow",
      maxzoom: DOTS_MAX_ZOOM,
      layout: { visibility: "none" },
      paint: {
        // Small and pale with a dark edge: readable on grey, amber and red alike.
        // Zoomed out they are smaller and their edge fainter, so the red bottleneck
        // and the amber nearly-full roads show through; they fade out into cars.
        "circle-radius": ["interpolate", ["linear"], ["zoom"], 10, 1, 12, 1.4, DOTS_MAX_ZOOM, 2.3],
        "circle-color": colour.dot,
        "circle-stroke-color": colour.edge,
        "circle-stroke-width": ["interpolate", ["linear"], ["zoom"], 10, 0.4, 13, 0.6],
        "circle-opacity": ["interpolate", ["linear"], ["zoom"], 10, 0.85, CARS_MIN_ZOOM, 1, DOTS_MAX_ZOOM, 0],
        "circle-stroke-opacity": ["interpolate", ["linear"], ["zoom"], 10, 0.45, 12, 0.6, CARS_MIN_ZOOM, 0.85, DOTS_MAX_ZOOM, 0],
      },
    },
    beforeLayer,
  );
  // A car is three short lines along the road: outline, body, windscreen.
  const carLayer = (id: string, part: number, color: string, width: (px: number) => number, opacity = 1): void => {
    map.addLayer(
      {
        id,
        type: "line",
        source: "flow-cars",
        minzoom: CARS_MIN_ZOOM,
        filter: ["==", ["get", "part"], part],
        layout: { visibility: "none" },
        paint: {
          "line-color": color,
          "line-width": byLaneWidth(width(0), width),
          "line-opacity": opacity === 1 ? fadeIn : opacity,
        },
      },
      beforeLayer,
    );
  };
  // Pale cars with a dark outline: the same mark as the dots, on roads of any colour.
  carLayer("flow-cars-edge", 0, colour.edge, (px) => CAR_WIDTH_LANES * px + 2);
  carLayer("flow-cars", 1, colour.dot, (px) => CAR_WIDTH_LANES * px);
  carLayer("flow-cars-glass", 2, colour.edge, (px) => Math.max(0.5, CAR_WIDTH_LANES * px - 1.6), 0.6);
  const layers = ["flow", "flow-cars-edge", "flow-cars", "flow-cars-glass"];
  const src = map.getSource("flow") as maplibregl.GeoJSONSource;
  const carSrc = map.getSource("flow-cars") as maplibregl.GeoJSONSource;
  const line = (a: [number, number], b: [number, number], part: number): Feature<LineString> => ({
    type: "Feature",
    geometry: { type: "LineString", coordinates: [a, b] },
    properties: { part },
  });

  let vehicles: Vehicle[] = [];
  let on = false;
  let moving = false;
  let last = 0;
  let lastDraw = 0;

  let dotsDrawn = false;
  let carsDrawn = false;
  const draw = (): void => {
    const zoom = map.getZoom();
    const shown = lerpStops(SHOWN, zoom);
    const wantDots = zoom < DOTS_MAX_ZOOM;
    const wantCars = zoom >= CARS_MIN_ZOOM;
    // Only what is on screen (plus a margin) is sent to the map.
    const b = map.getBounds();
    const padX = (b.getEast() - b.getWest()) * 0.1;
    const padY = (b.getNorth() - b.getSouth()) * 0.1;
    const [w, e, s, n] = [b.getWest() - padX, b.getEast() + padX, b.getSouth() - padY, b.getNorth() + padY];
    const features: Feature<Point>[] = [];
    const lanes = new Map<number, Seen[]>(); // road and lane → cars in it
    for (const v of vehicles) {
      if (v.rank >= shown) continue;
      const p = pointAt(v.route.track, v.at);
      if (p[0] < w || p[0] > e || p[1] < s || p[1] > n) continue;
      const si = spanAt(v.route, v.at);
      const span = v.route.spans[si]!;
      if (wantDots) {
        features.push({ type: "Feature", geometry: { type: "Point", coordinates: p }, properties: { cut: span.cut } });
      }
      if (wantCars) {
        const k = span.road * 64 + laneIndex(v.share, span.lanes);
        const seen: Seen = { v, span: si, local: v.at - span.start };
        const list = lanes.get(k);
        if (list) list.push(seen);
        else lanes.set(k, [seen]);
      }
    }

    const cars: Feature<LineString>[] = [];
    if (wantCars) {
      const px = lanePx(zoom);
      const mpp = metresPerPixel(zoom, map.getCenter().lat);
      const length = Math.max(CAR_MIN_PX, CAR_LENGTH_LANES * px) * mpp; // metres
      const glass = px >= GLASS_MIN_LANE_PX;
      for (const list of lanes.values()) {
        list.sort((a, b) => b.local - a.local);
        const spaced = spaceOut(
          list.map((c) => c.local),
          (length * CAR_GAP) / 1000,
        );
        list.forEach((c, i) => {
          const local = spaced[i]!;
          if (local < 0) return; // no room on this road yet: it appears a moment later
          const { route, share } = c.v;
          const km = route.spans[c.span]!.start + local;
          const [east, north] = heading(route.track, segmentAt(route.track, km));
          const side = lateral(route, c.span, km, share) * px * mpp; // metres right of centre
          const mid = shift(pointAt(route.track, km), north * side, -east * side);
          const along = (m: number): [number, number] => shift(mid, east * m, north * m);
          cars.push(line(along(-length / 2 - mpp), along(length / 2 + mpp), 0));
          cars.push(line(along(-length / 2), along(length / 2), 1));
          if (glass) cars.push(line(along(length * 0.1), along(length * 0.3), 2));
        });
      }
    }

    if (wantDots || dotsDrawn) src.setData({ type: "FeatureCollection", features });
    if (wantCars || carsDrawn) carSrc.setData({ type: "FeatureCollection", features: cars });
    dotsDrawn = wantDots;
    carsDrawn = wantCars;
  };

  // requestAnimationFrame stops by itself while the tab is hidden.
  const tick = (now: number): void => {
    if (!on || !moving) return;
    const dt = last ? Math.min(now - last, 100) / 1000 : 0; // clamp after a hidden tab
    last = now;
    const zoom = map.getZoom();
    const step = (lerpStops(SPEED_PX, zoom) * metresPerPixel(zoom, map.getCenter().lat) * dt) / 1000; // km
    for (const v of vehicles) v.at = advance(v.at, step, v.route.track.km);
    if (now - lastDraw >= FRAME_MS) {
      lastDraw = now;
      draw();
    }
    requestAnimationFrame(tick);
  };
  // Standing still (reduced motion), the picture still has to follow the map.
  map.on("move", () => {
    if (on && !moving) draw();
  });

  return {
    show(dir, animate) {
      vehicles = vehiclesFor(dir);
      const wasMoving = on && moving;
      on = true;
      moving = animate;
      last = 0;
      draw();
      for (const id of layers) map.setLayoutProperty(id, "visibility", "visible");
      if (animate && !wasMoving) requestAnimationFrame(tick);
    },
    hide() {
      on = false;
      for (const id of layers) map.setLayoutProperty(id, "visibility", "none");
      src.setData(EMPTY);
      carSrc.setData(EMPTY);
      dotsDrawn = carsDrawn = false;
    },
  };
}
