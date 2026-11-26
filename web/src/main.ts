// The site (web/index.html): a dark shell, a light map, the capacity as the
// main finding, one panel view at a time (decision 0016). Every number comes
// from the published data files.
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import type { Feature, FeatureCollection, LineString, Point } from "geojson";
import type { Direction, EdgeProps, FixesFile, LaneDrop, LaneDropsFile, Manifest, ReplayFile } from "./types";
import { colors } from "./colors";
import { OSM_CREDIT, baseStyle } from "./basemap";
import { initFlow, type Flow } from "./flow";
import { LANES_FULL_ZOOM, laneLineOffset, roadWidth } from "./lanes";
import { dropSlug, hashFor, parseHash, stopsRun } from "./links";
import { sharedRoads, stretches } from "./profile";
import { NEARLY_FULL, countWord, fmtExact, ROUTES_DONE, routeCaption, routeProgress, profileLabel, profileSentence, profileTitle, sharedName, bottleneckSentence, confirmedSentence, dropSentence, dropShields, dropTitle, expresswayRefs, fixDifficulty, fixSentence, fmtVph, lanesText, resultLine, roadLabel, roadTitle, whatIfBars } from "./copy";
import { laneDiagram } from "./lanediagram";

const DATA = `${import.meta.env.BASE_URL}data/kemuning-pj`;

// Display constants, not traffic numbers.
const KEY_DROPS = 10; // biggest squeezes shown at every zoom
const REST_DROPS_ZOOM = 13; // the other markers appear from here
const SHIELD_MIN_KM = 3; // an expressway gets a route marker from this much of the drive
const DROP_HIT = 12; // tap reach around a marker, px
const ROAD_HIT = 7; // tap reach around a road, px
const FOCUS_ZOOM = 14.5; // closest the map goes when showing one feature
const KEEP_ZOOM = 13.5; // a feature already in view at this zoom or closer keeps the reader's camera
const CAMERA_MS = 350; // one short move between views; none with reduced motion
const EDGE_GAP = 8; // what the camera keeps between the map's controls and a road, px
const RUN_TOTAL_MS = 10000; // the whole search, whatever the number of routes
const RUN_STEP_MAX_MS = 1100; // and never slower than this per route
const CARD_MIN_MAP_W = 320; // the map card needs this much map to sit in, px
// The tallest card (a lane drop with a lane diagram, a guessed flag and a
// long name) is about 360 px; with its 16 px offset, tip and edge gaps it
// needs about 400 px of map. A smaller map shows the card in the sheet.
const CARD_MIN_MAP_H = 420;
const CARD_LINGER_MS = 280; // time to move the pointer from a marker onto its card
const DRAG_SNAP = 48; // a drag of the phone sheet's handle this long leaves the height it started at, px
const DRAG_FLICK = 0.5; // or one this fast, px per ms
const DRAG_TAP = 6; // anything shorter is a tap, px
const SHEET_MIN = 96; // the least a dragged sheet shows: its handle and a line or two, px
const SHEET_MS = 220; // the sheet settling at a height; --dur-sheet in style.css
const PHONE = "(max-width: 760px) and (orientation: portrait), (max-width: 560px)";

type View = "summary" | "list" | "detail" | "how";
/** Before, during and after "Run the analysis". Nothing computed shows before it. */
type State = "home" | "running" | "result";
type Edge = Feature<LineString, EdgeProps>;

const ENDS: Record<Direction, { from: string; to: string }> = {
  inbound: { from: "Kota Kemuning", to: "Petaling Jaya" },
  outbound: { from: "Petaling Jaya", to: "Kota Kemuning" },
};

const still = (): boolean => window.matchMedia("(prefers-reduced-motion: reduce)").matches;
const phone = (): boolean => window.matchMedia(PHONE).matches;
const canHover = (): boolean => window.matchMedia("(hover: hover)").matches;

function byId<T extends HTMLElement = HTMLElement>(id: string): T {
  return document.getElementById(id) as T;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, text?: string, cls?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (text !== undefined) e.textContent = text;
  if (cls) e.className = cls;
  return e;
}

function link(href: string, text: string): HTMLAnchorElement {
  const a = el("a", text);
  a.href = href;
  a.target = "_blank";
  a.rel = "noopener";
  return a;
}

async function fetchJSON<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(path);
    if (!res.ok) return null;
    return (await res.json()) as T;
  } catch {
    return null;
  }
}

// ---------- state ----------

const opened = parseHash(window.location.hash);
let dir = opened.dir;
let view: View = opened.view;
let shownDrop: LaneDrop | null = null; // the lane drop in the detail view, for the link
let state: State = opened.solved ? "result" : "home";
let flow: Flow | null = null; // set once the map has loaded
let flowOn = true; // the reader's choice, kept across runs and directions
let runId = 0;
/** The one quick look that is open: a card on the map, or on phones a block in the sheet. */
let card: { popup: maplibregl.Popup | null; node: HTMLElement; what: LaneDrop | Edge; pinned: boolean; at: [number, number] } | null = null;
let cardTimer = 0; // closes a hovered card shortly after the pointer leaves // a newer run, a skip or a direction switch cancels the one in progress
let opener: HTMLElement | null = null; // where focus goes back to
let userMoved = false; // the reader moved the map since the view last framed it
/** Phones: the sheet's three heights (decision 0018). Summary is the default. */
type Sheet = "collapsed" | "summary" | "expanded";
let mini = false; // the phone sheet is collapsed to its handle and one line
let settleH: number | null = null; // the height a settling sheet is on its way to, px
let settleTimer = 0;
let mapReady = false; // the panel never waits for the map
const rowOf = new Map<LaneDrop, HTMLButtonElement>();

function syncLink(): void {
  const place = {
    dir,
    view: view === "detail" ? ("summary" as const) : view,
    solved: state === "result",
    drop: view === "detail" && shownDrop ? dropSlug(shownDrop.key) : null,
  };
  window.history.replaceState(null, "", hashFor(place) || window.location.pathname + window.location.search);
}

function say(text: string): void {
  byId("live").textContent = text;
}

// ---------- load ----------

byId("retry-btn").addEventListener("click", () => window.location.reload());

const [edgesFile, manifest, dropsFile, fixes, replay, base] = await Promise.all([
  fetchJSON<FeatureCollection<LineString, EdgeProps>>(`${DATA}/edges.geojson`),
  fetchJSON<Manifest>(`${DATA}/manifest.json`),
  fetchJSON<LaneDropsFile>(`${DATA}/lane_drops.json`),
  fetchJSON<FixesFile>(`${DATA}/fixes.json`),
  fetchJSON<ReplayFile>(`${DATA}/replay.json`),
  baseStyle(),
]);

if (!edgesFile || !manifest) {
  // No invented numbers: without the model's files the page says so.
  document.body.dataset.load = "failed";
  byId("failed").hidden = false;
  throw new Error("jalanflow: road data did not load");
}
const all: Edge[] = edgesFile.features;
const drops: LaneDrop[] = dropsFile?.drops ?? [];
const edgeById = new Map(all.map((f) => [f.properties.id, f]));
const MOST_LANES = Math.max(1, ...all.map((f) => f.properties.lanes));

const c = colors();
const routeColour = getComputedStyle(document.documentElement).getPropertyValue("--route").trim();

// ---------- the map ----------

const map = new maplibregl.Map({
  container: "map",
  style: base.style,
  center: [101.58, 3.06],
  zoom: 11,
  attributionControl: { compact: phone() },
});
map.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");
map.on("movestart", (e) => {
  if ("originalEvent" in e && e.originalEvent) userMoved = true;
});

const drive = (d: Direction): Edge[] => all.filter((f) => f.properties.dir === d && f.properties.flow_vph > 0);
const cutOf = (d: Direction): Edge[] => all.filter((f) => f.properties.dir === d && f.properties.is_cut);
const nearlyFull = (p: EdgeProps): boolean =>
  !p.is_cut && p.capacity_vph > 0 && p.flow_vph / p.capacity_vph >= NEARLY_FULL;

function collection(features: Edge[]): FeatureCollection<LineString, EdgeProps> {
  return { type: "FeatureCollection", features };
}

