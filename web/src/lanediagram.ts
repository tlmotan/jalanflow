// The lane diagram on a lane drop's map card (decision 0015, cards v2): the
// lanes coming in and going out, drawn as the zoomed-in map draws lanes
// (decision 0012: equal stripes, schematic). Travel runs left to right, so
// the driver's left is the top. Pure: geometry only, no DOM, so `npm test`
// checks it.
//
// Everything comes from lane_drops.json: each road's lane count and, at a
// merge, the side it joins on (`side`, worked out from the OSM geometry in
// the pipeline). Where the data does not say which side lanes are lost on
// (every narrowing, and merges it cannot tell apart), both edges taper.
import type { LaneDrop, LaneDropRoad } from "./types.ts";

/** Diagram size and layout, in SVG units (px at 1:1). */
export const DIAGRAM = {
  width: 240,
  lane: 10, // one lane's width
  top: 18, // room for the lane counts above the road
  bottom: 6,
  goreEnd: 80, // where the solid line between two joining roads gives way
  taperStart: 112,
  taperEnd: 170,
} as const;

export interface Line {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  kind: "lane" | "gore"; // dashed lane line, or the solid line where roads join
}

export interface Diagram {
  width: number;
  height: number;
  road: string; // SVG path of the road surface
  lines: Line[];
  inLabel: string; // "2 + 3": each road coming in, left to right
  outLabel: string;
  guessed: boolean; // some lane count is guessed: dashed edge
  joinSide: "left" | "right" | null; // where a road joins, when the data says
}

/**
 * Where the lost lanes leave the road. At a merge of two roads with known
 * sides, they go on the side of the road that does not carry on (the ramp,
 * or the road whose ref is not the one going out). Otherwise null: taper
 * both edges.
 */
export function joiningSide(d: LaneDrop): "left" | "right" | null {
  if (d.kind !== "merge" || d.from.length !== 2 || d.to.length !== 1) return null;
  const out = d.to[0]!;
  const carries = (r: LaneDropRoad) => !r.class.endsWith("_link") && r.ref !== "" && r.ref === out.ref;
  const through = d.from.filter(carries);
  if (through.length !== 1) return null;
  const joining = d.from.find((r) => r !== through[0])!;
  return joining.side === "left" || joining.side === "right" ? joining.side : null;
}

/** Roads coming in, left to right across the carriageway, when every side is known. */
function ordered(roads: LaneDropRoad[]): LaneDropRoad[] | null {
  const rank = { left: 0, middle: 1, right: 2 } as const;
  if (roads.length < 2 || roads.some((r) => r.side === "")) return null;
  return [...roads].sort((a, b) => rank[a.side as keyof typeof rank] - rank[b.side as keyof typeof rank]);
}

export function laneDiagram(d: LaneDrop): Diagram {
  const { width: W, lane, top: T, taperStart: x0, taperEnd: x1 } = DIAGRAM;
  const n = Math.max(1, d.lanes_in);
  const m = Math.max(1, Math.min(n, d.lanes_out));
  const side = joiningSide(d);
  const inBottom = T + n * lane;
  // The out lanes keep the edge of the road that carries on.
  const outTop = side === "left" ? T + (n - m) * lane : side === "right" ? T : T + ((n - m) * lane) / 2;
  const outBottom = outTop + m * lane;
  const road = `M0 ${T} L${x0} ${T} L${x1} ${outTop} L${W} ${outTop} L${W} ${outBottom} L${x1} ${outBottom} L${x0} ${inBottom} L0 ${inBottom} Z`;

  const groups = d.kind === "merge" ? ordered(d.from) : null;
  // Lane counts of the roads coming in, as stacked: boundaries between roads are gores.
  const sizes = groups ? groups.map((r) => r.lanes) : [n];
  const goreAt = new Set<number>();
  let acc = 0;
  for (const s of sizes.slice(0, -1)) {
    acc += s;
    goreAt.add(acc);
  }

  const lines: Line[] = [];
  for (let k = 1; k < n; k++) {
    const y = T + k * lane;
    // A line runs on only if it is also a line between the lanes going out.
    // A lost one stops just before the tapering edge reaches it; one that
    // falls between two out lanes (both edges taper) stops at the taper.
    const continues = y > outTop && y < outBottom && (y - outTop) % lane === 0;
    let end = W;
    if (y <= outTop) end = x0 + ((x1 - x0) * (y - T)) / Math.max(1e-9, outTop - T) - 8;
    else if (y >= outBottom) end = x0 + ((x1 - x0) * (inBottom - y)) / Math.max(1e-9, inBottom - outBottom) - 8;
    else if (!continues) end = x0 - 8;
    if (end < 0) continue;
    if (goreAt.has(k)) {
      lines.push({ x1: 0, y1: y, x2: Math.min(DIAGRAM.goreEnd, end), y2: y, kind: "gore" });
      if (end > DIAGRAM.goreEnd) lines.push({ x1: DIAGRAM.goreEnd, y1: y, x2: end, y2: y, kind: "lane" });
    } else {
      lines.push({ x1: 0, y1: y, x2: end, y2: y, kind: "lane" });
    }
  }
  // Out lanes that do not line up with an incoming line (both edges tapered).
  for (let k = 1; k < m; k++) {
    const y = outTop + k * lane;
    if (!lines.some((l) => l.y1 === y && l.x2 === W)) lines.push({ x1: x1, y1: y, x2: W, y2: y, kind: "lane" });
  }

  return {
    width: W,
    height: inBottom + DIAGRAM.bottom,
    road,
    lines,
    inLabel: groups ? groups.map((r) => r.lanes).join(" + ") : String(d.lanes_in),
    outLabel: String(d.lanes_out),
    guessed: d.lanes_inferred,
    joinSide: side,
  };
}
