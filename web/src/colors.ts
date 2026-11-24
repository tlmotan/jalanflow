// Site colours for MapLibre paint and canvas drawing. They are defined once,
// as CSS variables in style.css (:root); this reads them from the page and
// never repeats a hex value (docs/design/project/DESIGN_BRIEF.md §4).

export interface Colors {
  bg: string; // the page; knock-out behind markers
  ink: string; // the drive, markers, flow dots, the orb
  road: string; // modelled roads that are not on the drive
  cut: string; // where it jams: confirmed jams and the bottleneck
  full: string; // roads the analysis fills to 90 % or more
  mapStreet: string; // basemap streets
  mapWater: string; // basemap water
}

let cached: Colors | null = null;

/** The palette from style.css. Call after the stylesheet has loaded. */
export function colors(): Colors {
  if (cached) return cached;
  const css = getComputedStyle(document.documentElement);
  const read = (name: string): string => css.getPropertyValue(name).trim();
  cached = {
    bg: read("--bg"),
    ink: read("--ink"),
    road: read("--road"),
    cut: read("--cut"),
    full: read("--full"),
    mapStreet: read("--map-street"),
    mapWater: read("--map-water"),
  };
  return cached;
}

/**
 * Colours for the road and its cars once lanes are drawn (decisions 0012,
 * 0013): a pale road with an ink edge, and small ink cars.
 */
export interface LaneColours {
  surface: string;
  edge: string; // road edge (casing)
  laneLine: string;
  carEdge: string; // knock-out around a car
  carBody: string;
  carGlass: string;
}

export function laneColours(): LaneColours {
  const c = colors();
  return { surface: c.road, edge: c.ink, laneLine: c.bg, carEdge: c.bg, carBody: c.ink, carGlass: c.bg };
}