function boundsOf(features: Edge[]): maplibregl.LngLatBounds {
  const b = new maplibregl.LngLatBounds();
  for (const f of features) for (const p of f.geometry.coordinates) b.extend([p[0]!, p[1]!]);
  return b;
}

function fitPadding(): number {
  return phone() ? 36 : 64;
}

// ---------- the camera ----------
// Each view says what the map should frame: the whole drive (start, result,
// how it works), every lane drop (the list) or one feature (the detail).
// Moving to a new view frames it again, so a zoom meant for one view never
// carries into the next. Within a view the reader's own pans and zooms are
// kept: a resize or a card does not snap the map back. Map cards move the
// map only as far as it takes to show them.
// The phone sheet never moves the camera (decision 0018): the camera pads by
// the sheet as it stands when the reader asks to go somewhere (a view, the
// analysis, a direction, a picked place), and dragging, opening or closing
// the sheet afterwards leaves the map where it is.

type Frame = { kind: "drive" } | { kind: "drops" } | { kind: "feature"; b: maplibregl.LngLatBounds };
let frame: Frame = view === "list" ? { kind: "drops" } : { kind: "drive" };

/**
 * Phones: how much of the map's height the floating sheet covers (decision
 * 0017). The map never changes size; the sheet sits over its lower part. 0 on
 * desktop, where the panel is beside the map.
 */
function sheetCover(): number {
  const m = map.getContainer().getBoundingClientRect();
  const panel = byId("panel");
  if (getComputedStyle(panel).position !== "fixed") return 0;
  // A settling sheet counts at the height it is on its way to, so a camera
  // move asked for meanwhile works to where the sheet comes to rest.
  return Math.round(Math.max(0, Math.min(m.height, settleH ?? panel.offsetHeight)));
}

/** Phones: how much of the map the sheet covers right now, mid-drag included. For the credit line. */
function sheetShown(): number {
  const panel = byId("panel");
  return getComputedStyle(panel).position === "fixed" ? panel.offsetHeight : 0;
}

/**
 * Space to keep clear inside the map: the zoom buttons (top right), the
 * credit line (bottom right) and, on phones, the part under the sheet, so
 * what a view frames sits in the map that is still showing.
 */
function padding(): maplibregl.PaddingOptions {
  const box = map.getContainer();
  const base = fitPadding();
  const cover = sheetCover();
  const shown = box.clientHeight - cover;
  const corner = (sel: string): HTMLElement | null => box.querySelector<HTMLElement>(sel);
  const buttons = corner(".maplibregl-ctrl-top-right")?.offsetWidth ?? 0;
  const credit = corner(".maplibregl-ctrl-bottom-right")?.offsetHeight ?? 0;
  // Never more than a third of the map per side, or a short phone map has no room left.
  const cap = (v: number, size: number): number => Math.min(v, Math.floor(size / 3));
  return {
    top: cap(base, shown),
    left: cap(base, box.clientWidth),
    right: cap(Math.max(base, buttons + EDGE_GAP), box.clientWidth),
    bottom: cover + cap(Math.max(base, credit + EDGE_GAP), shown),
  };
}

function frameBounds(f: Frame): maplibregl.LngLatBounds {
  if (f.kind === "feature") return f.b;
  if (f.kind === "drops") {
    const b = new maplibregl.LngLatBounds();
    for (const d of drops) b.extend([d.pos.lon, d.pos.lat]);
    if (!b.isEmpty()) return b;
  }
  return boundsOf(drive(dir));
}

/** Move to the frame. "jump" for resizes and first draw; "ease" between views. Never a move for nothing. */
function frameMap(how: "jump" | "ease"): void {
  if (!mapReady) return;
  const b = frameBounds(frame);
  if (b.isEmpty()) return;
  // maxZoom only when set: MapLibre takes an undefined one as a value and returns NaN.
  const cam = map.cameraForBounds(b, frame.kind === "feature" ? { padding: padding(), maxZoom: FOCUS_ZOOM } : { padding: padding() });
  if (!cam?.center || cam.zoom === undefined) return;
  const here = map.project(map.getCenter());
  const there = map.project(cam.center);
  if (Math.abs(map.getZoom() - cam.zoom) < 0.05 && Math.hypot(here.x - there.x, here.y - there.y) < 2) return;
  const target = { center: cam.center, zoom: cam.zoom, bearing: 0 };
  if (how === "ease" && !still()) map.easeTo({ ...target, duration: CAMERA_MS });
  else map.jumpTo(target);
}

/** A new view: frame what it is about, whatever the reader had zoomed to before. */
function setFrame(f: Frame): void {
  frame = f;
  userMoved = false;
  frameMap("ease");
}

/** True when the whole box is inside the padded map at a zoom close enough to read it. */
function inView(b: maplibregl.LngLatBounds): boolean {
  if (map.getZoom() < KEEP_ZOOM) return false;
  const pad = padding();
  const box = map.getContainer();
  return [b.getSouthWest(), b.getNorthEast()].every((ll) => {
    const p = map.project(ll);
    return p.x >= pad.left! && p.y >= pad.top! && p.x <= box.clientWidth - pad.right! && p.y <= box.clientHeight - pad.bottom!;
  });
}

/** The detail: the feature, unless the reader already has it in view, close up. */
function focus(b: maplibregl.LngLatBounds): void {
  if (!mapReady || b.isEmpty()) return;
  frame = { kind: "feature", b };
  if (inView(b)) {
    userMoved = true; // their camera, kept
    return;
  }
  setFrame(frame);
}

