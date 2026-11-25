// Deep links (src/links.ts). Run with `npm test`.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dropSlug, hashFor, parseHash } from "../src/links.ts";

const place = (over = {}) => ({ dir: "inbound", view: "summary", solved: false, drop: null, ...over });

test("dropSlug turns a lane drop key into a link-safe name", () => {
  const cases = [
    ["node:33987957", "n33987957"],
    ["way:148018437>1116586907", "w148018437-1116586907"],
    ["way:25774683", "w25774683"],
  ];
  for (const [key, want] of cases) assert.equal(dropSlug(key), want);
});

test("parseHash", () => {
  const cases = [
    ["", place()],
    ["#", place()],
    ["#inbound", place()],
    ["#outbound", place({ dir: "outbound" })],
    ["#inbound/result", place({ solved: true })],
    ["#outbound/result", place({ dir: "outbound", solved: true })],
    ["#outbound/lane-drops", place({ dir: "outbound", view: "list" })],
    ["#inbound/how", place({ view: "how" })],
    ["#outbound/drop-n33987957", place({ dir: "outbound", drop: "n33987957" })],
    ["#inbound/drop-w148018437-1116586907", place({ drop: "w148018437-1116586907" })],
    ["#inbound/drop-21", place()],
    ["#inbound/drop-<script>", place()],
    ["#nonsense", place()],
    ["#outbound/nonsense", place({ dir: "outbound" })],
  ];
  for (const [hash, want] of cases) assert.deepEqual(parseHash(hash), want, hash);
});

test("links shared before the direction switch still open", () => {
  const cases = [
    ["#result", place({ solved: true })],
    ["#lane-drops", place({ view: "list" })],
    ["#how", place({ view: "how" })],
    ["#drop-n33987957", place({ drop: "n33987957" })],
  ];
  for (const [hash, want] of cases) assert.deepEqual(parseHash(hash), want, hash);
});

test("every link reads back as what it was made from", () => {
  for (const dir of ["inbound", "outbound"]) {
    const places = [
      place({ dir }),
      place({ dir, solved: true }),
      place({ dir, view: "list" }),
      place({ dir, view: "how" }),
      place({ dir, drop: "n1" }),
    ];
    for (const p of places) assert.deepEqual(parseHash(hashFor(p)), p);
  }
  assert.equal(hashFor(place()), "");
});

test("real corridor: every lane drop gets its own link", () => {
  const { drops } = JSON.parse(readFileSync(new URL("../public/data/kemuning-pj/lane_drops.json", import.meta.url), "utf8"));
  const slugs = drops.map((d) => dropSlug(d.key));
  assert.equal(new Set(slugs).size, drops.length);
  for (const s of slugs) assert.deepEqual(parseHash(hashFor(place({ drop: s }))), place({ drop: s }));
});
