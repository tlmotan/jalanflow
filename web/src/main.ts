import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import type { FixesFile } from "./types";

const CORRIDOR = "federal-highway";
const DATA = `/data/${CORRIDOR}`;

// Free OSM-based style. Verify terms before launch (ARCHITECTURE.md §2).
const STYLE = "https://tiles.openfreemap.org/styles/positron";

const map = new maplibregl.Map({
  container: "map",
  style: STYLE,
  center: [101.6, 3.09],
  zoom: 11.5,
});

map.on("load", async () => {
  map.addSource("edges", { type: "geojson", data: `${DATA}/edges.geojson` });
  map.addLayer({
    id: "edges",
    type: "line",
    source: "edges",
    filter: ["==", ["get", "dir"], "inbound"],
    paint: {
      "line-width": ["interpolate", ["linear"], ["get", "lanes"], 1, 2, 5, 7],
      "line-color": [
        "case",
        ["get", "is_cut"], "#d62728",
        [">=", ["get", "utilisation"], 0.9], "#ff9f1c",
        "#4a90a4",
      ],
      "line-dasharray": ["case", ["get", "lanes_inferred"], ["literal", [2, 1]], ["literal", [1, 0]]],
    },
  });

  map.on("click", "edges", (e) => {
    const p = e.features?.[0]?.properties;
    if (!p) return;
    new maplibregl.Popup()
      .setLngLat(e.lngLat)
      .setHTML(
        `<b>${p.name || p.ref || "Unnamed road"}</b><br>` +
          `${p.lanes} lanes${p.lanes_inferred === true || p.lanes_inferred === "true" ? " (guessed)" : ""}<br>` +
          `Capacity ${p.capacity_vph} veh/h · flow ${p.flow_vph}` +
          (p.is_cut === true || p.is_cut === "true" ? "<br><b>Bottleneck</b>" : ""),
      )
      .addTo(map);
  });

  try {
    const fixes = (await (await fetch(`${DATA}/fixes.json`)).json()) as FixesFile;
    const ol = document.getElementById("fixes")!;
    for (const f of fixes.inbound.slice(0, 5)) {
      const li = document.createElement("li");
      li.textContent = `Edge ${f.edge_id}: ${f.change}, +${f.gain_vph} veh/h (${f.difficulty})`;
      ol.appendChild(li);
    }
  } catch {
    // No data yet: run `make analyze`.
  }
});