/** Shift the map the least it takes to bring a box (screen px, relative to the map) inside it. */
function nudge(left: number, top: number, right: number, bottom: number, margin: number): void {
  const box = map.getContainer();
  // The sheet (phones) and the credit line above it hide what is under them.
  const credit = box.querySelector<HTMLElement>(".maplibregl-ctrl-bottom-right")?.offsetHeight ?? 0;
  const floor = box.clientHeight - sheetCover() - credit - margin;
  const dx = left < margin ? left - margin : right > box.clientWidth - margin ? Math.min(right - (box.clientWidth - margin), left - margin) : 0;
  const dy = top < margin ? top - margin : bottom > floor ? Math.min(bottom - floor, top - margin) : 0;
  if (Math.abs(dx) < 1 && Math.abs(dy) < 1) return;
  map.panBy([dx, dy], { duration: still() ? 0 : CAMERA_MS });
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

// Malaysian expressway route marker: yellow hexagon, dark number.
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

// Labels that change with the direction; cleared and drawn again on a switch.
let dirPins: maplibregl.Marker[] = [];
function pin(node: Element, at: [number, number], anchor: maplibregl.PositionAnchor, offset: [number, number]): maplibregl.Marker {
  return new maplibregl.Marker({ element: node as HTMLElement, anchor, offset }).setLngLat(at).addTo(map);
}

function drawDirPins(): void {
  for (const m of dirPins) m.remove();
  dirPins = [];
  // One route marker per expressway that carries a real share of the drive.
  const byRef = new Map<string, { km: number; longest: Edge; longestKm: number }>();
  for (const f of drive(dir)) {
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
    if (r.km >= SHIELD_MIN_KM) dirPins.push(pin(shield(ref), midpoint(r.longest.geometry.coordinates), "center", [0, -20]));
  }
  // The bottleneck says what it is in words, not only in red.
  // Its label opens towards the middle of the drive, clear of the map's edge and buttons.
  const middle = boundsOf(drive(dir)).getCenter().lng;
  for (const f of state === "result" ? cutOf(dir) : []) {
    const lanes = `${f.properties.lanes} lane${f.properties.lanes === 1 ? "" : "s"}`;
    const at = midpoint(f.geometry.coordinates);
    const east = at[0] > middle;
    dirPins.push(pin(el("div", `Bottleneck, ${lanes}`, "tag limit-tag"), at, east ? "right" : "left", [east ? -14 : 14, 0]));
  }
}

/** The two ends of the drive, from the first route the search found. */
function drawPlaces(): void {
  const path = replay?.inbound.steps[0]?.edges;
  const first = edgeById.get(path?.[0]?.[0] ?? -1)?.geometry.coordinates[0];
  const last = edgeById.get(path?.[path.length - 1]?.[0] ?? -1)?.geometry.coordinates.at(-1);
  if (first) pin(el("div", ENDS.inbound.from, "place"), [first[0]!, first[1]!], "top-left", [-10, 12]);
  if (last) pin(el("div", ENDS.inbound.to, "place"), [last[0]!, last[1]!], "bottom-right", [10, -10]);
  for (const d of drops) {
    if (d.confirmed) pin(el("div", "Confirmed jam", "tag"), [d.pos.lon, d.pos.lat], "left", [14, 0]);
  }
}

// Road widths come from the site's lane table (src/lanes.ts): one line by
// lane count when zoomed out, lanes × lane width when zoomed in, so the lane
// lines and the cars sit exactly on the road (decision 0012).
const ROAD_W = roadWidth();
const widen = (by: number): maplibregl.ExpressionSpecification => roadWidth(by);
// Guessed lane counts: a dashed line zoomed out; once lanes are drawn the
// surface is solid and the road edge is dashed instead (the "guessed-edge" layer).
const GUESS_DASH: maplibregl.ExpressionSpecification = [
  "step",
  ["zoom"],
  ["case", ["get", "lanes_inferred"], ["literal", [1.6, 1.2]], ["literal", [1, 0]]],
  LANES_FULL_ZOOM,
  ["literal", [1, 0]],
];
const ROUND = { "line-cap": "round", "line-join": "round" } as const;

function addLayers(): void {
  map.addSource("quiet", { type: "geojson", data: collection([]) });
  map.addSource("drive", {
    type: "geojson",
    data: collection([]),
    // If the basemap failed to load, our own roads carry the OSM credit (AGENTS.md).
    ...(base.tiles ? {} : { attribution: OSM_CREDIT }),
  });

  map.addSource("found", { type: "geojson", data: collection([]) });
  map.addSource("trace", { type: "geojson", data: collection([]) });

  map.addLayer({ id: "quiet", type: "line", source: "quiet", layout: ROUND,
    paint: { "line-color": c.road, "line-width": ["interpolate", ["linear"], ["zoom"], 10, 1, 15, 3] } });
  // The road picked in the panel: a dark outline under everything else on it.
  map.addLayer({ id: "picked", type: "line", source: "drive", layout: ROUND, filter: ["==", ["get", "id"], -1],
    paint: { "line-color": c.ink, "line-width": widen(9) } });
  map.addLayer({ id: "picked-gap", type: "line", source: "drive", layout: ROUND, filter: ["==", ["get", "id"], -1],
    paint: { "line-color": c.bg, "line-width": widen(5) } });
  // Zoomed in, a guessed lane count shows as a dashed road edge.
  map.addLayer({ id: "guessed-edge", type: "line", source: "drive", minzoom: LANES_FULL_ZOOM,
    filter: ["get", "lanes_inferred"], layout: { "line-cap": "butt", "line-join": "round" },
    paint: { "line-color": c.ink, "line-width": widen(2.5), "line-dasharray": [0.6, 0.4] } });
  // Before the analysis: every road between the two places, all alike.
  map.addLayer({ id: "plain", type: "line", source: "drive", layout: { "line-join": "round" },
    paint: {
      "line-color": routeColour,
      "line-width": ROAD_W,
      "line-dasharray": GUESS_DASH,
    } });
  // While it runs: roads that carry something so far, and the route being added.
  map.addLayer({ id: "found", type: "line", source: "found", layout: ROUND,
    paint: { "line-color": routeColour, "line-width": ROAD_W } });
  map.addLayer({ id: "trace", type: "line", source: "trace", layout: ROUND,
    paint: { "line-color": c.ink, "line-width": widen(1.5) } });
  // Roads the analysis uses. A guessed lane count is dashed.
  map.addLayer({ id: "route", type: "line", source: "drive", layout: { "line-join": "round" },
    filter: ["all", ["!", ["get", "is_cut"]], ["<", ["get", "utilisation"], NEARLY_FULL]],
    paint: {
      "line-color": routeColour,
      "line-width": ROAD_W,
      "line-dasharray": GUESS_DASH,
    } });
  // Nearly full: amber with a dark edge, so it does not rest on a pale colour alone.
  const full: maplibregl.FilterSpecification = ["all", ["!", ["get", "is_cut"]], [">=", ["get", "utilisation"], NEARLY_FULL]];
  map.addLayer({ id: "full-edge", type: "line", source: "drive", layout: ROUND, filter: full,
    paint: { "line-color": c.ink, "line-width": widen(2.5) } });
  map.addLayer({ id: "full", type: "line", source: "drive", layout: ROUND, filter: full,
    paint: { "line-color": c.full, "line-width": ROAD_W } });
  // The bottleneck: widest, red, and labelled in words (drawDirPins).
  map.addLayer({ id: "cut-edge", type: "line", source: "drive", layout: ROUND, filter: ["get", "is_cut"],
    paint: { "line-color": c.bg, "line-width": widen(9) } });
  map.addLayer({ id: "cut", type: "line", source: "drive", layout: ROUND, filter: ["get", "is_cut"],
    paint: { "line-color": c.cut, "line-width": widen(5) } });
  // Lane lines, once zoomed in: equal stripes from the lane count, not surveyed positions.
  for (let k = 1; k < MOST_LANES; k++) {
    map.addLayer({ id: `lane-lines-${k}`, type: "line", source: "drive", minzoom: LANES_FULL_ZOOM,
      filter: [">", ["get", "lanes"], k],
      paint: {
        "line-color": c.bg,
        "line-width": ["interpolate", ["linear"], ["zoom"], LANES_FULL_ZOOM, 0.5, 16, 1, 18, 1.5],
        "line-opacity": ["interpolate", ["linear"], ["zoom"], LANES_FULL_ZOOM, 0, LANES_FULL_ZOOM + 0.5, 0.8],
        "line-dasharray": [6, 6],
        "line-offset": laneLineOffset(k),
      } });
  }
  // Invisible, wide: what a tap on a road lands on.
  map.addLayer({ id: "drive-hit", type: "line", source: "drive", layout: ROUND,
    paint: { "line-color": c.ink, "line-opacity": 0, "line-width": 18 } });

  const points: FeatureCollection<Point> = {
    type: "FeatureCollection",
    features: drops.map((d, i) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [d.pos.lon, d.pos.lat] },
      properties: { i, rank: d.rank, confirmed: d.confirmed !== null, guessed: d.lanes_inferred },
    })),
  };
  map.addSource("drops", { type: "geojson", data: points });
  const key: maplibregl.ExpressionSpecification = ["any", ["get", "confirmed"], ["<=", ["get", "rank"], KEY_DROPS]];
  for (const tier of ["drops-rest", "drops-key"] as const) {
    map.addLayer({
      id: tier, type: "circle", source: "drops",
      filter: tier === "drops-key" ? key : ["!", key],
      minzoom: tier === "drops-key" ? 0 : REST_DROPS_ZOOM,
      paint: {
        // Red = a jam a driver confirmed, solid = lane drop, hollow = lane count guessed.
        "circle-radius": ["interpolate", ["linear"], ["zoom"], 10, 3.2, 15, 5.5],
        "circle-color": ["case", ["get", "confirmed"], c.cut, ["get", "guessed"], c.bg, c.ink],
        "circle-stroke-color": ["case", ["get", "confirmed"], c.bg, ["get", "guessed"], c.ink, c.bg],
        "circle-stroke-width": 1.5,
      },
    });
  }
  // A jam a driver confirmed: a red dot in a red ring. It is red like the bottleneck
  // because both are where it jams; the ring and its label say this one was seen,
  // the thick line and its label say that one was worked out (decision 0014).
  map.addLayer({ id: "drops-seen", type: "circle", source: "drops", filter: ["get", "confirmed"],
    paint: { "circle-radius": 9, "circle-color": c.bg, "circle-opacity": 0, "circle-stroke-color": c.cut, "circle-stroke-width": 2.2 } });
  // The marker picked in the list or on the map.
  map.addLayer({ id: "drop-picked", type: "circle", source: "drops", filter: ["==", ["get", "i"], -1],
    paint: { "circle-radius": 12, "circle-color": c.ink, "circle-opacity": 0.14, "circle-stroke-color": c.ink, "circle-stroke-width": 2.4 } });
}

