// Flow: vehicles in proportion to the max flow each road carries, all at
// one speed. Zoomed out they are dots on the road line; zoomed in they are
// small cars, each in its own lane. Packed roads are full, sparse roads
// have room. This shows capacity, not live traffic, and never queues
// (decisions 0011, 0012).
import type maplibregl from "maplibre-gl";
import type { Feature, FeatureCollection, LineString, Point } from "geojson";
import type { EdgeProps } from "./types";
import { colors } from "./colors";
import type { LaneColours } from "./trial";
import { advance, heading, pointAt, segmentAt, shift } from "./track";
import { byLaneWidth, laneIndex, lanePx, lerpStops, metresPerPixel } from "./lanes";
import { buildRoutes, lateral, makeVehicles, spaceOut, spanAt, type Vehicle } from "./vehicles";

// Display constants (docs/DESIGN.md §9-10), not traffic numbers.
const PER_KM_PER_1000_VPH = 15; // every vehicle: 2,000 veh/h in a lane → one per 33 m
const MAX_VEHICLES = 3000; // keep phones smooth; density scales down to fit
// Share of the vehicles shown at each zoom. Zoomed out, one in three (6,000
// veh/h → 30 dots per km); all of them once lanes are wide enough.
const SHOWN: [number, number][] = [
  [13, 1 / 3],
  [14, 0.5],
  [15, 0.75],
  [16, 1],
];
// Screen speed (px per second) by zoom, the same for every vehicle.
const SPEED_PX: [number, number][] = [
  [11, 16],
  [14, 34],
  [16, 60],
  [18, 90],
];
const FRAME_MS = 33; // ~30 fps
const DOTS_MAX_ZOOM = 14.25; // dots fade out …
const CARS_MIN_ZOOM = 13.5; // … as cars fade in
const CAR_LENGTH_LANES = 1.7; // car size in lane widths
const CAR_WIDTH_LANES = 0.7;
const CAR_MIN_PX = 4;
const CAR_GAP = 1.3; // nose-to-nose spacing in car lengths when cars share a lane
const GLASS_MIN_LANE_PX = 5; // draw the windscreen once a car is big enough

const EMPTY: FeatureCollection = { type: "FeatureCollection", features: [] };

export interface FlowDots {
  /** animate false: draw the vehicles once, still (prefers-reduced-motion). */
  start: (animate: boolean) => void;
  stop: () => void;
  readonly on: boolean;
}

interface Seen {
  v: Vehicle;
  span: number;
  local: number; // km from the start of the span
}

