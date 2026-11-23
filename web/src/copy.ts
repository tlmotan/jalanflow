// The site's sentences, built from the data (docs/CONTENT.md). Pure: no DOM
// and no map, so `npm test` can check the wording.
import type { Fix, LaneDrop, LaneDropRoad } from "./types.ts";

/** Round to the nearest 100 veh/h with thousands separators (CONTENT.md). */
export function fmtVph(n: number): string {
  return (Math.round(n / 100) * 100).toLocaleString("en-US");
}

const WORDS = ["no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"];
/** Small counts as words inside a sentence. */
export function countWord(n: number): string {
  return WORDS[n] ?? String(n);
}

/** short: ref only ("E11") for titles; full adds the name. */
export function roadLabel(r: LaneDropRoad, short = false): string {
  if (r.ref && r.name) return short ? r.ref : `${r.ref} ${r.name}`;
  if (r.ref || r.name) return r.ref || r.name;
  if (!r.class.endsWith("_link")) return r.class;
  if (r.leaves && r.joins && r.leaves !== r.joins) return `ramp ${r.leaves} → ${r.joins}`;
  if (r.leaves && r.joins) return `ramp on ${r.leaves}`;
  if (r.joins) return `ramp onto ${r.joins}`;
  if (r.leaves) return `ramp off ${r.leaves}`;
  return "ramp";
}

/** A lane drop's name: the confirmed jam's local name, or its roads. */
export function dropTitle(d: LaneDrop): string {
  if (d.confirmed) return d.confirmed.name;
  const first = d.to[0] ?? d.from[0];
  if (d.kind === "narrowing" && first) return `${roadLabel(first, true)} narrows`;
  const into = [...new Set(d.to.map((r) => roadLabel(r, true)))].join(" + ");
  const roads = [...new Set(d.from.map((r) => roadLabel(r, true)))];
  return `${roads.join(" + ")} merge${into ? ` into ${into}` : ""}`;
}

/** "3 → 2 lanes" (CONTENT.md, numbers). */
export function lanesText(d: LaneDrop): string {
  return `${d.lanes_in} → ${d.lanes_out} lanes`;
}

export function dropBody(d: LaneDrop): string {
  return `${d.lanes_in} lanes come in, ${d.lanes_out} carry on.`;
}

/** Second sentence on the landing view; empty when nothing is confirmed. */
export function confirmedSentence(confirmed: number): string {
  if (confirmed <= 0) return "";
  if (confirmed === 1) return "A local driver confirmed one of them as a daily jam.";
  return `A local driver confirmed ${countWord(confirmed)} of them as daily jams.`;
}

export function resultSentence(maxFlowVph: number): string {
  return `At most ${fmtVph(maxFlowVph)} vehicles an hour can get from Kota Kemuning to PJ.`;
}

/** A road on the bottleneck, as edges.geojson describes it. */
export interface CutRoad {
  id: number;
  ref: string;
  name: string;
  class: string;
  lanes: number;
}

function cutName(r: CutRoad): string {
  return [r.ref, r.name].filter(Boolean).join(" ") || r.class;
}

/** Bottleneck roads with the same name and lane count, counted once. */
function stretches(cut: CutRoad[]): { name: string; lanes: number }[] {
  const seen = new Map<string, { name: string; lanes: number }>();
  for (const r of cut) seen.set(`${cutName(r)}|${r.lanes}`, { name: cutName(r), lanes: r.lanes });
  return [...seen.values()];
}

/** "It runs out at one 3-lane stretch of E5 Lebuhraya Shah Alam." */
export function bottleneckSentence(cut: CutRoad[]): string {
  const list = stretches(cut);
  if (list.length === 0) return "";
  if (list.length === 1 && cut.length === 1) {
    const s = list[0]!;
    return `It runs out at one ${s.lanes}-lane stretch of ${s.name}.`;
  }
  const parts = list.map((s) => `${s.name} (${s.lanes} lanes)`);
  const joined = parts.length > 1 ? `${parts.slice(0, -1).join(", ")} and ${parts[parts.length - 1]}` : parts[0]!;
  return `It runs out at ${countWord(cut.length)} stretches taken together: ${joined}.`;
}

/** [before, number to stress, after]; the middle part is empty when there is no number. */
export type FixParts = [string, string, string];

/**
 * The tested fix as a sentence. `fixes` are the +1 lane tests for this
 * direction; the one with the biggest gain leads.
 */
export function fixSentence(fixes: Fix[], cut: CutRoad[]): FixParts {
  if (fixes.length === 0) return ["", "", ""];
  const best = fixes.filter((f) => f.helps).sort((a, b) => b.gain_vph - a.gain_vph)[0];
  if (!best) {
    return ["One more lane on any single bottleneck road would gain nothing: another road is just as tight.", "", ""];
  }
  const road = cut.find((r) => r.id === best.edge_id);
  const where = cut.length === 1 || !road ? "there" : `on ${cutName(road)}`;
  const stays = best.next_cut_edges.includes(best.edge_id);
  return [
    `One more lane ${where} would raise the limit to `,
    fmtVph(best.new_max_flow_vph),
    stays ? ", and that stretch would still be the bottleneck." : ", and the bottleneck would move to another road.",
  ];
}

/** How hard the leading fix is to build, with the pipeline's reasons. */
export function fixDifficulty(fixes: Fix[]): string {
  const best = fixes.filter((f) => f.helps).sort((a, b) => b.gain_vph - a.gain_vph)[0];
  if (!best) return "";
  const why = best.reasons.length ? `: ${best.reasons.join("; ")}` : "";
  return `Rated ${best.difficulty} to build${why}.`;
}

export function pathCaption(i: number, n: number): string {
  return `Path ${Math.min(i, n)} of ${n}.`;
}