const RESULT_LAYERS = ["route", "full-edge", "full", "cut-edge", "cut"];
const RUN_LAYERS = ["found", "trace"];

/** The map for the state: lanes only, then the search, then what it found. */
function styleMap(): void {
  if (!mapReady) return;
  const vis = (on: boolean): "visible" | "none" => (on ? "visible" : "none");
  for (const id of RESULT_LAYERS) map.setLayoutProperty(id, "visibility", vis(state === "result"));
  for (const id of RUN_LAYERS) map.setLayoutProperty(id, "visibility", vis(state === "running"));
  map.setLayoutProperty("plain", "visibility", vis(state !== "result"));
  map.setPaintProperty("plain", "line-opacity", state === "running" ? 0.3 : 1);
  drawDirPins();
  syncFlow();
}

/** Dots only on the result, only if wanted; still with reduced motion. */
function syncFlow(): void {
  if (!flow) return;
  if (state === "result" && flowOn) flow.show(dir, !still());
  else flow.hide();
}

function setTrace(source: "found" | "trace", ids: Iterable<number>): void {
  if (!mapReady) return;
  const features = [...ids].map((id) => edgeById.get(id)).filter((f): f is Edge => f !== undefined);
  (map.getSource(source) as maplibregl.GeoJSONSource).setData(collection(features));
}

function showAllDrops(on: boolean): void {
  if (!mapReady) return;
  map.setLayerZoomRange("drops-rest", on ? 0 : REST_DROPS_ZOOM, 24);
}

function pickRoad(id: number | null): void {
  if (!mapReady) return;
  for (const layer of ["picked", "picked-gap"]) map.setFilter(layer, ["==", ["get", "id"], id ?? -1]);
}

function pickDrop(d: LaneDrop | null): void {
  if (!mapReady) return;
  map.setFilter("drop-picked", ["==", ["get", "i"], d ? drops.indexOf(d) : -1]);
}

// ---------- the panel ----------

function setView(next: View, from?: HTMLElement | null): void {
  if (state === "running" && next !== "summary") return; // the search has the panel
  closeCard(true);
  if (from) opener = from;
  const changed = next !== view;
  view = next;
  document.body.dataset.view = next;
  mini = false; // a view the reader asked for is shown
  syncSheet();
  showAllDrops(next === "list");
  if (next !== "detail") {
    shownDrop = null;
    pickRoad(null);
    pickDrop(null);
  }
  syncLink();
  if (next === "summary") {
    // Back where the reader was, or at the top of the findings.
    (opener?.isConnected ? opener : byId("panel")).focus();
    opener = null;
  } else {
    document.querySelector<HTMLElement>(`[data-v="${next}"] [data-back]`)?.focus();
  }
  byId("panel").scrollTop = 0;
  // The detail frames its own feature (focus); the other views frame here.
  if (changed && next !== "detail") setFrame(next === "list" ? { kind: "drops" } : { kind: "drive" });
}

function facts(rows: [string, string][]): void {
  byId("detail-facts").replaceChildren(...rows.flatMap(([k, v]) => [el("dt", k), el("dd", v)]));
}

function mapLinks(lat: number, lon: number, more: HTMLAnchorElement[] = []): void {
  const ll = `${lat.toFixed(5)},${lon.toFixed(5)}`;
  byId("detail-links").replaceChildren(
    link(`https://www.google.com/maps/@?api=1&map_action=pano&viewpoint=${ll}`, "Look at it in Street View"),
    link(`https://www.google.com/maps/search/?api=1&query=${ll}`, "Google Maps"),
    ...more,
  );
}

function openRoad(f: Edge, from: HTMLElement | null): void {
  const p = f.properties;
  byId("detail-title").textContent = roadTitle(p);
  if (state === "running") return;
  const solved = state === "result";
  const guessed = p.lanes_inferred ? " (guessed from the road type)" : "";
  // What the road carries, and whether it is the bottleneck, are results:
  // they are held back until the analysis has run.
  facts([
    ["Lanes", `${p.lanes}${guessed}`],
    ["Can carry", `about ${fmtVph(p.capacity_vph)} veh/h`],
    ...(solved ? [["The analysis sends", `${fmtVph(p.flow_vph)} veh/h`] as [string, string]] : []),
  ]);
  const { from: a, to: b } = ENDS[dir];
  byId("detail-note").textContent = !solved
    ? "Run the analysis to see how much of this it would use."
    : p.is_cut
    ? `This is the bottleneck. Every route from ${a} to ${b} has to cross a stretch like this one, and the analysis fills it completely, so nothing more gets through.`
    : nearlyFull(p)
      ? "Nearly full in the analysis. It is not what sets the limit: the bottleneck is. Nothing here is measured traffic."
      : "It has room to spare in the analysis.";
  const [lon, lat] = midpoint(f.geometry.coordinates);
  mapLinks(lat, lon);
  shownDrop = null; // roads have no lasting id, so no link of their own
  setView("detail", from);
  pickDrop(null);
  pickRoad(p.id);
  focus(boundsOf([f]));
}

function roadsText(roads: LaneDrop["from"]): string {
  if (roads.length === 0) return "Not known";
  return roads.map((r) => `${roadLabel(r)} (${r.lanes}${r.lanes_inferred ? ", guessed" : ""})`).join(" + ");
}

function openDrop(d: LaneDrop, from: HTMLElement | null): void {
  if (state === "running") return;
  byId("detail-title").textContent = dropTitle(d);
  facts([
    ["Lanes", lanesText(d)],
    ["Coming in", roadsText(d.from)],
    ["Carrying on", roadsText(d.to)],
  ]);
  const notes = [d.kind === "merge" ? "Roads join here and lanes are lost." : "The road loses lanes here."];
  if (d.confirmed) notes.push(`A local driver confirmed this as a daily jam (${d.confirmed.source}).`);
  else notes.push("This counts lanes, not cars: it is not a report of a jam.");
  if (d.lanes_inferred) notes.push("Some lane counts here are guessed from the road type. Check on Street View.");
  byId("detail-note").textContent = notes.join(" ");
  const [osmType, osmId] = d.key.startsWith("node:") ? ["node", d.key.slice(5)] : ["way", String(d.to[0]?.osm_way_id ?? "")];
  mapLinks(d.pos.lat, d.pos.lon, [link(`https://www.openstreetmap.org/${osmType}/${osmId}`, "OpenStreetMap")]);
  shownDrop = d;
  setView("detail", from);
  pickRoad(null);
  pickDrop(d);
  focus(new maplibregl.LngLatBounds([d.pos.lon, d.pos.lat], [d.pos.lon, d.pos.lat]));
}

function renderList(): void {
  byId("open-list").textContent = `See all ${drops.length} lane drops`;
  byId("open-list").hidden = drops.length === 0;
  const confirmed = drops.filter((d) => d.confirmed).length;
  // The landing counts: one line per mark, the number in bold (DESIGN_BRIEF.md §1).
  const row = (mark: string, ...parts: (string | Node)[]): HTMLElement => {
    const li = el("li");
    const i = el("i", undefined, mark);
    i.setAttribute("aria-hidden", "true");
    li.append(i, el("span"));
    li.lastElementChild!.append(...parts);
    return li;
  };
  if (drops.length === 0) byId("counts").replaceChildren(el("li", "The lane-drop data did not load."));
  else {
    const lines = [row("dot", el("b", `${drops.length} places`), " where more lanes come in than carry on.")];
    const said = confirmedSentence(confirmed);
    const word = countWord(confirmed);
    if (said) {
      const at = said.indexOf(` ${word} `);
      lines.push(at < 0 ? row("dot ring", said) : row("dot ring", said.slice(0, at + 1), el("b", word), said.slice(at + 1 + word.length)));
    }
    byId("counts").replaceChildren(...lines);
  }
  byId("list").replaceChildren(
    ...drops.map((d) => {
      const li = el("li");
      const btn = el("button");
      btn.type = "button";
      btn.append(el("b", `${d.lanes_in} → ${d.lanes_out}`), el("span", dropTitle(d)));
      if (d.confirmed) btn.append(el("em", "Confirmed jam"));
      else if (d.lanes_inferred) btn.append(el("em", "Lanes guessed"));
      btn.addEventListener("click", () => openDrop(d, btn));
      // The list and the map point at each other.
      for (const on of ["mouseenter", "focus"]) btn.addEventListener(on, () => pickDrop(d));
      for (const off of ["mouseleave", "blur"]) btn.addEventListener(off, () => view === "list" && pickDrop(null));
      rowOf.set(d, btn);
      li.append(btn);
      return li;
    }),
  );
}

