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

/** A road counts as nearly full from this share of what it can carry (shown yellow on the map). */
export const NEARLY_FULL = 0.9;

/** A road on the map, as edges.geojson describes it. */
export interface RoadFacts extends CutRoad {
  lanes_inferred: boolean;
  capacity_vph: number;
  flow_vph: number;
  is_cut: boolean;
}

/** A road's name for a title: "E5 Lebuhraya Shah Alam", or what kind of road it is. */
export function roadTitle(r: RoadFacts): string {
  const named = [r.ref, r.name].filter(Boolean).join(" ");
  if (named) return named;
  return r.class.endsWith("_link") ? "Ramp" : "Unnamed road";
}

/** "3 lanes, able to carry about 6,000 veh/h." */
export function roadBody(r: RoadFacts): string {
  const lanes = `${r.lanes} lane${r.lanes === 1 ? "" : "s"}`;
  return `${lanes}, able to carry about ${fmtVph(r.capacity_vph)} veh/h.`;
}

/**
 * What else to say about a road. The share it carries and whether it is the
 * bottleneck are results of the analysis, so they are held back until it
 * has run (`solved`). `cutCount` is how many roads make up the bottleneck.
 */
export function roadNotes(r: RoadFacts, solved: boolean, cutCount: number): string[] {
  const notes: string[] = [];
  if (r.lanes_inferred) notes.push("The lane count is guessed from the road type. Check on Street View.");
  if (!solved) return notes;
  if (r.is_cut) {
    notes.push(
      cutCount <= 1
        ? "This is the bottleneck. Every route from Kota Kemuning to PJ has to cross this stretch, and the analysis fills it completely, so nothing more gets through."
        : "This is part of the bottleneck. Every route from Kota Kemuning to PJ has to cross one of the red stretches, and the analysis fills them all, so nothing more gets through.",
    );
  } else if (r.flow_vph > 0) {
    const full = r.capacity_vph > 0 && r.flow_vph / r.capacity_vph >= NEARLY_FULL;
    notes.push(
      full
        ? `The analysis sends ${fmtVph(r.flow_vph)} veh/h along it, so it is nearly full. It is not what sets the limit: the bottleneck is.`
        : `The analysis sends ${fmtVph(r.flow_vph)} veh/h along it, so it has room to spare.`,
    );
  }
  return notes;
}

export function pathCaption(i: number, n: number): string {
  return `Path ${Math.min(i, n)} of ${n}.`;
}

/** A stretch of the lane strip (profile.ts), as far as the wording needs it. */
export interface ProfilePart {
  lanes: number;
  km: number;
  cut: boolean;
  full: boolean;
}

/** The roads' shared name ("E5 Lebuhraya Shah Alam"), or "" if they differ. */
export function sharedName(roads: CutRoad[]): string {
  const names = new Set(roads.map(cutName));
  return names.size === 1 ? [...names][0]! : "";
}

/** "Lanes along the first 4.9 km of E5 Lebuhraya Shah Alam, which every route uses:" */
export function profileTitle(parts: ProfilePart[], road: string): string {
  const km = parts.reduce((sum, p) => sum + p.km, 0).toFixed(1);
  return `Lanes along the first ${km} km of ${road || "the drive"}, which every route uses:`;
}

/**
 * What the lane strip shows, in words. The lesson (wide parts do not help
 * while a narrow one limits them) is said only when the data shows it: the
 * bottleneck is on the strip and every wider part has room to spare.
 */
export function profileSentence(parts: ProfilePart[]): string {
  if (parts.length === 0) return "";
  const lanes = parts.map((p) => p.lanes);
  const most = Math.max(...lanes);
  const fewest = Math.min(...lanes);
  if (most === fewest) return `${most} lane${most === 1 ? "" : "s"} all the way.`;
  const cut = parts.filter((p) => p.cut);
  const cutLanes = Math.min(...cut.map((p) => p.lanes));
  const wider = parts.filter((p) => !p.cut && p.lanes > cutLanes);
  if (cut.length && wider.length && wider.every((p) => !p.full)) {
    return `Up to ${most} lanes wide, but the ${cutLanes}-lane bottleneck decides how much gets through. The wider parts have room to spare, so more lanes there would add nothing.`;
  }
  return `Between ${fewest} and ${most} lanes wide.`;
}

/** The strip read out: "Lanes in order: 3 (bottleneck), 4, 5, 3 (nearly full), 5." */
export function profileLabel(parts: ProfilePart[]): string {
  const said = parts.map((p) => `${p.lanes}${p.cut ? " (bottleneck)" : p.full ? " (nearly full)" : ""}`);
  return `Lanes in order: ${said.join(", ")}.`;
}
