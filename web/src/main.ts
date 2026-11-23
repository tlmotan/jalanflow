import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
// GeoJSON types ship with maplibre-gl (@types/geojson); no extra dependency.
import type { Feature, FeatureCollection, LineString, Point } from "geojson";
import type { EdgeProps, FixesFile, LaneDrop, LaneDropsFile, Manifest, ReplayFile } from "./types";
import { colors } from "./colors";
import {
  bottleneckSentence,
  confirmedSentence,
  dropBody,
  dropTitle,
  fixDifficulty,
  fixSentence,
  fmtVph,
  lanesText,
  pathCaption,
  resultSentence,
  roadLabel,
  type CutRoad,
} from "./copy";
import { initReplay } from "./replay";
import { initFlowDots } from "./particles";
import { initOrb } from "./orb";
import { initViews, type State, type View } from "./views";
import { LANES_FULL_ZOOM, LANES_START_ZOOM, laneLineOffset, roadWidth } from "./lanes";
import { OSM_CREDIT, baseStyle, laneColours, trial } from "./trial";

const CORRIDOR = "kemuning-pj";
// BASE_URL is "/" locally and "/jalanflow/" on GitHub Pages (vite --base).
const DATA = `${import.meta.env.BASE_URL}data/${CORRIDOR}`;

// Display constants (docs/DESIGN.md §10), not traffic numbers.
const KEY_DROPS = 10; // biggest squeezes shown at every zoom
const REST_DROPS_ZOOM = 13.5; // the other markers appear from here
const QUIET = 0.35; // lane-drop markers step back once the analysis has run
const DRIVE_OPACITY = 0.5; // the drive before the analysis
const DRIVE_EMPTY = 0.16; // a road with no flow yet, during the replay
const DRIVE_FILL = 0.5; // added to DRIVE_EMPTY as a road fills
const DRIVE_RESULT = 0.2; // the drive under the flow dots
const HIT_RADIUS = 14; // tap target around a marker, px
const SHIELD_MIN_KM = 3; // an expressway gets a route marker from this much of the drive
const SKIP_GUARD_MS = 600; // a double tap on Run must not reach Skip or zoom the map
const PHONE = "(max-width: 700px)";

async function fetchJSON<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(path);
    if (!res.ok) return null;
    return (await res.json()) as T;
  } catch {
    return null;
  }
}

// Start every data download now, so they run alongside the map style and
// tiles instead of queueing one by one after the map has loaded.
// fetchJSON never rejects, so these are safe to leave un-awaited.
const dropsData = fetchJSON<LaneDropsFile>(`${DATA}/lane_drops.json`);
const edgesData = fetchJSON<FeatureCollection<LineString, EdgeProps>>(`${DATA}/edges.geojson`);
const fixesData = fetchJSON<FixesFile>(`${DATA}/fixes.json`);
const replayData = fetchJSON<ReplayFile>(`${DATA}/replay.json`);
const manifestData = fetchJSON<Manifest>(`${DATA}/manifest.json`);

const look = trial();
const c = colors();
const lane = laneColours(look.lanes);

function byId<T extends HTMLElement = HTMLElement>(id: string): T {
  return document.getElementById(id) as T;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, text?: string, cls?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (text !== undefined) e.textContent = text;
  if (cls) e.className = cls;
  return e;
}

const still = (): boolean => window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const phone = (): boolean => window.matchMedia(PHONE).matches;

// ---------- the text column: needs no map, so it fills as data arrives ----------

const orb = initOrb(byId<HTMLCanvasElement>("orb"));
// Set once the map layers exist; until then a view change only moves text.
let styleMap: (view: View, state: State) => void = () => {};
const views = initViews((view, state) => styleMap(view, state));

document.querySelectorAll<HTMLElement>("[data-open-how]").forEach((b) => {
  b.addEventListener("click", () => views.open("how"));
});
document.querySelectorAll<HTMLElement>("[data-open-list]").forEach((b) => {
  b.addEventListener("click", () => views.open("list"));
});

let selectDrop: (d: LaneDrop | null) => void = () => {};
let flyToDrop: (d: LaneDrop) => void = () => {};

function coords(d: LaneDrop): string {
  return `${d.pos.lat.toFixed(5)},${d.pos.lon.toFixed(5)}`;
}

function link(href: string, text: string): HTMLAnchorElement {
  const a = el("a", text);
  a.href = href;
  a.target = "_blank";
  a.rel = "noopener";
  return a;
}

