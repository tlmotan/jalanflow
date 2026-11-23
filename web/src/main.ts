import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
// GeoJSON types ship with maplibre-gl (@types/geojson); no extra dependency.
import type { FeatureCollection, LineString, Point } from "geojson";
import type { EdgeProps, FixesFile, LaneDrop, LaneDropRoad, LaneDropsFile, ReplayFile } from "./types";
import { COLORS, NEARLY_FULL } from "./colors";
import { initReplay } from "./replay";
import { initFlowDots } from "./particles";

const CORRIDOR = "kemuning-pj";
// BASE_URL is "/" locally and "/jalanflow/" on GitHub Pages (vite --base).
const DATA = `${import.meta.env.BASE_URL}data/${CORRIDOR}`;

// Free OSM-based style. Verify terms before launch (ARCHITECTURE.md §2).
const STYLE = "https://tiles.openfreemap.org/styles/positron";
// Font served by the style above, used for rank labels.
const LABEL_FONT = ["Noto Sans Regular"];

const map = new maplibregl.Map({
  container: "map",
  style: STYLE,
  center: [101.58, 3.06],
  zoom: 11.5,
});
map.addControl(new maplibregl.NavigationControl(), "top-right");

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

function el<K extends keyof HTMLElementTagNameMap>(tag: K, text?: string, cls?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (text !== undefined) e.textContent = text;
  if (cls) e.className = cls;
  return e;
}

// short: ref only (e.g. "E11") for titles; full adds the name for popups.
function roadLabel(r: LaneDropRoad, short = false): string {
  if (r.ref && r.name) return short ? r.ref : `${r.ref} ${r.name}`;
  if (r.ref || r.name) return r.ref || r.name;
  if (!r.class.endsWith("_link")) return r.class;
  if (r.leaves && r.joins && r.leaves !== r.joins) return `ramp ${r.leaves} → ${r.joins}`;
  if (r.leaves && r.joins) return `ramp on ${r.leaves}`;
  if (r.joins) return `ramp onto ${r.joins}`;
  if (r.leaves) return `ramp off ${r.leaves}`;
  return "ramp";
}

function title(d: LaneDrop): string {
  if (d.confirmed) return d.confirmed.name;
  const first = d.to[0] ?? d.from[0];
  if (d.kind === "narrowing" && first) return `${roadLabel(first, true)} narrows`;
  const into = [...new Set(d.to.map((r) => roadLabel(r, true)))].join(" + ");
  const roads = [...new Set(d.from.map((r) => roadLabel(r, true)))];
  return `${roads.join(" + ")} merge${into ? ` into ${into}` : ""}`;
}

function lanesText(d: LaneDrop): string {
  return `${d.lanes_in} → ${d.lanes_out} lanes (loses ${d.lanes_lost})`;
}

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

function roadLines(label: string, roads: LaneDropRoad[]): HTMLElement {
  const p = el("div", `${label}: `, "roads");
  p.append(
    roads
      .map((r) => `${roadLabel(r)} (${r.lanes}${r.lanes_inferred ? ", guessed" : ""})`)
      .join(" + "),
  );
  return p;
}

function popupContent(d: LaneDrop): HTMLElement {
  const box = el("div", undefined, "popup");
  box.append(el("b", `#${d.rank} ${title(d)}`));
  box.append(el("div", lanesText(d)));
  box.append(el("div", d.kind === "merge" ? "Roads join here and lanes are lost." : "The road loses lanes here.", "note"));
  box.append(roadLines("In", d.from), roadLines("Out", d.to));
  if (d.confirmed) box.append(el("div", `Confirmed: ${d.confirmed.source}`, "badge confirmed"));
  if (d.lanes_inferred) {
    box.append(el("div", "Some lane counts here are guesses (no lanes tag in OSM). Check on Street View.", "badge guessed"));
  }
  const links = el("div", undefined, "links");
  const [osmType, osmId] = d.key.startsWith("node:")
    ? ["node", d.key.slice(5)]
    : ["way", String(d.to[0]?.osm_way_id ?? "")];
  links.append(
    link(`https://www.google.com/maps/@?api=1&map_action=pano&viewpoint=${coords(d)}`, "Street View"),
    document.createTextNode(" · "),
    link(`https://www.google.com/maps/search/?api=1&query=${coords(d)}`, "Google Maps"),
    document.createTextNode(" · "),
    link(`https://www.openstreetmap.org/${osmType}/${osmId}`, "OSM"),
  );
  box.append(links);
  return box;
}