export function initFlowDots(
  map: maplibregl.Map,
  edges: FeatureCollection<LineString, EdgeProps>,
  car: LaneColours,
  beforeLayer?: string,
): FlowDots {
  const c = colors();
  const carrying = edges.features.filter((f) => f.properties.flow_vph > 0 && f.geometry.coordinates.length >= 2);
  const routes = buildRoutes(
    carrying.map((f) => ({
      coords: f.geometry.coordinates,
      lanes: f.properties.lanes,
      flow: f.properties.flow_vph,
      utilisation: f.properties.utilisation,
      cut: f.properties.is_cut,
    })),
  );
  const vehicles = makeVehicles(routes, PER_KM_PER_1000_VPH, MAX_VEHICLES);

  map.addSource("flow-dots", { type: "geojson", data: EMPTY });
  map.addSource("flow-cars", { type: "geojson", data: EMPTY });
  map.addLayer(
    {
      id: "flow-dots",
      type: "circle",
      source: "flow-dots",
      maxzoom: DOTS_MAX_ZOOM,
      layout: { visibility: "none" },
      paint: {
        // The same ink dot as everywhere else; red only on the bottleneck.
        // How full a road is shows in how many dots it carries (DESIGN.md §4).
        "circle-radius": ["interpolate", ["linear"], ["zoom"], 10, 1.5, 15, 3],
        "circle-color": ["case", ["get", "cut"], c.cut, c.ink],
        "circle-opacity": ["interpolate", ["linear"], ["zoom"], CARS_MIN_ZOOM, 1, DOTS_MAX_ZOOM, 0],
      },
    },
    beforeLayer,
  );
  // A car is three short lines along the road: outline, body, windscreen.
  const fadeIn: maplibregl.ExpressionSpecification = ["interpolate", ["linear"], ["zoom"], CARS_MIN_ZOOM, 0, DOTS_MAX_ZOOM, 1];
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
  carLayer("flow-cars-edge", 0, car.carEdge, (px) => CAR_WIDTH_LANES * px + 2);
  carLayer("flow-cars", 1, car.carBody, (px) => CAR_WIDTH_LANES * px);
  carLayer("flow-cars-glass", 2, car.carGlass, (px) => Math.max(0.5, CAR_WIDTH_LANES * px - 1.6), 0.6);
  const layers = ["flow-dots", "flow-cars-edge", "flow-cars", "flow-cars-glass"];
  const dotSrc = map.getSource("flow-dots") as maplibregl.GeoJSONSource;
  const carSrc = map.getSource("flow-cars") as maplibregl.GeoJSONSource;

  const line = (a: [number, number], b: [number, number], part: number): Feature<LineString> => ({
    type: "Feature",
    geometry: { type: "LineString", coordinates: [a, b] },
    properties: { part },
  });

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

    const dotFeatures: Feature<Point>[] = [];
    const lanes = new Map<number, Seen[]>(); // road and lane → cars in it
    for (const v of vehicles) {
      if (v.rank >= shown) continue;
      const p = pointAt(v.route.track, v.at);
      if (p[0] < w || p[0] > e || p[1] < s || p[1] > n) continue;
      const si = spanAt(v.route, v.at);
      const span = v.route.spans[si]!;
      if (wantDots) {
        dotFeatures.push({
          type: "Feature",
          geometry: { type: "Point", coordinates: p },
          properties: { cut: span.cut },
        });
      }
      if (wantCars) {
        const k = span.road * 64 + laneIndex(v.share, span.lanes);
        const seen: Seen = { v, span: si, local: v.at - span.start };
        const list = lanes.get(k);
        if (list) list.push(seen);
        else lanes.set(k, [seen]);
      }
    }

    const carFeatures: Feature<LineString>[] = [];
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
          carFeatures.push(line(along(-length / 2 - mpp), along(length / 2 + mpp), 0));
          carFeatures.push(line(along(-length / 2), along(length / 2), 1));
          if (glass) carFeatures.push(line(along(length * 0.1), along(length * 0.3), 2));
        });
      }
    }

    if (wantDots || dotsDrawn) dotSrc.setData({ type: "FeatureCollection", features: dotFeatures });
    if (wantCars || carsDrawn) carSrc.setData({ type: "FeatureCollection", features: carFeatures });
    dotsDrawn = wantDots;
    carsDrawn = wantCars;
  };

  let on = false;
  let moving = false;
  let raf = 0;
  let last = 0;
  let lastDraw = 0;
  const tick = (now: number): void => {
    if (!on) return;
    const dt = last ? Math.min(now - last, 100) / 1000 : 0; // clamp after a hidden tab
    last = now;
    const zoom = map.getZoom();
    const step = (lerpStops(SPEED_PX, zoom) * metresPerPixel(zoom, map.getCenter().lat) * dt) / 1000; // km
    // advance wraps by modulo: one frame can be longer than a short route.
    for (const v of vehicles) v.at = advance(v.at, step, v.route.track.km);
    if (now - lastDraw >= FRAME_MS) {
      lastDraw = now;
      draw();
    }
    raf = requestAnimationFrame(tick);
  };
  // Standing still (reduced motion), the picture still has to follow the map.
  map.on("move", () => {
    if (on && !moving) draw();
  });

  return {
    get on() {
      return on;
    },
    start(animate) {
      if (on) return;
      on = true;
      moving = animate;
      last = 0;
      draw();
      for (const id of layers) map.setLayoutProperty(id, "visibility", "visible");
      if (animate) raf = requestAnimationFrame(tick);
    },
    stop() {
      on = false;
      cancelAnimationFrame(raf);
      for (const id of layers) map.setLayoutProperty(id, "visibility", "none");
    },
  };
}