function roadsLine(d: LaneDrop): string {
  const side = (roads: LaneDrop["from"]): string =>
    roads.map((r) => `${roadLabel(r)} (${r.lanes}${r.lanes_inferred ? ", guessed" : ""})`).join(" + ");
  return `In: ${side(d.from)}. Out: ${side(d.to)}.`;
}

function openDrop(d: LaneDrop): void {
  if (views.state === "running") return;
  byId("detail-title").textContent = dropTitle(d);
  byId("detail-body").textContent = dropBody(d);
  byId("detail-roads").textContent = roadsLine(d);
  const notes: string[] = [d.kind === "merge" ? "Roads join here and lanes are lost." : "The road loses lanes here."];
  if (d.confirmed) notes.push(`A local driver confirmed this as a daily jam (${d.confirmed.source}).`);
  if (d.lanes_inferred) notes.push("Some lane counts here are guessed from the road type. Check on Street View.");
  byId("detail-note").textContent = notes.join(" ");
  const [osmType, osmId] = d.key.startsWith("node:")
    ? ["node", d.key.slice(5)]
    : ["way", String(d.to[0]?.osm_way_id ?? "")];
  byId("detail-links").replaceChildren(
    link(`https://www.google.com/maps/@?api=1&map_action=pano&viewpoint=${coords(d)}`, "Look at it in Street View"),
    link(`https://www.google.com/maps/search/?api=1&query=${coords(d)}`, "Google Maps"),
    link(`https://www.openstreetmap.org/${osmType}/${osmId}`, "OpenStreetMap"),
  );
  views.open("detail");
  selectDrop(d);
  flyToDrop(d);
}

function listRow(d: LaneDrop): HTMLLIElement {
  const li = el("li");
  const btn = el("button", undefined, "row");
  btn.type = "button";
  btn.append(el("b", lanesText(d).replace(" lanes", "")), el("span", dropTitle(d)));
  if (d.confirmed) btn.append(el("em", "Confirmed jam"));
  else if (d.lanes_inferred) btn.append(el("em", "Lanes guessed"));
  btn.addEventListener("click", () => openDrop(d));
  li.append(btn);
  return li;
}

function renderDrops(f: LaneDropsFile): void {
  const n = f.drops.length;
  const confirmed = f.drops.filter((d) => d.confirmed).length;
  byId("lede").replaceChildren(
    el("b", String(n)),
    ` places around KESAS and LDP where more lanes come in than carry on. ${confirmedSentence(confirmed)}`,
  );
  document.querySelectorAll<HTMLElement>("[data-open-list]").forEach((b) => {
    b.textContent = `See all ${n} lane drops`;
  });
  byId("list").append(...f.drops.map(listRow));
}

// "How this works": the lane-drop notes, then the capacity notes, then the data date.
async function renderHow(drops: LaneDropsFile | null): Promise<void> {
  const how = byId("how");
  const manifest = await manifestData;
  const notes = [...(drops?.assumptions ?? []), ...(manifest?.assumptions ?? [])];
  how.append(...notes.map((a) => el("li", a)));
  if (drops) how.append(el("li", drops.ranking), el("li", `OpenStreetMap data from ${drops.osm_date}.`));
}

const textReady = dropsData.then((drops) => {
  if (drops) renderDrops(drops);
  else byId("lede").textContent = "No data yet: run `make analyze`.";
  void renderHow(drops);
  return drops;
});

// ---------- the map ----------

const map = new maplibregl.Map({
  container: "map",
  style: await baseStyle(look.base),
  center: [101.58, 3.06],
  zoom: 11.5,
  // The credit folds into a button on phones, where it would wrap.
  attributionControl: { compact: phone() },
});
map.addControl(new maplibregl.NavigationControl(), "top-right");
// On phones the map shrinks and grows with the text below it.
new ResizeObserver(() => map.resize()).observe(map.getContainer());

// Keep fitted features clear of the text column, measured from the column
// itself (DESIGN.md §5). On phones the map already ends where the text starts.
function textPadding(): Required<maplibregl.PaddingOptions> {
  if (phone()) return { top: 40, bottom: 40, left: 36, right: 36 };
  const right = byId("text").getBoundingClientRect().right;
  return { top: 70, bottom: 70, left: right + 70, right: 70 };
}

/** Screen offset that lands a point in the middle of the space the text leaves. */
function clearOffset(): [number, number] {
  const p = textPadding();
  return [(p.left - p.right) / 2, (p.top - p.bottom) / 2];
}

