// Deep links (docs/design/project/DESIGN_BRIEF.md §11): the address bar always names what the
// column is showing, so a link can be shared straight to a finding.
//   (none)            landing
//   #result           the result of the analysis, without replaying it
//   #lane-drops       the list
//   #how              How this works
//   #drop-n33987957   one lane drop
// Pure: no DOM, so `npm test` can check it.

export type Target =
  | { kind: "home" }
  | { kind: "result" }
  | { kind: "list" }
  | { kind: "how" }
  | { kind: "drop"; slug: string };

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

/** What a link's "#…" part asks for. Anything unknown is the landing view. */
export function parseHash(hash: string): Target {
  const h = hash.replace(/^#/, "");
  if (h === "result") return { kind: "result" };
  if (h === "lane-drops") return { kind: "list" };
  if (h === "how") return { kind: "how" };
  const drop = /^drop-([nw][0-9-]+)$/.exec(h);
  if (drop) return { kind: "drop", slug: drop[1]! };
  return { kind: "home" };
}

/** The "#…" part for a target; empty for the landing view. */
export function hashFor(t: Target): string {
  switch (t.kind) {
    case "result":
      return "#result";
    case "list":
      return "#lane-drops";
    case "how":
      return "#how";
    case "drop":
      return `#drop-${t.slug}`;
    default:
      return "";
  }
}
