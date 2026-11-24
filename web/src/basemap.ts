// The basemap (decision 0013, decided 2026-11-24): the OpenFreeMap style
// drained to pale greys close to the page, with no labels, so only our ink
// stands out.
import type { LayerSpecification, StyleSpecification } from "maplibre-gl";
import { colors } from "./colors";

// Free OSM-based style. Verify terms before launch (ARCHITECTURE.md §2).
const STYLE_URL = "https://tiles.openfreemap.org/styles/positron";

/** Credit for our own road data, shown when no basemap carries it. */
export const OSM_CREDIT = '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap contributors</a>, ODbL';

/** No tiles: the modelled roads on the plain page. Used only if the basemap cannot be fetched. */
function plain(): StyleSpecification {
  return {
    version: 8,
    sources: {},
    layers: [{ id: "background", type: "background", paint: { "background-color": colors().bg } }],
  };
}

/** One basemap layer drained to greys close to the page. */
function drain(layer: LayerSpecification): LayerSpecification {
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
    default:
      // No labels or icons: places are named by our own markers.
      return hide(layer);
  }
}

export interface Base {
  style: StyleSpecification;
  /** False when the basemap could not be fetched: our own roads must then carry the OSM credit. */
  tiles: boolean;
}

/** The pale grey basemap. Falls back to the plain page if the style cannot be fetched. */
export async function baseStyle(): Promise<Base> {
  try {
    const res = await fetch(STYLE_URL);
    if (!res.ok) return { style: plain(), tiles: false };
    const style = (await res.json()) as StyleSpecification;
    return { style: { ...style, layers: style.layers.map(drain) }, tiles: true };
  } catch {
    return { style: plain(), tiles: false };
  }
}