// A label in the page's own type, pinned to the map.
function pin(node: Element, at: [number, number], anchor: maplibregl.PositionAnchor, offset: [number, number]): void {
  new maplibregl.Marker({ element: node as HTMLElement, anchor, offset }).setLngLat(at).addTo(map);
}

// Markers, bottom first.
const DROP_TIERS = ["drops-rest", "drops-key"] as const;
const DROP_LAYERS = ["drops-rest", "drops-key", "drops-core"] as const;
let dropLayersReady = false;

/** The list is open: every marker at every zoom. Otherwise the zoom rule (DESIGN.md §1). */
function showAllDrops(all: boolean): void {
  if (!dropLayersReady) return;
  for (const id of ["drops-rest", "drops-rest-hit"]) map.setLayerZoomRange(id, all ? 0 : REST_DROPS_ZOOM, 24);
}

function addDropLayers(f: LaneDropsFile): void {
  const data: FeatureCollection<Point> = {
    type: "FeatureCollection",
    features: f.drops.map((d, i) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [d.pos.lon, d.pos.lat] },
      properties: { i, rank: d.rank, confirmed: d.confirmed !== null, guessed: d.lanes_inferred },
    })),
  };
  map.addSource("drops", { type: "geojson", data });
  // Two tiers so the roads stay readable: the confirmed spots and the
  // biggest squeezes always show; the rest appear on zooming in.
  const key: maplibregl.ExpressionSpecification = ["any", ["get", "confirmed"], ["<=", ["get", "rank"], KEY_DROPS]];
  const open: maplibregl.ExpressionSpecification = ["any", ["get", "confirmed"], ["get", "guessed"]];
  for (const tier of DROP_TIERS) {
    const filter: maplibregl.FilterSpecification = tier === "drops-key" ? key : ["!", key];
    const minzoom = tier === "drops-key" ? 0 : REST_DROPS_ZOOM;
    // The same ink dot three ways: ringed = confirmed jam, solid = lane
    // drop, hollow = lane count guessed. Never red (DESIGN.md §4).
    map.addLayer({
      id: tier,
      type: "circle",
      source: "drops",
      filter,
      minzoom,
      paint: {
        "circle-radius": ["case", ["get", "confirmed"], 7, ["get", "guessed"], 3, 3.5],
        "circle-color": ["case", open, c.bg, c.ink],
        "circle-stroke-color": ["case", open, c.ink, c.bg],
        "circle-stroke-width": ["case", ["get", "confirmed"], 1.5, ["get", "guessed"], 1.3, 1],
      },
    });
    map.addLayer({
      id: `${tier}-hit`,
      type: "circle",
      source: "drops",
      filter,
      minzoom,
      paint: { "circle-radius": HIT_RADIUS, "circle-color": c.bg, "circle-opacity": 0.01 },
    });
    map.on("click", `${tier}-hit`, (e) => {
      const i = e.features?.[0]?.properties?.["i"];
      const d = typeof i === "number" ? f.drops[i] : undefined;
      if (d) openDrop(d);
    });
    map.on("mouseenter", `${tier}-hit`, () => (map.getCanvas().style.cursor = "pointer"));
    map.on("mouseleave", `${tier}-hit`, () => (map.getCanvas().style.cursor = ""));
  }
  // The dot inside a confirmed jam's ring.
  map.addLayer({
    id: "drops-core",
    type: "circle",
    source: "drops",
    filter: ["get", "confirmed"],
    paint: { "circle-radius": 3, "circle-color": c.ink },
  });
  // The lane drop whose detail is open.
  map.addLayer({
    id: "drops-selected",
    type: "circle",
    source: "drops",
    filter: ["==", ["get", "i"], -1],
    paint: { "circle-radius": 12, "circle-opacity": 0, "circle-stroke-color": c.ink, "circle-stroke-width": 2 },
  });
  selectDrop = (d) => map.setFilter("drops-selected", ["==", ["get", "i"], d ? f.drops.indexOf(d) : -1]);
  flyToDrop = (d) =>
    map.flyTo({ center: [d.pos.lon, d.pos.lat], zoom: Math.max(map.getZoom(), 15), offset: clearOffset() });
  for (const d of f.drops) {
    if (d.confirmed) pin(el("div", "Confirmed jam", "tag jam"), [d.pos.lon, d.pos.lat], "left", [13, 0]);
  }
  dropLayersReady = true;

  const bounds = new maplibregl.LngLatBounds();
  for (const d of f.drops) bounds.extend([d.pos.lon, d.pos.lat]);
  if (!bounds.isEmpty()) map.fitBounds(bounds, { padding: textPadding(), animate: false });
}