function renderHow(): void {
  const how = byId("how");
  const notes = [...(dropsFile?.assumptions ?? []), ...manifest!.assumptions];
  how.replaceChildren(...notes.map((a) => el("li", a)));
  if (dropsFile) how.append(el("li", dropsFile.ranking));
  const src = el("li", `Road data: OpenStreetMap, as of ${manifest!.osm_date}. `);
  src.append(link("https://www.openstreetmap.org/copyright", "© OpenStreetMap contributors"));
  how.append(src);
}

/** The map's roads and labels for the direction. */
function renderMap(): void {
  if (!mapReady) return;
  const mine = drive(dir);
  const used = new Set(mine);
  (map.getSource("quiet") as maplibregl.GeoJSONSource).setData(collection(all.filter((f) => !used.has(f))));
  (map.getSource("drive") as maplibregl.GeoJSONSource).setData(collection(mine));
  drawDirPins();
  if (!userMoved) frameMap("ease");
  syncFlow();
}

/**
 * The lane strip on the result (DESIGN_BRIEF.md §13): one bar per stretch of
 * the part every route uses, as tall as its lanes and as long as its km.
 * Hidden when the traffic splits at once and no road is shared.
 */
function renderProfile(): void {
  const shared = sharedRoads(
    drive(dir).map((f) => ({ ...f.properties, coords: f.geometry.coordinates })),
    manifest!.max_flow_vph[dir],
  );
  const parts = stretches(shared);
  byId("profile").hidden = parts.length === 0;
  if (parts.length === 0) return;
  const total = parts.reduce((sum, p) => sum + p.km, 0);
  const strip = byId("profile-strip");
  strip.setAttribute("aria-label", profileLabel(parts));
  strip.style.setProperty("--lanes", String(Math.max(...parts.map((p) => p.lanes))));
  let at = 0;
  strip.replaceChildren(
    ...parts.map((p) => {
      const mark = p.cut ? " cut" : p.full ? " full" : "";
      const part = el("div", undefined, `part${mark}${p.guessed ? " guessed" : ""}`);
      part.style.flexGrow = String(p.km);
      const bar = el("i");
      bar.style.setProperty("--n", String(p.lanes));
      const bed = el("div", undefined, "bed");
      bed.append(bar);
      part.append(bed, el("span", `${p.lanes}${p.guessed ? "?" : ""}`));
      // Named in words as well as colour; the label hangs towards the middle.
      if (p.cut) part.append(el("em", "Bottleneck", (at + p.km / 2) / total > 0.6 ? "end" : ""));
      at += p.km;
      return part;
    }),
  );
  byId("profile-title").textContent = profileTitle(parts, sharedName(shared));
  byId("profile-say").textContent = profileSentence(parts);
}

/** Everything that depends on the direction: the panel's words, then the map's roads. */
function renderDirection(announce: boolean): void {
  const { from, to } = ENDS[dir];
  const max = manifest!.max_flow_vph[dir];
  const cut = cutOf(dir);
  const cutProps = cut.map((f) => f.properties);

  document.querySelectorAll<HTMLButtonElement>(".seg button").forEach((b) => {
    b.setAttribute("aria-pressed", String(b.dataset.dir === dir));
  });
  for (const id of ["sum-dir", "run-dir", "running-dir"]) byId(id).textContent = `from ${from} to ${to}`;
  byId("cap").textContent = fmtVph(max);

  const limit = bottleneckSentence(cutProps);
  byId("limit-text").textContent = limit || "The analysis found no single limiting stretch.";
  byId<HTMLButtonElement>("limit-btn").disabled = cut.length === 0;
  document.querySelector<HTMLElement>("#limit-btn .hint")!.hidden = cut.length === 0;

  // The tested lane: a what-if, kept apart from the road as it is today.
  const tests = fixes?.[dir] ?? [];
  const best = tests.filter((f) => f.helps).sort((a, b) => b.gain_vph - a.gain_vph)[0];
  byId("whatif").hidden = tests.length === 0;
  // Two bars from the same zero on one scale (copy.ts whatIfBars). No gain
  // shows equal bars; no baseline shows none.
  const then = (best ?? tests[0])?.new_max_flow_vph ?? NaN;
  const bars = whatIfBars(max, then);
  byId("bars").hidden = !bars;
  if (bars) {
    const pct = (share: number): string => `${(share * 100).toFixed(2)}%`;
    byId("cmp-now").textContent = fmtVph(max);
    byId("cmp-then").textContent = fmtVph(then);
    byId("cmp-gain").textContent = then > max ? `+${fmtVph(then - max)}` : "";
    byId("bar-now").style.width = pct(bars.now);
    byId("bar-then").style.width = pct(bars.then - bars.gain);
    byId("bar-gain").style.width = pct(bars.gain);
  }
  const why = tests[0]?.why_no_gain;
  byId("whatif-say").textContent =
    !best && why ? `One more lane there would not raise the limit: ${why}.` : fixSentence(tests, cutProps).join("");
  const hard = fixDifficulty(tests);
  byId("whatif-hard").textContent = hard;
  byId("whatif-hard").hidden = !hard;

  renderProfile();
  renderMap();
  syncSheet();
  if (!announce) return;
  say(
    state === "result"
      ? `From ${from} to ${to}: at most ${fmtVph(max)} vehicles an hour. ${limit}`
      : `From ${from} to ${to}. Run the analysis to see what the roads can carry.`,
  );
}

// ---------- the quick look: a card beside a marker or road ----------
// A preview, not a second panel: a name, the lanes, one or two facts and a
// way into the full detail. The panel stays the place for everything else.

type Picked = { drop: LaneDrop; road?: undefined } | { road: Edge; drop?: undefined };

/** red: where it jams (a confirmed jam or the bottleneck); the glyph's shape tells which. */
function cardKind(swatch: string, text: string, red = false): HTMLElement {
  const p = el("p", undefined, red ? "card-kind red" : "card-kind");
  const i = el("i", undefined, swatch);
  i.setAttribute("aria-hidden", "true");
  // Shown in capitals by the stylesheet (decision 0015); the words themselves stay
  // in sentence case, which is what a screen reader reads.
  p.append(i, el("b", text));
  return p;
}

/** Room for a card on the map that is showing. A small map shows the preview in the sheet instead. */
function roomForCard(): boolean {
  const box = map.getContainer();
  return box.clientWidth >= CARD_MIN_MAP_W && box.clientHeight - sheetCover() >= CARD_MIN_MAP_H;
}

/** The card's name, after the road's route shields (only exact E-numbers). */
function cardName(title: string, refs: string[]): HTMLElement {
  const p = el("p", undefined, "card-title");
  for (const ref of refs) p.append(shield(ref));
  p.append(el("span", title));
  return p;
}

/** A guessed lane count, said next to the lanes it is about, with the key's dashed line. */
function cardGuessed(text: string): HTMLElement {
  const p = el("p", undefined, "card-flag");
  const i = el("i", undefined, "sw dash");
  i.setAttribute("aria-hidden", "true");
  p.append(i, el("span", text));
  return p;
}

