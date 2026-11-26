// Deep links (docs/design/project/DESIGN_BRIEF.md §11): the address bar names
// the direction and what the panel shows, so a link can be shared straight
// to a finding.
//   (none)                     the start, towards Petaling Jaya
//   #outbound                  the start, towards Kota Kemuning
//   #inbound/result            the result, without replaying the search
//   #outbound/lane-drops       the list
//   #inbound/how               How this works
//   #inbound/drop-n33987957    one lane drop
// Links from before the direction switch (#result, #lane-drops, #how,
// #drop-n33987957) still open, towards Petaling Jaya.
// Pure: no DOM, so `npm test` can check it.
import type { Direction } from "./types.ts";

export interface Place {
  dir: Direction;
  view: "summary" | "list" | "how";
  /** The link is to the result, not to the start. */
  solved: boolean;
  /** One lane drop's detail, by its link name (dropSlug); null for none. */
  drop: string | null;
}

/**
 * A lane drop's name in a link, from its key in lane_drops.json:
 * "node:33987957" → "n33987957", "way:148018437>1116586907" →
 * "w148018437-1116586907". The key is built from OpenStreetMap ids, so
 * unlike the rank it still points at the same place after the data is
 * refreshed.
 */
export function dropSlug(key: string): string {
  const [type, rest = ""] = key.split(":");
  return `${type === "node" ? "n" : "w"}${rest.replace(/[^0-9]+/g, "-")}`;
}

/** What a link's "#…" part asks for. Anything unknown is the start, towards Petaling Jaya. */
export function parseHash(hash: string): Place {
  const parts = hash.replace(/^#/, "").split("/");
  const named = parts[0] === "inbound" || parts[0] === "outbound";
  const v = (named ? parts[1] : parts[0]) ?? "";
  return {
    dir: parts[0] === "outbound" ? "outbound" : "inbound",
    view: v === "lane-drops" ? "list" : v === "how" ? "how" : "summary",
    solved: v === "result",
    drop: /^drop-([nw][0-9-]+)$/.exec(v)?.[1] ?? null,
  };
}

/** The "#…" part for a place; empty for the start towards Petaling Jaya. */
export function hashFor(p: Place): string {
  const tail = p.drop ? `/drop-${p.drop}` : p.view === "list" ? "/lane-drops" : p.view === "how" ? "/how" : p.solved ? "/result" : "";
  return p.dir === "inbound" && !tail ? "" : `#${p.dir}${tail}`;
}

/**
 * True when a link must stop the analysis that is running: it names the other
 * direction, and the routes being traced belong to the one shown.
 */
export function stopsRun(running: boolean, shown: Direction, next: Place): boolean {
  return running && next.dir !== shown;
}