/**
 * How dark the drive is drawn when zoomed out, by state. Once lanes are
 * drawn (LANES_FULL_ZOOM) the road surface is solid in every state.
 */
function driveOpacity(state: State): maplibregl.ExpressionSpecification {
  const filling: maplibregl.ExpressionSpecification = [
    "case",
    ["==", ["typeof", ["feature-state", "f"]], "number"],
    ["+", DRIVE_EMPTY, ["*", DRIVE_FILL, ["min", 1, ["/", ["feature-state", "f"], ["get", "capacity_vph"]]]]],
    DRIVE_EMPTY,
  ];
  // Roads whose flow a later route cancelled end at 0: seen in the replay only.
  const carrying = (shown: number): maplibregl.ExpressionSpecification => ["case", ["<=", ["get", "flow_vph"], 0], 0, shown];
  const far = state === "running" ? filling : carrying(state === "result" ? DRIVE_RESULT : DRIVE_OPACITY);
  const near = state === "running" ? 1 : carrying(1);
  return ["interpolate", ["linear"], ["zoom"], LANES_START_ZOOM, far, LANES_FULL_ZOOM, near];
}

/** Kilometres along a line (equirectangular: fine at corridor scale). */
function lineKm(line: number[][]): number {
  let km = 0;
  for (let i = 1; i < line.length; i++) {
    const [lon1, lat1] = line[i - 1] as [number, number];
    const [lon2, lat2] = line[i] as [number, number];
    const x = (lon2 - lon1) * Math.cos(((lat1 + lat2) / 2) * (Math.PI / 180));
    km += Math.hypot(x, lat2 - lat1) * 111.32;
  }
  return km;
}

function midpoint(line: number[][]): [number, number] {
  const m = line[Math.floor(line.length / 2)] ?? line[0]!;
  return [m[0]!, m[1]!];
}

// Malaysian expressway route marker: yellow hexagon, ink number.
function shield(ref: string): SVGSVGElement {
  const NS = "http://www.w3.org/2000/svg";
  const w = ref.length > 2 ? 18 : 15;
  const h = 9.5;
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("class", "shield");
  svg.setAttribute("width", String(2 * w + 4));
  svg.setAttribute("height", String(2 * h + 4));
  svg.setAttribute("viewBox", `${-w - 2} ${-h - 2} ${2 * w + 4} ${2 * h + 4}`);
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", `Expressway ${ref}`);
  const hex = document.createElementNS(NS, "polygon");
  hex.setAttribute("points", `${-w},0 ${-w + 6},${-h} ${w - 6},${-h} ${w},0 ${w - 6},${h} ${-w + 6},${h}`);
  const text = document.createElementNS(NS, "text");
  text.setAttribute("y", "4.5");
  text.textContent = ref;
  svg.append(hex, text);
  return svg;
}

// One route marker per expressway that carries a real share of the drive,
// beside its longest stretch.
function addShields(drive: Feature<LineString, EdgeProps>[]): void {
  const byRef = new Map<string, { km: number; longest: Feature<LineString, EdgeProps>; longestKm: number }>();
  for (const f of drive) {
    const ref = f.properties.ref;
    if (!/^E\d+$/.test(ref)) continue;
    const km = lineKm(f.geometry.coordinates);
    const got = byRef.get(ref);
    if (!got) byRef.set(ref, { km, longest: f, longestKm: km });
    else {
      got.km += km;
      if (km > got.longestKm) [got.longest, got.longestKm] = [f, km];
    }
  }
  for (const [ref, r] of byRef) {
    if (r.km >= SHIELD_MIN_KM) pin(shield(ref), midpoint(r.longest.geometry.coordinates), "center", [0, -20]);
  }
}

