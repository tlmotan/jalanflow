// Trial looks, switched by URL parameter so they can be compared side by
// side. Decision 0013 leaves two things open: the basemap, and the road and
// car colours once lanes are drawn. When they are chosen, delete the losing
// options and the parameters.
//   ?base=none | pale | labels      ?lanes=pale | ink
import type { LayerSpecification, StyleSpecification } from "maplibre-gl";
import { colors } from "./colors";

// Free OSM-based style. Verify terms before launch (ARCHITECTURE.md §2).
const STYLE_URL = "https://tiles.openfreemap.org/styles/positron";
const LABELS_MIN_ZOOM = 13; // place names only once zoomed in

export type Base = "none" | "pale" | "labels";
export type LaneLook = "pale" | "ink";

export interface Trial {
  base: Base;
  lanes: LaneLook;
}

export function trial(): Trial {
  const q = new URLSearchParams(window.location.search);
  const base = q.get("base");
  const lanes = q.get("lanes");
  return {
    base: base === "none" || base === "labels" ? base : "pale",
    lanes: lanes === "ink" ? "ink" : "pale",
  };
}

/** Colours for the zoomed-in road and its cars (decision 0012). */
export interface LaneColours {
  surface: string;
  edge: string; // road edge (casing)
  laneLine: string;
  carEdge: string;
  carBody: string;
  carGlass: string;
}

export function laneColours(look: LaneLook): LaneColours {
  const c = colors();
  return look === "ink"
    ? { surface: c.ink, edge: c.ink, laneLine: c.bg, carEdge: c.ink, carBody: c.bg, carGlass: c.ink }
    : { surface: c.road, edge: c.ink, laneLine: c.bg, carEdge: c.bg, carBody: c.ink, carGlass: c.bg };
}

/** Credit for our own road data, shown when no basemap carries it. */
export const OSM_CREDIT = '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap contributors</a>, ODbL';

/** No tiles: the modelled roads on the plain page, as in the mockup. */
function plain(): StyleSpecification {
  return {
    version: 8,
    sources: {},
    layers: [{ id: "background", type: "background", paint: { "background-color": colors().bg } }],
  };
}

/** The basemap drained to greys close to the page, so only our ink stands out. */
function drain(layer: LayerSpecification, base: Base): LayerSpecification {
  const c = colors();
  const hide = (l: LayerSpecification): LayerSpecification => ({ ...l, layout: { ...l.layout, visibility: "none" } });
  const water = layer.id.includes("water");
  switch (layer.type) {
    case "background":
      return { ...layer, paint: { "background-color": c.bg } };
    case "fill":
      // Only water keeps a tint; parks, buildings and land use melt into the page.
      return water ? { ...layer, paint: { "fill-color": c.mapWater } } : hide(layer);
    case "line":
      if (layer.id.includes("rail") || layer.id.includes("boundary") || layer.id.includes("aeroway")) return hide(layer);
      return { ...layer, paint: { ...layer.paint, "line-color": water ? c.mapWater : c.mapStreet } };
    case "symbol":
      if (base !== "labels" || !layer.id.startsWith("label_")) return hide(layer);
      return {
        ...layer,
        minzoom: Math.max(layer.minzoom ?? 0, LABELS_MIN_ZOOM),
        paint: { "text-color": c.muted, "text-halo-color": c.bg, "text-halo-width": 1.5 },
      };
    default:
      return hide(layer);
  }
}

/** The map style for a trial basemap. Falls back to no tiles if the style cannot be fetched. */
export async function baseStyle(base: Base): Promise<StyleSpecification> {
  if (base === "none") return plain();
  try {
    const res = await fetch(STYLE_URL);
    if (!res.ok) return plain();
    const style = (await res.json()) as StyleSpecification;
    return { ...style, layers: style.layers.map((l) => drain(l, base)) };
  } catch {
    return plain();
  }
}
