// "Run the analysis": replays Dinic's real max flow search on the map,
// route by route, until the bottleneck appears (decision 0011). Roads fill
// as flow is pushed; nothing here is invented — every route, amount and
// colour comes from replay.json / edges.geojson.
import maplibregl from "maplibre-gl";
import type { FeatureCollection, LineString } from "geojson";
import type { EdgeProps, Replay } from "./types";
import { COLORS } from "./colors";

// Display timings (docs/DESIGN.md §6): the whole replay fits in about
// TOTAL_MS whatever the number of routes, and never more than STEP_MAX_MS
// per route so a short trace doesn't crawl.
const TOTAL_MS = 12000;
const STEP_MAX_MS = 1800;
const DRAW_SHARE = 0.6; // share of each step spent drawing the route
const PULSES = 3;
const PULSE_MS = 450;

export interface ReplayOptions {
  map: maplibregl.Map;
  edges: FeatureCollection<LineString, EdgeProps>; // inbound, source "edges"
  replay: Replay;
  padding: () => maplibregl.PaddingOptions;
  beforeLayer?: string;
  onFinish: () => void; // start the flow dots
  onStart: () => void; // stop the flow dots
}

/** Round to the nearest 100 veh/h with thousands separators (CONTENT.md). */
export function fmtVph(n: number): string {
  return (Math.round(n / 100) * 100).toLocaleString("en-US");
}

const reducedMotion = (): boolean => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));
const frame = (): Promise<number> => new Promise((r) => requestAnimationFrame(r));

export function initReplay(o: ReplayOptions): void {
  const { map, edges, replay } = o;
  const byId = new Map<number, EdgeProps>();
  const bounds = new maplibregl.LngLatBounds();
  const cutBounds = new maplibregl.LngLatBounds();
  for (const f of edges.features) {
    byId.set(f.properties.id, f.properties);
    for (const c of f.geometry.coordinates) {
      const ll: [number, number] = [c[0]!, c[1]!];
      bounds.extend(ll);
      if (f.properties.is_cut) cutBounds.extend(ll);
    }
  }
  const cutCount = edges.features.filter((f) => f.properties.is_cut).length;

  map.addLayer(
    {
      id: "replay-path",
      type: "line",
      source: "edges",
      filter: ["in", ["get", "id"], ["literal", []]],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": COLORS.path, "line-width": 11, "line-opacity": 0.85, "line-blur": 3 },
    },
    o.beforeLayer,
  );
  map.addLayer(
    {
      id: "replay-cut",
      type: "line",
      source: "edges",
      filter: ["==", ["get", "is_cut"], true],
      layout: { "line-cap": "round", visibility: "none" },
      paint: { "line-color": COLORS.cut, "line-width": 14, "line-opacity": 0.35 },
    },
    "edges",
  );

  const section = document.getElementById("run")!;
  const runBtn = document.getElementById("run-btn") as HTMLButtonElement;
  const status = document.getElementById("run-status")!;
  const count = document.getElementById("run-count")!;
  const caption = document.getElementById("run-caption")!;
  document.getElementById("run-max")!.textContent = fmtVph(replay.max_flow_vph);
  section.hidden = false;

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
  const say = (text: string): void => {
    caption.textContent = text;
  };

  // Plain road names along a route, in order: "E5 → Jalan Kebun → E11".
  const via = (ids: number[]): string => {
    const names: string[] = [];
    for (const id of ids) {
      const p = byId.get(id);
      const n = p ? p.ref || p.name : "";
      if (n && names[names.length - 1] !== n && !names.includes(n)) names.push(n);
    }
    const short = names.length > 4 ? [...names.slice(0, 2), "…", ...names.slice(-1)] : names;
    return short.join(" → ");
  };

  const finish = (animated: boolean): void => {
    running = false;
    map.removeFeatureState({ source: "edges" });
    setPath([]);
    count.textContent = fmtVph(replay.max_flow_vph);
    const roads = cutCount === 1 ? "road is" : "roads are";
    say(
      `No route has room left. The red ${roads} the bottleneck: at most ${fmtVph(replay.max_flow_vph)} veh/h ` +
        `get from Kota Kemuning to PJ.`,
    );
    runBtn.textContent = "↻ Replay";
    if (!cutBounds.isEmpty()) map.fitBounds(cutBounds, { padding: o.padding(), maxZoom: 13, animate: animated });
    o.onFinish();
  };

  const pulseCut = async (id: number): Promise<void> => {
    map.setLayoutProperty("replay-cut", "visibility", "visible");
    for (let i = 0; i < PULSES * 2 && id === run; i++) {
      map.setPaintProperty("replay-cut", "line-opacity", i % 2 === 0 ? 0.7 : 0.15);
      await sleep(PULSE_MS / 2);
    }
    map.setPaintProperty("replay-cut", "line-opacity", 0.35);
  };

  const play = async (): Promise<void> => {
    const id = ++run;
    running = true;
    o.onStart();
    runBtn.textContent = "⏭ Skip";
    status.hidden = false;
    map.setLayoutProperty("replay-cut", "visibility", "none");
    if (reducedMotion()) {
      finish(false);
      map.setLayoutProperty("replay-cut", "visibility", "visible");
      return;
    }

    flow = new Map();
    for (const f of edges.features) setFlow(f.properties.id, 0);
    count.textContent = "0";
    say("Each round, the algorithm looks for the shortest routes from Kota Kemuning to PJ that still have room.");
    map.fitBounds(bounds, { padding: o.padding(), duration: 800 });
    await sleep(1200);

    const steps = replay.steps;
    const stepMs = Math.min(STEP_MAX_MS, TOTAL_MS / Math.max(1, steps.length));
    let pushed = 0;
    let phase = 0;
    for (const st of steps) {
      if (id !== run) return;
      const ids = st.edges.map((e) => e[0]);
      if (st.phase !== phase) phase = st.phase;
      say(`Round ${phase} of ${replay.phases}: route via ${via(ids)}, room for ${fmtVph(st.amount_vph)} veh/h.`);

      // Trace the route from Kota Kemuning towards PJ.
      const t0 = performance.now();
      const drawMs = stepMs * DRAW_SHARE;
      for (let now = t0; now - t0 < drawMs; now = await frame()) {
        if (id !== run) return;
        setPath(ids.slice(0, Math.ceil(((now - t0) / drawMs) * ids.length)));
      }
      setPath(ids);

      // Push the flow: roads change colour as they fill.
      for (const [eid, delta] of st.edges) setFlow(eid, (flow.get(eid) ?? 0) + delta);
      pushed += st.amount_vph;
      count.textContent = fmtVph(pushed);
      await sleep(stepMs * (1 - DRAW_SHARE));
      if (id !== run) return;
      setPath([]);
    }
    if (id !== run) return;
    finish(true);
    await pulseCut(id);
  };

  runBtn.addEventListener("click", () => {
    if (running) {
      run++; // stop the loop
      finish(false);
      map.setLayoutProperty("replay-cut", "visibility", "visible");
    } else {
      void play();
    }
  });
}