// Max flow layers: drawn once cmd/analyze has written edges.geojson.
// The site shows the inbound direction (Kota Kemuning → PJ).
async function addFlowLayers(): Promise<void> {
  const all = await edgesData;
  if (!all) return;
  const inbound = all.features.filter((f) => f.properties.dir === "inbound");
  const drive = inbound.filter((f) => f.properties.flow_vph > 0);
  const edges: FeatureCollection<LineString, EdgeProps> = { type: "FeatureCollection", features: inbound };
  const before = map.getLayer(DROP_TIERS[0]) ? DROP_TIERS[0] : undefined;

  // Every other modelled road, quiet.
  map.addSource("roads-quiet", {
    type: "geojson",
    data: { type: "FeatureCollection", features: all.features.filter((f) => f.properties.dir !== "inbound") },
  });
  map.addLayer(
    {
      id: "roads-quiet",
      type: "line",
      source: "roads-quiet",
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": c.road, "line-width": ["interpolate", ["linear"], ["zoom"], 11, 1, 15, 2.5] },
    },
    before,
  );

  // promoteId lets the replay darken roads by feature-state "f" (flow so far).
  map.addSource("edges", {
    type: "geojson",
    data: edges,
    promoteId: "id",
    // With no basemap, our own roads carry the OSM credit (AGENTS.md).
    ...(look.base === "none" ? { attribution: OSM_CREDIT } : {}),
  });
  map.addLayer(
    {
      id: "edges",
      type: "line",
      source: "edges",
      // Round ends close the wedge between two roads meeting at an angle
      // once they are drawn many pixels wide.
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        // One ink line when zoomed out; lanes × lane width when zoomed in.
        "line-width": roadWidth(),
        "line-color": ["interpolate", ["linear"], ["zoom"], LANES_START_ZOOM, c.ink, LANES_FULL_ZOOM, lane.surface],
        "line-opacity": driveOpacity(views.state),
        // Guessed lane counts: dashed line when zoomed out. Once lanes
        // are drawn the surface is solid and the road edge is dashed.
        "line-dasharray": [
          "step",
          ["zoom"],
          ["case", ["get", "lanes_inferred"], ["literal", [2, 2]], ["literal", [1, 0]]],
          LANES_FULL_ZOOM,
          ["literal", [1, 0]],
        ],
      },
    },
    before,
  );
  addLaneLayers(edges, before);
  addShields(drive);

  const replay = (await replayData)?.inbound;
  if (!replay) return;

  // The two ends of the drive, from the first route the search found.
  const ends = replay.steps[0]?.edges.map((e) => e[0]) ?? [];
  const edgeById = new Map(inbound.map((f) => [f.properties.id, f]));
  const first = edgeById.get(ends[0] ?? -1)?.geometry.coordinates[0];
  const last = edgeById.get(ends[ends.length - 1] ?? -1)?.geometry.coordinates.at(-1);
  if (first) pin(el("div", "Kota Kemuning", "place"), [first[0]!, first[1]!], "top-left", [-10, 14]);
  if (last) pin(el("div", "Petaling Jaya", "place"), [last[0]!, last[1]!], "bottom-right", [10, -10]);

  const cutEdges = inbound.filter((f) => f.properties.is_cut);
  const cut: CutRoad[] = cutEdges.map((f) => f.properties);
  for (const f of cutEdges) {
    const lanes = `${f.properties.lanes} lane${f.properties.lanes === 1 ? "" : "s"}`;
    pin(el("div", `Bottleneck, ${lanes}`, "tag cut"), midpoint(f.geometry.coordinates), "bottom-left", [12, -8]);
  }

  // The result, as sentences (CONTENT.md).
  const fixes = (await fixesData)?.inbound ?? [];
  const say = resultSentence(replay.max_flow_vph);
  byId("result-say").textContent = say;
  byId("result-cut").textContent = bottleneckSentence(cut);
  const [beforeNum, num, afterNum] = fixSentence(fixes, cut);
  byId("result-fix").replaceChildren(beforeNum, el("b", num), afterNum);
  byId("result-fix-why").textContent = fixDifficulty(fixes);

  const dots = initFlowDots(map, edges, lane, before);
  const flowBtn = byId<HTMLButtonElement>("flow-btn");
  const setDots = (on: boolean): void => {
    if (on) dots.start(!still());
    else dots.stop();
    flowBtn.setAttribute("aria-pressed", String(on));
    flowBtn.textContent = on ? "Hide flow" : "Show flow";
  };
  flowBtn.addEventListener("click", () => setDots(!dots.on));

  const count = byId("run-count");
  const live = byId("live");
  let startedAt = 0;
  const controls = initReplay({
    map,
    edges,
    replay,
    padding: textPadding,
    // Under the lane lines and the flow dots, straight on the road.
    beforeLayer: map.getLayer("lane-lines-1") ? "lane-lines-1" : before,
    onStart: () => {
      startedAt = performance.now();
      // The second tap of a double tap on Run can land on the map where
      // the button was: it must not zoom the map away from the replay.
      map.doubleClickZoom.disable();
      window.setTimeout(() => map.doubleClickZoom.enable(), SKIP_GUARD_MS);
      selectDrop(null);
      setDots(false);
      views.home();
      views.setState("running");
      count.textContent = "0";
      orb.setDisorder(1);
      byId("skip-btn").focus();
    },
    onProgress: (pushed, step, steps) => {
      count.textContent = fmtVph(pushed);
      byId("run-caption").textContent = pathCaption(step + 1, steps);
      live.textContent = `${fmtVph(pushed)} vehicles an hour so far.`;
      // Disorder = 1 − found ÷ max; never fully ordered until the end.
      orb.setDisorder(Math.max(0.12, 1 - pushed / Math.max(1, replay.max_flow_vph)));
    },
    onFinish: () => {
      orb.setDisorder(0);
      views.setState("result");
      setDots(true);
      live.textContent = say;
      byId("replay-btn").focus();
    },
  });
  const runBtn = byId<HTMLButtonElement>("run-btn");
  runBtn.addEventListener("click", controls.play);
  byId("replay-btn").addEventListener("click", controls.play);
  byId("skip-btn").addEventListener("click", () => {
    if (performance.now() - startedAt >= SKIP_GUARD_MS) controls.skip();
  });
  runBtn.hidden = false;
}