let popup: maplibregl.Popup | null = null;

function show(d: LaneDrop, fly: boolean): void {
  const at: [number, number] = [d.pos.lon, d.pos.lat];
  if (fly) {
    // Land the point right of the side panel (wide screens) or above the
    // bottom sheet (phones), a little low so the popup above it fits.
    const wide = map.getContainer().clientWidth > 600;
    map.flyTo({ center: at, zoom: Math.max(map.getZoom(), 15), offset: wide ? [170, 80] : [0, -40] });
  }
  popup?.remove();
  popup = new maplibregl.Popup({ maxWidth: "300px" }).setLngLat(at).setDOMContent(popupContent(d)).addTo(map);
}

function listItem(d: LaneDrop): HTMLLIElement {
  const li = el("li");
  li.value = d.rank;
  const btn = el("button", undefined, "drop");
  btn.type = "button";
  btn.append(el("span", title(d), "name"), el("span", lanesText(d) + (d.lanes_inferred ? " · guessed" : ""), "lanes"));
  if (d.confirmed) li.classList.add("confirmed");
  btn.addEventListener("click", () => show(d, true));
  li.append(btn);
  return li;
}

function renderPanel(f: LaneDropsFile): void {
  const confirmed = f.drops.filter((d) => d.confirmed);
  document.getElementById("confirmed")!.append(...confirmed.map(listItem));
  document.getElementById("drops")!.append(...f.drops.map(listItem));
  document.getElementById("all-heading")!.textContent = `All lane drops (${f.drops.length}), biggest squeeze first`;
  document.getElementById("ranking")!.textContent = f.ranking;
  document.getElementById("assumptions")!.append(...f.assumptions.map((a) => el("li", a)));
  document.getElementById("osm-date")!.textContent = `OpenStreetMap data from ${f.osm_date}.`;
}

function addDropLayers(f: LaneDropsFile): void {
  const data: FeatureCollection<Point> = {
    type: "FeatureCollection",
    features: f.drops.map((d, i) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [d.pos.lon, d.pos.lat] },
      properties: {
        i,
        rank: d.rank,
        lost: d.lanes_lost,
        confirmed: d.confirmed !== null,
        guessed: d.lanes_inferred,
      },
    })),
  };
  map.addSource("drops", { type: "geojson", data });
  map.addLayer({
    id: "drops",
    type: "circle",
    source: "drops",
    paint: {
      "circle-radius": ["interpolate", ["linear"], ["get", "lost"], 1, 7, 4, 13],
      "circle-color": ["case", ["get", "confirmed"], COLORS.cut, ["get", "guessed"], "#ffffff", COLORS.full],
      "circle-stroke-color": ["case", ["get", "guessed"], COLORS.full, "#ffffff"],
      "circle-stroke-width": 2,
    },
  });
  map.addLayer({
    id: "drop-ranks",
    type: "symbol",
    source: "drops",
    minzoom: 12,
    layout: {
      "text-field": ["to-string", ["get", "rank"]],
      "text-font": LABEL_FONT,
      "text-size": 11,
      "text-allow-overlap": true,
    },
    paint: { "text-color": ["case", ["get", "guessed"], COLORS.ink, "#ffffff"] },
  });
  map.on("click", "drops", (e) => {
    const i = e.features?.[0]?.properties?.["i"];
    const d = typeof i === "number" ? f.drops[i] : undefined;
    if (d) show(d, false);
  });
  map.on("mouseenter", "drops", () => (map.getCanvas().style.cursor = "pointer"));
  map.on("mouseleave", "drops", () => (map.getCanvas().style.cursor = ""));

  const bounds = new maplibregl.LngLatBounds();
  for (const d of f.drops) bounds.extend([d.pos.lon, d.pos.lat]);
  if (bounds.isEmpty()) return;
  map.fitBounds(bounds, { padding: panelPadding(), animate: false });
}

// Keep fitted features clear of the side panel on wide screens; the panel
// sits at the bottom on phones (style.css, docs/DESIGN.md §5).
function panelPadding(): maplibregl.PaddingOptions {
  const wide = map.getContainer().clientWidth > 600;
  const sheet = document.getElementById("panel")!.offsetHeight;
  return wide ? { top: 40, bottom: 40, left: 380, right: 40 } : { top: 30, bottom: sheet + 20, left: 30, right: 30 };
}

