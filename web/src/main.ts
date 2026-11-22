import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
// GeoJSON types ship with maplibre-gl (@types/geojson); no extra dependency.
import type { FeatureCollection, Point } from "geojson";
import type { FixesFile, LaneDrop, LaneDropRoad, LaneDropsFile } from "./types";

const CORRIDOR = "kemuning-pj";
const DATA = `/data/${CORRIDOR}`;

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
      "circle-color": ["case", ["get", "confirmed"], "#d62728", ["get", "guessed"], "#ffffff", "#ff9f1c"],
      "circle-stroke-color": ["case", ["get", "guessed"], "#ff9f1c", "#ffffff"],
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
    paint: { "text-color": ["case", ["get", "guessed"], "#333333", "#ffffff"] },
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
  // Keep drops clear of the side panel on wide screens; the panel sits at
  // the bottom on phones (style.css).
  const wide = map.getContainer().clientWidth > 600;
  const sheet = document.getElementById("panel")!.offsetHeight;
  map.fitBounds(bounds, {
    padding: wide ? { top: 40, bottom: 40, left: 380, right: 40 } : { top: 30, bottom: sheet + 20, left: 30, right: 30 },
    animate: false,
  });
}

// Max flow layers: only drawn once cmd/analyze writes edges.geojson / fixes.json.
async function addFlowLayers(): Promise<void> {
  const edges = await fetchJSON<FeatureCollection>(`${DATA}/edges.geojson`);
  if (edges) {
    map.addSource("edges", { type: "geojson", data: edges });
    map.addLayer(
      {
        id: "edges",
        type: "line",
        source: "edges",
        filter: ["==", ["get", "dir"], "inbound"],
        paint: {
          "line-width": ["interpolate", ["linear"], ["get", "lanes"], 1, 2, 5, 7],
          "line-color": ["case", ["get", "is_cut"], "#d62728", [">=", ["get", "utilisation"], 0.9], "#ff9f1c", "#4a90a4"],
          "line-dasharray": ["case", ["get", "lanes_inferred"], ["literal", [2, 1]], ["literal", [1, 0]]],
        },
      },
      map.getLayer("drops") ? "drops" : undefined,
    );
  }
  const fixes = await fetchJSON<FixesFile>(`${DATA}/fixes.json`);
  if (fixes) {
    const ol = document.getElementById("fixes")!;
    for (const f of fixes.inbound.slice(0, 5)) {
      ol.append(el("li", `Edge ${f.edge_id}: ${f.change}, +${f.gain_vph} veh/h (${f.difficulty})`));
    }
    document.getElementById("fixes-section")!.hidden = false;
  }
}

map.on("load", async () => {
  // The map can be created before style.css sizes #map; measure again.
  map.resize();
  const drops = await fetchJSON<LaneDropsFile>(`${DATA}/lane_drops.json`);
  if (drops) {
    renderPanel(drops);
    addDropLayers(drops);
  } else {
    document.getElementById("drops")!.append(el("li", "No data yet: run `make analyze`."));
  }
  await addFlowLayers();
});