// Road edge and lane lines, drawn once the map is close enough for lanes
// (lanes.ts, decision 0012). Lanes are equal stripes from the OSM lane
// count: OSM does not say where each lane actually runs.
function addLaneLayers(edges: FeatureCollection<LineString, EdgeProps>, before?: string): void {
  const fadeIn: maplibregl.ExpressionSpecification = [
    "interpolate", ["linear"], ["zoom"], LANES_START_ZOOM, 0, LANES_FULL_ZOOM, 1,
  ];
  const shown: maplibregl.FilterSpecification = [">", ["get", "flow_vph"], 0];
  for (const guessed of [false, true]) {
    map.addLayer(
      {
        id: guessed ? "edges-casing-guessed" : "edges-casing",
        type: "line",
        source: "edges",
        minzoom: LANES_START_ZOOM,
        filter: ["all", shown, ["==", ["get", "lanes_inferred"], guessed]],
        layout: { "line-cap": guessed ? "butt" : "round", "line-join": "round" },
        paint: {
          "line-color": lane.edge,
          "line-width": roadWidth(2.5),
          "line-opacity": fadeIn,
          ...(guessed ? { "line-dasharray": [0.6, 0.4] } : {}),
        },
      },
      "edges",
    );
  }
  const most = Math.max(1, ...edges.features.map((f) => f.properties.lanes));
  for (let k = 1; k < most; k++) {
    map.addLayer(
      {
        id: `lane-lines-${k}`,
        type: "line",
        source: "edges",
        minzoom: LANES_FULL_ZOOM,
        filter: ["all", shown, [">", ["get", "lanes"], k]],
        paint: {
          "line-color": lane.laneLine,
          "line-width": ["interpolate", ["linear"], ["zoom"], LANES_FULL_ZOOM, 0.5, 16, 1, 18, 1.5],
          "line-opacity": ["interpolate", ["linear"], ["zoom"], LANES_FULL_ZOOM, 0, LANES_FULL_ZOOM + 0.5, 0.75],
          "line-dasharray": [6, 6],
          "line-offset": laneLineOffset(k),
        },
      },
      before,
    );
  }
}

map.on("load", async () => {
  // The map can be created before style.css sizes #map; measure again.
  map.resize();
  // Text first: the map is fitted to the space the column leaves.
  const drops = await textReady;
  if (drops) addDropLayers(drops);
  await addFlowLayers();

  styleMap = (view, state) => {
    showAllDrops(view === "list");
    if (view !== "detail") selectDrop(null);
    if (map.getLayer("edges")) map.setPaintProperty("edges", "line-opacity", driveOpacity(state));
    if (!dropLayersReady) return;
    // Markers lead on the landing view and in the list; after the
    // analysis they step back behind the result.
    const quiet = state === "home" || view !== "main" ? 1 : QUIET;
    for (const id of DROP_LAYERS) {
      map.setPaintProperty(id, "circle-opacity", quiet);
      map.setPaintProperty(id, "circle-stroke-opacity", quiet);
    }
  };
  styleMap(views.view, views.state);
});
