// "Run the analysis": replays Dinic's real max flow search on the map,
// route by route, until the bottleneck appears (decision 0011). The drive
// darkens as flow is pushed; nothing here is invented — every route and
// amount comes from replay.json / edges.geojson.
//
// This module owns the map side only. The text column (counter, buttons,
// views) is driven from main.ts through the callbacks.
import maplibregl from "maplibre-gl";
import type { FeatureCollection, LineString } from "geojson";
import type { EdgeProps, Replay } from "./types";
import { colors } from "./colors";
import { roadWidth } from "./lanes";
import { NEARLY_FULL } from "./copy";

// Display timings (docs/design/project/DESIGN_BRIEF.md §6): the whole replay fits in about
// TOTAL_MS whatever the number of routes, and never more than STEP_MAX_MS
// per route so a short trace doesn't crawl.
const TOTAL_MS = 12000;
const STEP_MAX_MS = 1800;
const DRAW_SHARE = 0.6; // share of each step spent drawing the route
const PULSES = 3;
const PULSE_MS = 450;
const CUT_HALO_OPACITY = 0.25;

export interface ReplayOptions {
  map: maplibregl.Map;
  edges: FeatureCollection<LineString, EdgeProps>; // inbound, source "edges"
  replay: Replay;
  padding: () => maplibregl.PaddingOptions;
  beforeLayer?: string;
  /** The search starts (or restarts) from zero. */
  onStart: () => void;
  /** After each route: veh/h found so far, and which route of how many. */
  onProgress: (pushedVph: number, step: number, steps: number) => void;
  /** The result is on the map: by finishing, skipping or reduced motion. */
  onFinish: () => void;
}

export interface ReplayControls {
  /** Run from the start. Does nothing while a run is in progress. */
  play: () => void;
  /** Jump to the result. Does nothing unless a run is in progress. */
  skip: () => void;
  /** Show the result without replaying the search (a shared link to it). */
  result: () => void;
}

const reducedMotion = (): boolean => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));
const frame = (): Promise<number> => new Promise((r) => requestAnimationFrame(r));

export function initReplay(o: ReplayOptions): ReplayControls {
  const { map, edges, replay } = o;
  const c = colors();
  const bounds = new maplibregl.LngLatBounds();
  for (const f of edges.features) {
    for (const p of f.geometry.coordinates) bounds.extend([p[0]!, p[1]!]);
  }

  // The route being traced: solid ink, a little wider than the road.
  map.addLayer(
    {
      id: "replay-path",
      type: "line",
      source: "edges",
      filter: ["in", ["get", "id"], ["literal", []]],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": c.ink, "line-width": roadWidth(1.5) },
    },
    o.beforeLayer,
  );
  // The bottleneck, shown only once the search has finished: a soft halo
  // under the road (it pulses) and the road itself in red.
  map.addLayer(
    {
      id: "replay-cut",
      type: "line",
      source: "edges",
      filter: ["==", ["get", "is_cut"], true],
      layout: { "line-cap": "round", visibility: "none" },
      paint: { "line-color": c.cut, "line-width": roadWidth(10), "line-opacity": CUT_HALO_OPACITY },
    },
    "edges",
  );
  // Roads the analysis fills to NEARLY_FULL or more, shown with the result.
  map.addLayer(
    {
      id: "full-line",
      type: "line",
      source: "edges",
      filter: ["all", [">=", ["get", "utilisation"], NEARLY_FULL], ["!", ["get", "is_cut"]], [">", ["get", "flow_vph"], 0]],
      layout: { "line-cap": "round", "line-join": "round", visibility: "none" },
      paint: { "line-color": c.full, "line-width": roadWidth(3) },
    },
    "edges",
  );
  map.addLayer(
    {
      id: "cut-line",
      type: "line",
      source: "edges",
      filter: ["==", ["get", "is_cut"], true],
      layout: { "line-cap": "round", "line-join": "round", visibility: "none" },
      paint: { "line-color": c.cut, "line-width": roadWidth(3) },
    },
    o.beforeLayer,
  );
  const showCut = (on: boolean): void => {
    for (const id of ["replay-cut", "full-line", "cut-line"]) map.setLayoutProperty(id, "visibility", on ? "visible" : "none");
  };

  let run = 0; // bumped on every start / skip; stale loops stop
  let running = false;
  let flow = new Map<number, number>();

  const setFlow = (id: number, f: number): void => {
    flow.set(id, f);
    map.setFeatureState({ source: "edges", id }, { f });
  };
  const setPath = (ids: number[]): void => {
    map.setFilter("replay-path", ["in", ["get", "id"], ["literal", ids]]);
  };

  const finish = (animated: boolean): void => {
    running = false;
    map.removeFeatureState({ source: "edges" });
    setPath([]);
    showCut(true);
    // The whole drive, so the bottleneck is seen against the trip it limits.
    if (!bounds.isEmpty()) map.fitBounds(bounds, { padding: o.padding(), animate: animated });
    o.onFinish();
  };

  const pulseCut = async (id: number): Promise<void> => {
    for (let i = 0; i < PULSES * 2 && id === run; i++) {
      map.setPaintProperty("replay-cut", "line-opacity", i % 2 === 0 ? 0.6 : 0.1);
      await sleep(PULSE_MS / 2);
    }
    map.setPaintProperty("replay-cut", "line-opacity", CUT_HALO_OPACITY);
  };

  const play = async (): Promise<void> => {
    const id = ++run;
    running = true;
    showCut(false);
    o.onStart();
    if (reducedMotion()) {
      finish(false);
      return;
    }

    flow = new Map();
    for (const f of edges.features) setFlow(f.properties.id, 0);
    const steps = replay.steps;
    o.onProgress(0, 0, steps.length);
    map.fitBounds(bounds, { padding: o.padding(), duration: 800 });
    await sleep(1200);

    const stepMs = Math.min(STEP_MAX_MS, TOTAL_MS / Math.max(1, steps.length));
    let pushed = 0;
    for (const [n, st] of steps.entries()) {
      if (id !== run) return;
      const ids = st.edges.map((e) => e[0]);

      // Trace the route from Kota Kemuning towards PJ.
      const t0 = performance.now();
      const drawMs = stepMs * DRAW_SHARE;
      for (let now = t0; now - t0 < drawMs; now = await frame()) {
        if (id !== run) return;
        setPath(ids.slice(0, Math.ceil(((now - t0) / drawMs) * ids.length)));
      }
      setPath(ids);

      // Push the flow: roads darken as they fill.
      for (const [eid, delta] of st.edges) setFlow(eid, (flow.get(eid) ?? 0) + delta);
      pushed += st.amount_vph;
      o.onProgress(pushed, n + 1, steps.length);
      await sleep(stepMs * (1 - DRAW_SHARE));
      if (id !== run) return;
      setPath([]);
    }
    if (id !== run) return;
    finish(true);
    await pulseCut(id);
  };

  return {
    play() {
      if (!running) void play();
    },
    skip() {
      if (!running) return;
      run++; // stop the loop
      finish(false);
    },
    result() {
      run++;
      o.onStart();
      finish(false);
    },
  };
}
