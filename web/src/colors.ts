// Site colours for MapLibre paint and canvas drawing. They are defined once,
// as CSS variables in style.css (:root); this reads them from the page and
// never repeats a hex value (docs/DESIGN.md §4).

export interface Colors {
  bg: string; // the page; knock-out behind markers
  ink: string; // the drive, markers, flow dots, the orb
  muted: string; // basemap labels
  road: string; // modelled roads that are not on the drive
  cut: string; // the bottleneck (min cut), and nothing else
  mapStreet: string; // trial basemap: streets
  mapWater: string; // trial basemap: water
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
    muted: read("--muted"),
    road: read("--road"),
    cut: read("--cut"),
    mapStreet: read("--map-street"),
    mapWater: read("--map-water"),
  };
  return cached;
}