// The lane diagram: the drop's own lane counts and, at a merge, the side a
// road joins on (lane_drops.json). Decorative for screen readers: the lanes
// are said in words right under it.
function cardDiagram(d: LaneDrop): SVGSVGElement {
  const NS = "http://www.w3.org/2000/svg";
  const g = laneDiagram(d);
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("class", g.guessed ? "lane-diagram guessed" : "lane-diagram");
  svg.setAttribute("viewBox", `0 0 ${g.width} ${g.height}`);
  svg.setAttribute("width", String(g.width));
  svg.setAttribute("height", String(g.height));
  svg.setAttribute("aria-hidden", "true");
  const reveal = document.createElementNS(NS, "g");
  reveal.setAttribute("class", "reveal");
  const road = document.createElementNS(NS, "path");
  road.setAttribute("class", "road");
  road.setAttribute("d", g.road);
  reveal.append(road);
  for (const l of g.lines) {
    const line = document.createElementNS(NS, "line");
    line.setAttribute("class", l.kind);
    for (const [k, v] of [["x1", l.x1], ["y1", l.y1], ["x2", l.x2], ["y2", l.y2]] as const) line.setAttribute(k, String(v));
    reveal.append(line);
  }
  const label = (text: string, x: number, cls: string) => {
    const t = document.createElementNS(NS, "text");
    t.setAttribute("class", cls);
    t.setAttribute("x", String(x));
    t.setAttribute("y", "12");
    t.textContent = text;
    return t;
  };
  svg.append(reveal, label(g.inLabel, 6, "in"), label(g.outLabel, g.width - 6, "out"));
  return svg;
}

function cardBody(what: Picked): HTMLElement {
  const box = el("div", undefined, "card");
  box.setAttribute("role", "group");
  let title: string;
  let source: string;
  if (what.drop) {
    const d = what.drop;
    title = dropTitle(d);
    // The map's own mark: ring = a jam a driver saw, hollow = lanes guessed, solid = lane drop.
    box.append(
      d.confirmed
        ? cardKind("dot ring", "Confirmed jam", true)
        : cardKind(d.lanes_inferred ? "dot hollow" : "dot", "Lane drop"),
    );
    const g = laneDiagram(d);
    box.append(cardName(title, dropShields(d)), cardDiagram(d), el("p", lanesText(d), "card-main"));
    if (d.lanes_inferred) box.append(cardGuessed("Some lane counts here are guessed."));
    source = `${d.confirmed ? "Seen by a local driver." : "From OpenStreetMap lane counts."} ${dropSentence(d, g.joinSide)}`;
  } else {
    const p = what.road.properties;
    title = roadTitle(p);
    const refs = expresswayRefs(p.ref);
    // Whether it is the bottleneck, or nearly full, is a result: only after the analysis.
    if (state !== "result") box.append(cardKind("sw", "Road"));
    else if (p.is_cut) box.append(cardKind("sw cut", "Bottleneck", true));
    else if (nearlyFull(p)) box.append(cardKind("sw full", "Nearly full"));
    else box.append(cardKind("sw", "Room to spare"));
    // With a shield the ref is said by the sign, so the name follows it alone.
    box.append(cardName(refs.length && p.name ? p.name : title, refs));
    box.append(el("p", `${p.lanes} lane${p.lanes === 1 ? "" : "s"}`, "card-main"));
    if (p.lanes_inferred) box.append(cardGuessed("Lane count guessed from the road type."));
    const cap = el("p", "Can carry about ", "card-cap");
    cap.append(el("b", fmtVph(p.capacity_vph)), " ", el("span", "veh/h", "unit"));
    box.append(cap);
    source =
      state === "result"
        ? "Worked out by the analysis, not measured."
        : "From lane counts. Run the analysis to see whether it limits the drive.";
  }
  box.setAttribute("aria-label", title);
  box.append(el("p", source, "card-note"));

  const acts = el("p", undefined, "card-acts");
  const details = el("button", "View details →", "link");
  details.type = "button";
  details.addEventListener("click", () => {
    closeCard(true);
    if (what.drop) openDrop(what.drop, null);
    else openRoad(what.road, null);
  });
  acts.append(details);
  if (!roomForCard()) {
    const close = el("button", "Close", "link");
    close.type = "button";
    close.addEventListener("click", () => closeCard());
    acts.append(close);
  }
  box.append(acts);
  return box;
}

/**
 * One at a time: opening a card closes the one before it. A hovered card
 * (pinned false) goes away when the pointer leaves; a clicked one stays.
 */
function openCard(what: Picked, at: [number, number], pinned = true): void {
  const key = what.drop ?? what.road;
  window.clearTimeout(cardTimer);
  if (card?.what === key) {
    if (pinned && !card.pinned) {
      card.pinned = true; // a click on a hovered card keeps it
      revealCard();
    }
    return;
  }
  closeCard(true);
  const node = cardBody(what);
  // The lane diagram draws itself once, in the direction of travel, when a
  // card is opened on purpose. A hover preview shows it finished; reduced
  // motion turns the animation off in the stylesheet (DESIGN_BRIEF.md §6).
  if (pinned) node.querySelector(".lane-diagram")?.classList.add("draw");
  pickDrop(what.drop ?? null);
  pickRoad(what.road?.properties.id ?? null);
  if (!roomForCard()) {
    // A map too small to hold a card: the preview takes the sheet's place.
    byId("peek").replaceChildren(node);
    byId("peek").hidden = false;
    document.body.dataset.peek = "on";
    card = { popup: null, node, what: key, pinned: true, at };
    mini = false; // the quick look the reader asked for is shown
    syncSheet();
    // The sheet may have changed height: keep the picked place on the map.
    requestAnimationFrame(() => {
      map.resize();
      revealCard();
    });
  } else {
    // No fixed anchor: MapLibre puts the card on whichever side has room, and
    // the offset keeps it off the marker. It stays pinned to the place as the map moves.
    const popup = new maplibregl.Popup({
      closeButton: false,
      closeOnClick: false,
      focusAfterOpen: false,
      maxWidth: "272px",
      offset: 16,
      className: "map-card",
    })
      .setLngLat(at)
      .setDOMContent(node)
      .addTo(map);
    card = { popup, node, what: key, pinned, at };
    if (pinned) requestAnimationFrame(() => revealCard());
    // The pointer may travel from the marker onto the card without losing it.
    node.addEventListener("mouseenter", () => window.clearTimeout(cardTimer));
    node.addEventListener("mouseleave", () => leaveCard());
  }
  if (pinned) say(`${node.getAttribute("aria-label") ?? ""}. ${node.querySelector(".card-main")?.textContent ?? ""}.`);
}

/**
 * A card keeps the reader's camera. The map moves only when the card or its
 * place would be cut off, and then only by the overflow. Hover previews never move it.
 */
function revealCard(): void {
  if (!card?.pinned || !mapReady) return;
  if (card.popup) {
    const node = card.popup.getElement();
    if (!node) return;
    const r = node.getBoundingClientRect();
    const c = map.getContainer().getBoundingClientRect();
    nudge(r.left - c.left, r.top - c.top, r.right - c.left, r.bottom - c.top, EDGE_GAP);
  } else {
    const p = map.project(card.at);
    nudge(p.x, p.y, p.x, p.y, 40);
  }
}

/** The pointer left a hovered card or its marker: close it after a moment, unless it was clicked. */
function leaveCard(): void {
  window.clearTimeout(cardTimer);
  if (!card || card.pinned) return;
  cardTimer = window.setTimeout(() => {
    if (card && !card.pinned) closeCard();
  }, CARD_LINGER_MS);
}

/** keepPick: the panel is about to show this same place, so its highlight stays. */
function closeCard(keepPick = false): void {
  window.clearTimeout(cardTimer);
  if (!card) return;
  const { popup } = card;
  card = null;
  if (popup) {
    popup.getElement()?.classList.add("leaving");
    window.setTimeout(() => popup.remove(), still() ? 0 : 120);
  } else {
    delete document.body.dataset.peek;
    byId("peek").hidden = true;
    byId("peek").replaceChildren();
    syncSheet();
  }
  if (!keepPick && view !== "detail") {
    pickDrop(null);
    pickRoad(null);
  }
}

// A click anywhere off the card and off the map closes it (the map handles its own clicks).
document.addEventListener("pointerdown", (e) => {
  if (!card || !(e.target instanceof Node)) return;
  // The handle only changes the sheet's height: the quick look stays.
  if (card.node.contains(e.target) || byId("map").contains(e.target) || byId("grab").contains(e.target)) return;
  closeCard();
});

// ---------- run the analysis ----------

const sleep = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));

function setState(next: State): void {
  closeCard(); // what a card says depends on whether the analysis has run
  if (next !== "running") runId++; // stops a search in progress
  state = next;
  document.body.dataset.state = next;
  setTrace("found", []);
  setTrace("trace", []);
  styleMap();
  syncLink();
  syncSheet();
  // The reader asked for this state, and its sheet may stand taller or
  // shorter: frame for it now, unless they have moved the map themselves.
  if (!userMoved) frameMap("ease");
}