// Max flow layers: only drawn once cmd/analyze writes edges.geojson / fixes.json.
// The site shows the inbound direction (Kota Kemuning → PJ).
async function addFlowLayers(): Promise<void> {
  const all = await edgesData;
  if (all) {
    const edges: FeatureCollection<LineString, EdgeProps> = {
      type: "FeatureCollection",
      features: all.features.filter((f) => f.properties.dir === "inbound"),
    };
    // promoteId lets the replay colour roads by feature-state "f" (flow so
    // far). With no state set, roads show the final result from properties.
    map.addSource("edges", { type: "geojson", data: edges, promoteId: "id" });
    const before = map.getLayer("drops") ? "drops" : undefined;
    map.addLayer(
      {
        id: "edges",
        type: "line",
        source: "edges",
        paint: {
          "line-width": ["interpolate", ["linear"], ["get", "lanes"], 1, 2, 5, 7],
          "line-color": [
            "case",
            ["==", ["typeof", ["feature-state", "f"]], "number"],
            [
              "case",
              ["<=", ["feature-state", "f"], 0],
              COLORS.idle,
              [">=", ["/", ["feature-state", "f"], ["get", "capacity_vph"]], NEARLY_FULL],
              COLORS.full,
              COLORS.spare,
            ],
            ["get", "is_cut"],
            COLORS.cut,
            [">=", ["get", "utilisation"], NEARLY_FULL],
            COLORS.full,
            COLORS.spare,
          ],
          // Roads whose flow a later route cancelled end at 0: replay only.
          "line-opacity": [
            "case",
            ["all", ["!=", ["typeof", ["feature-state", "f"]], "number"], ["<=", ["get", "flow_vph"], 0]],
            0,
            1,
          ],
          "line-dasharray": ["case", ["get", "lanes_inferred"], ["literal", [2, 1]], ["literal", [1, 0]]],
        },
      },
      before,
    );
    const replay = await replayData;
    if (replay) addReplay(edges, replay, before);
  }
  const fixes = await fixesData;
  if (fixes) {
    const ol = document.getElementById("fixes")!;
    // Only fixes that raise max flow on their own; the rest are explained
    // by why_no_gain (another equally tight road elsewhere).
    for (const f of fixes.inbound.filter((x) => x.helps).slice(0, 5)) {
      ol.append(el("li", `Edge ${f.edge_id}: ${f.change}, +${f.gain_vph} veh/h (${f.difficulty})`));
    }
    const noGain = fixes.inbound.filter((x) => !x.helps).length;
    if (noGain > 0) {
      ol.after(el("p", `${noGain} more bottleneck road${noGain === 1 ? "" : "s"} would gain nothing widened alone: another road is just as tight.`));
    }
    document.getElementById("fixes-section")!.hidden = false;
  }
}

function addReplay(edges: FeatureCollection<LineString, EdgeProps>, replay: ReplayFile, before?: string): void {
  const dots = initFlowDots(map, edges, before);
  const flowBtn = document.getElementById("flow-btn") as HTMLButtonElement;
  const flowNote = document.getElementById("flow-note")!;
  const setDots = (on: boolean, animate: boolean): void => {
    if (on) dots.start(animate);
    else dots.stop();
    flowBtn.setAttribute("aria-pressed", String(on));
    flowBtn.textContent = on ? "Hide flow" : "Show flow";
    flowNote.hidden = !on;
  };
  const still = (): boolean => window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  flowBtn.addEventListener("click", () => setDots(!dots.on, !still()));
  initReplay({
    map,
    edges,
    replay: replay.inbound,
    padding: panelPadding,
    beforeLayer: before,
    onStart: () => {
      popup?.remove();
      setDots(false, false);
      flowBtn.hidden = true;
    },
    onFinish: () => {
      flowBtn.hidden = false;
      // Reduced motion: no moving dots unless asked (docs/DESIGN.md §6).
      if (!still()) setDots(true, true);
    },
  });
}

// The side panel does not need the map: fill it as soon as the data arrives.
const panelReady = dropsData.then((drops) => {
  if (drops) renderPanel(drops);
  else document.getElementById("drops")!.append(el("li", "No data yet: run `make analyze`."));
  return drops;
});

map.on("load", async () => {
  // The map can be created before style.css sizes #map; measure again.
  map.resize();
  // Panel first: addDropLayers measures it to pad the map on phones.
  const drops = await panelReady;
  if (drops) addDropLayers(drops);
  await addFlowLayers();
});
