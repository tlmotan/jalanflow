// The lane strip in the result view (docs/design/project/DESIGN_BRIEF.md §13): the lanes
// along the part of the drive that every route uses, so a reader can see a
// wide road and the narrow stretch that limits it side by side. Pure, no DOM
// or MapLibre: tested with `npm test`.
//
// Nothing is computed about traffic here. Lanes, flow, capacity and the
// bottleneck all come from edges.geojson as the Go pipeline exported them.
import { track } from "./track.ts";
import { NEARLY_FULL } from "./copy.ts";

/** One road of the analysed direction, from edges.geojson. */
export interface ProfileRoad {
  id: number;
  coords: number[][]; // [lon, lat], in travel order
  ref: string;
  name: string;
  class: string;
  lanes: number;
  lanes_inferred: boolean;
  capacity_vph: number;
  flow_vph: number;
  is_cut: boolean;
}

/** A run of the shared part with one lane count and one mark. */
export interface Stretch {
  lanes: number;
  km: number;
  guessed: boolean; // some lane count in it is guessed
  cut: boolean; // the bottleneck
  full: boolean; // nearly full, and not the bottleneck
}

const nodeKey = (p: number[] | undefined): string => `${p?.[0]},${p?.[1]}`;

/**
 * The roads every route uses, in travel order: from where the traffic
 * starts, follow the road that carries the whole of the max flow until it
 * first shares it with another road. Empty if the traffic splits at once.
 */
export function sharedRoads(roads: ProfileRoad[], maxFlowVph: number): ProfileRoad[] {
  if (!(maxFlowVph > 0)) return [];
  const out = new Map<string, ProfileRoad[]>();
  const arrives = new Set<string>();
  for (const r of roads) {
    if (!(r.flow_vph > 0) || r.coords.length < 2) continue;
    const from = nodeKey(r.coords[0]);
    out.set(from, [...(out.get(from) ?? []), r]);
    arrives.add(nodeKey(r.coords[r.coords.length - 1]));
  }
  // The start: traffic leaves it and none arrives. Roads there must carry
  // everything between them, so a single road with the whole flow is the way in.
  let node: string | undefined;
  for (const [from, list] of out) {
    if (!arrives.has(from) && list.some((r) => r.flow_vph >= maxFlowVph - 0.5)) node = from;
  }
  const path: ProfileRoad[] = [];
  const seen = new Set<string>();
  while (node !== undefined && !seen.has(node)) {
    seen.add(node);
    const next = (out.get(node) ?? []).find((r) => r.flow_vph >= maxFlowVph - 0.5);
    if (!next) break;
    path.push(next);
    node = nodeKey(next.coords[next.coords.length - 1]);
  }
  return path;
}

/** Neighbouring roads with the same lane count and the same mark, joined. */
export function stretches(path: ProfileRoad[]): Stretch[] {
  const list: Stretch[] = [];
  for (const r of path) {
    const cut = r.is_cut;
    const full = !cut && r.capacity_vph > 0 && r.flow_vph / r.capacity_vph >= NEARLY_FULL;
    const km = track(r.coords).km;
    const last = list[list.length - 1];
    if (last && last.lanes === r.lanes && last.cut === cut && last.full === full) {
      last.km += km;
      last.guessed ||= r.lanes_inferred;
    } else {
      list.push({ lanes: r.lanes, km, guessed: r.lanes_inferred, cut, full });
    }
  }
  return list;
}