/** The result is on the page: the search finished, was skipped, or motion is reduced. */
function finish(): void {
  setState("result");
  const max = manifest!.max_flow_vph[dir];
  byId("panel").focus();
  byId("panel").scrollTop = 0;
  say(`At most ${fmtVph(max)} vehicles an hour can get from ${ENDS[dir].from} to ${ENDS[dir].to}. ${bottleneckSentence(cutOf(dir).map((f) => f.properties))}`);
}

/**
 * Replays the real search from replay.json, route by route: each route and
 * each amount is what the pipeline found, in the order it found them. With
 * reduced motion, or with no replay file, it goes straight to the result.
 */
async function run(): Promise<void> {
  const steps = replay?.[dir].steps ?? [];
  if (view === "summary") setFrame({ kind: "drive" }); // the search and its result cover the whole drive
  if (still() || steps.length === 0) return finish();
  setState("running");
  const id = ++runId;
  const n = steps.length;
  const segs = steps.map(() => el("i"));
  byId("run-segs").replaceChildren(...segs);
  const bar = byId("run-progress");
  bar.setAttribute("aria-valuemin", "0");
  bar.setAttribute("aria-valuemax", String(n));
  const progress = (i: number): void => {
    // i routes are done; the next one, if any, is being traced.
    segs.forEach((s, k) => s.className = k < i ? "done" : k === i ? "now" : "");
    byId("run-label").textContent = routeProgress(i, n);
    bar.setAttribute("aria-valuenow", String(i));
    bar.setAttribute("aria-valuetext", routeProgress(i, n));
  };
  byId("run-count").textContent = "0";
  progress(0);
  byId("run-caption").textContent = "";
  byId("skip-btn").focus();
  say("The analysis is running.");

  const stepMs = Math.min(RUN_STEP_MAX_MS, RUN_TOTAL_MS / n);
  const carried = new Map<number, number>(); // veh/h on each road so far
  let total = 0;
  for (const [i, step] of steps.entries()) {
    progress(i);
    setTrace("trace", step.edges.map(([edge]) => edge));
    // A negative delta: this route takes back flow an earlier one sent there.
    const movesEarlier = step.edges.some(([, delta]) => delta < 0);
    byId("run-caption").textContent = routeCaption(i + 1, step.amount_vph, movesEarlier);
    await sleep(stepMs);
    if (id !== runId) return; // skipped, restarted or the direction changed
    for (const [edge, delta] of step.edges) carried.set(edge, (carried.get(edge) ?? 0) + delta);
    total += step.amount_vph;
    setTrace("found", [...carried].filter(([, v]) => v > 0).map(([edge]) => edge));
    // Exact while it runs, so it agrees with the caption; the result is rounded.
    byId("run-count").textContent = fmtExact(total);
  }
  setTrace("trace", []);
  progress(n);
  byId("run-caption").textContent = ROUTES_DONE;
  await sleep(stepMs / 2);
  if (id === runId) finish();
}

// ---------- wiring ----------

document.querySelectorAll<HTMLButtonElement>(".seg button").forEach((b) => {
  b.addEventListener("click", () => {
    const next: Direction = b.dataset.dir === "outbound" ? "outbound" : "inbound";
    if (next === dir) return;
    dir = next;
    closeCard();
    userMoved = false; // a new drive: show all of it
    if (state === "running") setState("home"); // the search under way was for the other direction
    if (view === "detail") setView("summary"); // the picked road belongs to the other direction
    renderDirection(true);
    syncLink();
  });
});

byId("limit-btn").addEventListener("click", () => {
  const f = cutOf(dir)[0];
  if (f) openRoad(f, byId("limit-btn"));
});
document.querySelectorAll<HTMLElement>("[data-open]").forEach((b) => {
  b.addEventListener("click", () => setView(b.dataset.open === "how" ? "how" : "list", b));
});
byId("about-how").addEventListener("click", () => setView("how", byId("about-how")));
document.querySelectorAll<HTMLElement>("[data-back]").forEach((b) => {
  b.addEventListener("click", () => setView("summary"));
});
window.addEventListener("keydown", (e) => {
  if (e.key !== "Escape") return;
  if (card) closeCard(); // the quick look first, then the view under it
  else if (view !== "summary") setView("summary");
});
window.addEventListener("hashchange", () => {
  const p = parseHash(window.location.hash);
  if (p.dir !== dir) {
    // As the direction switch does: the search under way was for the other direction.
    const stop = stopsRun(state === "running", dir, p);
    dir = p.dir;
    closeCard();
    userMoved = false;
    if (stop) setState("home");
    renderDirection(true);
  }
  const linked = p.drop ? drops.find((d) => dropSlug(d.key) === p.drop) : undefined;
  if (linked) {
    if (linked !== shownDrop) openDrop(linked, null);
    return;
  }
  if (p.view !== view) setView(p.view);
  if (p.view === "summary" && p.solved !== (state === "result")) setState(p.solved ? "result" : "home");
});

// ---------- the phone sheet: three heights (decisions 0017, 0018) ----------
// Collapsed: the handle, one line on what the page shows, and that it is not
// live traffic. Summary: the top of the findings (the default). Expanded: all
// of the view, scrolling inside the sheet. Every change has a button: the
// handle hides the findings or brings them back, More detail / Less detail
// goes between summary and expanded. A drag of the handle does the same by
// height. None of it moves the camera.
const more = byId<HTMLButtonElement>("more-btn");

function sheetNow(): Sheet {
  if (mini) return "collapsed";
  const body = document.body.dataset;
  return view === "summary" && body.more !== "open" && body.peek !== "on" ? "summary" : "expanded";
}

/** The heights this view has: the list, a detail and a quick look have no shorter summary. */
function sheetStops(): Sheet[] {
  if (view !== "summary" || document.body.dataset.peek === "on") return ["collapsed", "expanded"];
  return state === "running" ? ["collapsed", "summary"] : ["collapsed", "summary", "expanded"];
}

/** What the collapsed sheet says the page is showing. */
function miniTitle(): string {
  if (card && !card.popup) return card.node.getAttribute("aria-label") ?? "";
  if (view === "list") return byId("list-h").textContent ?? "";
  if (view === "detail") return byId("detail-title").textContent ?? "";
  if (view === "how") return byId("how-h").textContent ?? "";
  if (state === "running") return "The analysis is running.";
  if (state === "result") return resultLine(manifest!.max_flow_vph[dir], ENDS[dir].from, ENDS[dir].to);
  return "Where the road runs out of room.";
}

/** The page's sheet attribute, the handle's words and the collapsed line, from the state. */
function syncSheet(): void {
  document.body.dataset.sheet = sheetNow();
  const grab = byId("grab");
  grab.setAttribute("aria-expanded", String(!mini));
  grab.setAttribute("aria-label", mini ? "Show the findings" : "Show more of the map");
  byId("mini-title").textContent = miniTitle();
}

function setMore(open: boolean): void {
  document.body.dataset.more = open ? "open" : "closed";
  more.setAttribute("aria-expanded", String(open));
  more.textContent = open ? "Less detail" : "More detail";
  syncSheet();
}

function applySheet(next: Sheet): void {
  mini = next === "collapsed";
  if (!mini && view === "summary") setMore(next === "expanded");
  if (next !== "expanded") byId("panel").scrollTop = 0;
  syncSheet();
}

/**
 * Change the sheet, and let it settle from the height it had (or from where
 * the finger left it) to its new one. Reduced motion, and the desktop
 * layout, change at once.
 */
function moveSheet(change: () => void, from?: number): void {
  const panel = byId("panel");
  window.clearTimeout(settleTimer);
  const h0 = from ?? panel.offsetHeight;
  const done = (): void => {
    settleH = null;
    delete document.body.dataset.settle;
    panel.style.height = "";
  };
  done();
  change();
  if (!phone() || still()) return;
  const h1 = panel.offsetHeight;
  if (Math.abs(h1 - h0) < 2) return;
  settleH = h1;
  panel.style.height = `${h0}px`;
  void panel.offsetHeight; // the start height is drawn before the move begins
  document.body.dataset.settle = "on";
  panel.style.height = `${h1}px`;
  settleTimer = window.setTimeout(done, SHEET_MS + 60);
}

more.addEventListener("click", () => moveSheet(() => setMore(document.body.dataset.more !== "open")));

/**
 * The handle at the top of the phone sheet. A tap, or Enter or Space, hides
 * the findings or brings them back. A drag makes the sheet as tall as the
 * finger asks, over everything the view holds; let go and it settles at the
 * nearest of the view's heights, or the next one in the direction of a flick.
 */
function wireGrab(): void {
  const grab = byId("grab");
  const panel = byId("panel");
  let start: { y: number; t: number; id: number; h: number; from: Sheet } | null = null;
  let dy = 0;
  let dragged = false;
  let dragEnd = -Infinity;
  const tallest = (): number => parseFloat(getComputedStyle(panel).maxHeight) || panel.offsetHeight;
  /** Where a drag that ended at height h, having moved dy at this speed, comes to rest. */
  const rest = (from: Sheet, h: number, speed: number): Sheet => {
    const stops = sheetStops();
    const at = Math.max(0, stops.indexOf(from));
    if (Math.abs(speed) > DRAG_FLICK) return stops[Math.max(0, Math.min(stops.length - 1, at + (speed < 0 ? 1 : -1)))]!;
    if (Math.abs(dy) < DRAG_SNAP) return stops[at]!;
    // Each height comes from its content, so measure them as they would stand.
    let best = stops[at]!;
    let gap = Infinity;
    for (const stop of stops) {
      applySheet(stop);
      const d = Math.abs(panel.offsetHeight - h);
      if (d < gap) [best, gap] = [stop, d];
    }
    return best;
  };
  grab.addEventListener("pointerdown", (e) => {
    if (e.button !== 0) return;
    start = { y: e.clientY, t: e.timeStamp, id: e.pointerId, h: panel.offsetHeight, from: sheetNow() };
    dy = 0;
    dragged = false;
    grab.setPointerCapture(e.pointerId);
  });
  grab.addEventListener("pointermove", (e) => {
    if (!start || e.pointerId !== start.id) return;
    dy = e.clientY - start.y;
    if (!dragged) {
      if (Math.abs(dy) < DRAG_TAP) return;
      dragged = true;
      window.clearTimeout(settleTimer);
      settleH = null;
      delete document.body.dataset.settle;
      document.body.dataset.drag = "on";
    }
    panel.style.height = `${Math.max(SHEET_MIN, Math.min(tallest(), start.h - dy))}px`;
  });
  grab.addEventListener("pointerup", (e) => {
    if (!start || e.pointerId !== start.id) return;
    const { from, t } = start;
    start = null;
    if (!dragged) return; // a tap: the click below
    dragEnd = e.timeStamp;
    const h = panel.offsetHeight;
    const speed = dy / Math.max(1, e.timeStamp - t); // px per ms, + is down
    delete document.body.dataset.drag;
    panel.style.height = "";
    const to = rest(from, h, speed);
    moveSheet(() => applySheet(to), h);
  });
  grab.addEventListener("pointercancel", () => {
    start = null;
    if (!dragged) return;
    dragged = false;
    delete document.body.dataset.drag;
    panel.style.height = "";
  });
  grab.addEventListener("click", (e) => {
    if (e.timeStamp - dragEnd < 400) return; // the click that ends a drag is not a tap
    moveSheet(() => {
      mini = !mini;
      if (mini) panel.scrollTop = 0;
      syncSheet();
    });
  });
}
wireGrab();

function wireMap(): void {
  const dropLayers = ["drops-key", "drops-rest"];
  const hit = (pt: maplibregl.Point): { drop?: LaneDrop; road?: Edge } => {
    const near = (r: number): [maplibregl.PointLike, maplibregl.PointLike] => [[pt.x - r, pt.y - r], [pt.x + r, pt.y + r]];
    // A marker wins where a marker and a road are both under the finger.
    const i = map.queryRenderedFeatures(near(DROP_HIT), { layers: dropLayers })[0]?.properties?.i;
    if (typeof i === "number" && drops[i]) return { drop: drops[i] };
    const id = map.queryRenderedFeatures(near(ROAD_HIT), { layers: ["drive-hit"] })[0]?.properties?.id;
    const road = typeof id === "number" ? edgeById.get(id) : undefined;
    return road ? { road } : {};
  };
  // A tap gives a quick look beside the feature; the panel keeps what it was showing.
  map.on("click", (e) => {
    const h = hit(e.point);
    if (state === "running") return;
    if (h.drop) openCard({ drop: h.drop }, [h.drop.pos.lon, h.drop.pos.lat]);
    else if (h.road) openCard({ road: h.road }, [e.lngLat.lng, e.lngLat.lat]);
    else closeCard();
  });
  map.on("mousemove", (e) => {
    const h = hit(e.point);
    map.getCanvas().style.cursor = h.drop || h.road ? "pointer" : "";
    // Pointing at a marker previews it; a click keeps the card open. Roads need a click.
    if (state !== "running" && canHover() && roomForCard()) {
      if (h.drop) openCard({ drop: h.drop }, [h.drop.pos.lon, h.drop.pos.lat], false);
      else leaveCard();
    }
    if (view === "list") {
      pickDrop(h.drop ?? null);
      for (const [d, row] of rowOf) row.classList.toggle("hot", d === h.drop);
    }
  });
  // The map grows and shrinks with the sheet on phones and with the window on desktop.
  // An open card keeps its place in view instead of the frame coming back.
  // On phones the sheet over the map grows and shrinks too (decision 0017):
  // the map keeps its size and the camera works in the part still showing.
  const box = map.getContainer();
  let size = "";
  const resized = new ResizeObserver(() => {
    const now = `${box.clientWidth}x${box.clientHeight}`;
    const mapResized = now !== size;
    if (mapResized) {
      size = now;
      map.resize();
    }
    // The credit line rides just above the sheet, mid-drag too.
    box.style.setProperty("--sheet-cover", `${sheetShown()}px`);
    // Only a change in the map's own size (a window resized, a phone turned)
    // re-frames it or moves a card. The sheet growing or shrinking over the
    // map does neither (decision 0018).
    if (!mapResized) return;
    // A map that shrank below a card's room moves the open card into the
    // sheet or panel instead of cutting it off.
    if (card?.popup && !roomForCard()) {
      const { what, at } = card;
      closeCard(true);
      openCard("properties" in what ? { road: what } : { drop: what }, at);
    } else if (card) revealCard();
    else if (!userMoved) frameMap(map.isEasing() ? "ease" : "jump");
  });
  resized.observe(box);
  resized.observe(byId("panel"));
}

byId("run-btn").addEventListener("click", () => void run());
byId("replay-btn").addEventListener("click", () => void run());
byId("skip-btn").addEventListener("click", () => finish());
const flowBtn = byId<HTMLButtonElement>("flow-btn");
flowBtn.addEventListener("click", () => {
  flowOn = !flowOn;
  // The words say what the button will do, so it needs no pressed state.
  flowBtn.textContent = flowOn ? "Hide flow" : "Show flow";
  byId("key-flow").hidden = !flowOn;
  syncFlow();
});

// The findings are words first: they show as soon as the data is in,
// whether or not the map has drawn.
renderList();
renderHow();
renderDirection(false);
// A link to one lane drop opens its detail (an unknown one opens the start).
const linkedDrop = opened.drop ? drops.find((d) => dropSlug(d.key) === opened.drop) : undefined;
if (linkedDrop) openDrop(linkedDrop, null);
document.body.dataset.view = view;
document.body.dataset.state = state;
document.body.dataset.load = "ready";

map.on("load", () => {
  mapReady = true;
  addLayers();
  drawPlaces();
  flow = initFlow(map, all, { dot: c.bg, edge: c.ink }, "drive-hit");
  renderMap();
  styleMap();
  wireMap();
  showAllDrops(view === "list");
  if (shownDrop) {
    // Opened from a link before the map was ready: mark and frame it now.
    pickDrop(shownDrop);
    focus(new maplibregl.LngLatBounds([shownDrop.pos.lon, shownDrop.pos.lat], [shownDrop.pos.lon, shownDrop.pos.lat]));
  }
});
