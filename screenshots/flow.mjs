// Walks the site's user flow and saves a JPG per screen. The screens and their
// IDs (B5, E3, …) are listed in docs/USERFLOW.md; keep the two in step.
// Exits non-zero if the page threw an error, a check failed (wrong address,
// wrong state, focus in the wrong place) or a required screen was not reached.
// Usage: CHROME_BIN=... node flow.mjs <baseUrl> <outDir>   (needs playwright; `make shots` sets it up)
// Defaults: http://localhost:5173 (the "web" server in .claude/launch.json), ./flow
//
// Passes, each in its own folder under <outDir>:
//   desktop, mobile, mobile-short   the full walk (USERFLOW.md §4, steps 2–8)
//   links-desktop, links-mobile     deep links, one fresh page each (§3 I)
//   failure-desktop, failure-mobile data that does not load (§3 A)
//   keyboard                        focus and Escape, desktop (§3 J)
//   small-phone, tablet, desktop-short, phone-landscape,
//   reduced-desktop, reduced-mobile the short walk at the other viewports (§2)
//
// Finding things to tap: markers and roads are drawn on the map canvas, so they
// are not in the page. Confirmed jams and the bottleneck have HTML labels 14 px
// beside them (pin() in web/src/main.ts), so taps aim there. Everything else is
// found by sweeping the visible map with mouse moves and keeping the points
// where the map shows the pointer cursor (main.ts sets it over a marker or a
// road), then tapping those until a card of the wanted kind opens.
// On phones the sheet floats over the bottom of the map (decision 0017), so
// only the part of the map still showing counts.
import { chromium } from "playwright";
import { mkdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const [base = "http://localhost:5173", out = "./flow"] = process.argv.slice(2);
const url = `${base}/`;
const here = dirname(fileURLToPath(import.meta.url));
const TAG_GAP = 14; // px between a map label and its feature (main.ts, pin offsets)
const PHONE = "(max-width: 760px) and (orientation: portrait), (max-width: 560px)"; // main.ts
const ROUTES_DONE = "No route has room left, so this is the most."; // copy.ts
const LOAD_ERROR = "jalanflow: road data did not load"; // main.ts throws it on purpose
const SWEEP_STEP = 10; // px between probe points
const SWEEP_TRIES = 70; // taps per sweep before giving up on a kind

// Lane drops for the deep links, from the published data so they never go stale.
const drops = JSON.parse(readFileSync(join(here, "../web/public/data/kemuning-pj/lane_drops.json"), "utf8")).drops;
const slug = (key) => { const [t, rest = ""] = key.split(":"); return `${t === "node" ? "n" : "w"}${rest.replace(/[^0-9]+/g, "-")}`; }; // links.ts dropSlug
const pick = (f) => { const d = drops.find(f); return d ? slug(d.key) : null; };
const LINKED = {
  jamNode: pick((d) => d.confirmed && d.key.startsWith("node:")),
  jamWay: pick((d) => d.confirmed && d.key.startsWith("way:")),
  merge: pick((d) => d.kind === "merge" && !d.confirmed && !d.lanes_inferred),
  narrowing: pick((d) => d.kind !== "merge" && !d.confirmed && !d.lanes_inferred),
  guessed: pick((d) => d.lanes_inferred && !d.confirmed),
};

let failed = false;
const report = [];
const browsers = new Set(); // closed by the pass, or after it if it stopped

/** One browser page for one pass, with the helpers every pass uses. */
async function session(name, viewport, { touch = false, reduced = false, expectLoadError = false } = {}) {
  const dir = join(out, name);
  mkdirSync(dir, { recursive: true });
  const browser = await chromium.launch({ executablePath: process.env.CHROME_BIN });
  browsers.add(browser);
  const ctx = await browser.newContext({
    viewport, deviceScaleFactor: 2, isMobile: touch, hasTouch: touch, colorScheme: "dark",
    reducedMotion: reduced ? "reduce" : "no-preference",
  });
  const page = await ctx.newPage();
  const errors = [];
  const wrong = []; // checks that failed
  const missed = []; // screens not reached
  page.on("pageerror", (e) => { if (!(expectLoadError && e.message.includes(LOAD_ERROR))) errors.push(e.message); });
  let n = 0;

  const s = {
    page, name,
    settle: async (ms = 1200) => { await page.waitForLoadState("networkidle").catch(() => {}); await page.waitForTimeout(ms); },
    shot: async (label) => {
      n++;
      const file = join(dir, `${String(n).padStart(2, "0")}-${label}.jpg`);
      await page.screenshot({ path: file, type: "jpeg", quality: 88 });
      console.log(file);
    },
    click: (sel) => page.locator(sel).first().click(),
    body: (attr) => page.getAttribute("body", `data-${attr}`),
    hash: () => page.evaluate(() => window.location.hash),
    phone: () => page.evaluate((q) => window.matchMedia(q).matches, PHONE),
    check(ok, what) { if (!ok) wrong.push(what); return ok; },
    async expectHash(want, where) {
      const got = await s.hash();
      s.check(got === want, `${where}: address ${JSON.stringify(got)}, expected ${JSON.stringify(want)}`);
    },
    async expectBody(want, where) {
      for (const [k, v] of Object.entries(want)) {
        const got = await s.body(k);
        s.check(got === v, `${where}: data-${k}=${got}, expected ${v}`);
      }
    },
    miss: (what) => missed.push(what),
    async load(hash = "") {
      await page.goto(url + hash);
      await page.waitForSelector("body[data-load=ready]");
      await s.settle(2500);
    },
    async done() {
      if (missed.length) report.push(`${name}: not reached: ${missed.join(", ")}`);
      if (wrong.length) { report.push(`${name}: checks failed:\n  - ${wrong.join("\n  - ")}`); failed = true; }
      if (errors.length) { report.push(`${name}: page errors: ${errors.join(" | ")}`); failed = true; }
      browsers.delete(browser);
      await browser.close();
    },
  };

  /** Phones: open or close the sheet's findings. A no-op on the desktop layout. */
  s.setMore = async (want) => {
    if (!(await s.phone())) return;
    if ((await s.body("more")) !== want) { await s.click("#more-btn"); await s.settle(500); }
  };
  s.cardOpen = async () => (await page.locator(".card").count()) > 0;
  s.closeCard = async () => {
    const close = page.locator("#peek .card button", { hasText: "Close" });
    if (await close.count()) await close.click();
    else if (await s.cardOpen()) await page.keyboard.press("Escape");
    await s.settle(500);
  };
  /** Back to the summary from a list, detail or How this works. */
  s.back = async () => {
    const v = await s.body("view");
    if (v !== "summary") await s.click(`[data-v=${v}] [data-back]`);
    await s.settle(900);
  };
  /** What the open card says it is: jam, lane-drop, lane-drop-guessed, road, bottleneck, nearly-full, room-to-spare. */
  s.cardKind = () => page.evaluate(() => {
    const k = document.querySelector(".card .card-kind");
    if (!k) return null;
    const text = k.querySelector("b")?.textContent ?? "";
    const hollow = k.querySelector("i")?.classList.contains("hollow");
    return { "Confirmed jam": "jam", "Lane drop": hollow ? "lane-drop-guessed" : "lane-drop", Road: "road",
      Bottleneck: "bottleneck", "Nearly full": "nearly-full", "Room to spare": "room-to-spare" }[text] ?? text;
  });

  /** The part of the map still showing: on phones, above the floating sheet. */
  s.shownMap = () => page.evaluate(() => {
    const m = document.getElementById("map").getBoundingClientRect();
    const p = document.getElementById("panel");
    const bottom = getComputedStyle(p).position === "fixed" ? Math.min(m.bottom, p.getBoundingClientRect().top) : m.bottom;
    return { left: m.left, right: m.right, top: m.top, bottom };
  });
  /** Feature points next to every visible map label with this text. */
  s.nearTags = async (text) => {
    const shown = await s.shownMap();
    const boxes = await page.locator("#map .tag", { hasText: text }).evaluateAll((els) =>
      els.map((e) => { const r = e.getBoundingClientRect(); return { l: r.left, r: r.right, y: r.top + r.height / 2 }; }));
    // A label may sit either side of its feature (the bottleneck's flips east or west).
    return boxes.flatMap((b) => [{ x: b.l - TAG_GAP, y: b.y }, { x: b.r + TAG_GAP, y: b.y }])
      .filter((p) => p.x > shown.left + 4 && p.x < shown.right - 4 && p.y > shown.top + 4 && p.y < shown.bottom - 4);
  };
  /** Tap each point until one opens a card; note the screen as missed otherwise. */
  s.openCardAt = async (points, what) => {
    for (const p of points) {
      await page.mouse.click(p.x, p.y);
      await s.settle(700);
      if (await s.cardOpen()) return true;
    }
    s.miss(what);
    return false;
  };
  /**
   * Points on the visible map where the cursor turns to a pointer: a marker or
   * a road. Synthetic mouse moves on the canvas, in one go, so it is quick.
   * Only points where the canvas is on top (not a label, control or card).
   */
  s.probe = async () => {
    const shown = await s.shownMap();
    const hits = await page.evaluate(({ shown, step }) => {
      const canvas = document.querySelector("#map canvas");
      const found = [];
      for (let y = shown.top + 12; y < shown.bottom - 24; y += step) {
        for (let x = shown.left + 12; x < shown.right - 12; x += step) {
          if (document.elementFromPoint(x, y) !== canvas) continue;
          canvas.dispatchEvent(new MouseEvent("mousemove", { clientX: x, clientY: y, bubbles: true }));
          if (canvas.style.cursor === "pointer") found.push({ x, y });
        }
      }
      canvas.dispatchEvent(new MouseEvent("mousemove", { clientX: shown.left + 2, clientY: shown.top + 2, bubbles: true }));
      return found;
    }, { shown, step: SWEEP_STEP });
    await page.waitForTimeout(400); // a hover preview closes on its own
    if (await s.cardOpen()) await page.keyboard.press("Escape");
    // One point per 24 px: a road is a run of hits.
    const kept = [];
    for (const h of hits) if (!kept.some((k) => Math.hypot(k.x - h.x, k.y - h.y) < 24)) kept.push(h);
    return kept;
  };
  /** Where the map's camera is, from a marker that moves with it. */
  const camera = () => page.evaluate(() => {
    const m = document.querySelector("#map .maplibregl-marker")?.getBoundingClientRect();
    return m ? `${Math.round(m.left)},${Math.round(m.top)}` : "";
  });
  /**
   * Tap probed points until a card of each wanted kind has opened, calling
   * onFound(kind, point) with that card open. Probes again if the camera moved.
   * Returns the point where each kind was found.
   */
  s.sweep = async (wanted, onFound) => {
    const left = new Set(wanted);
    const at = {};
    let tries = 0;
    while (left.size && tries < SWEEP_TRIES) {
      const points = await s.probe();
      if (!points.length) break;
      const cam = await camera();
      let moved = false;
      for (const p of points) {
        if (!left.size || tries >= SWEEP_TRIES) break;
        tries++;
        await page.mouse.click(p.x, p.y);
        await page.waitForTimeout(350);
        const kind = await s.cardKind();
        if (kind && left.has(kind)) {
          left.delete(kind);
          at[kind] = p;
          await onFound(kind, p);
        }
        if (await s.cardOpen()) await s.closeCard();
        if ((await camera()) !== cam) { moved = true; break; }
      }
      if (!moved) break;
    }
    for (const k of left) s.miss(`card: ${k}`);
    return at;
  };
  /** Drag the phone sheet's grabber by dy px (negative is up). hold: shoot mid-drag under this name. */
  s.dragSheet = async (dy, hold) => {
    const g = await page.locator("#grab").boundingBox();
    const x = g.x + g.width / 2, y = g.y + g.height / 2;
    await page.mouse.move(x, y);
    await page.mouse.down();
    for (let i = 1; i <= 8; i++) await page.mouse.move(x, y + (dy * i) / 8);
    if (hold) {
      await page.waitForTimeout(100);
      s.check((await s.body("drag")) === "on", "grabber held: data-drag is not on");
      await s.shot(hold);
    }
    await page.mouse.up();
    await s.settle(700);
  };
  s.dir = async () => page.locator('.seg button[aria-pressed="true"]').getAttribute("data-dir");
  s.setDir = async (d) => {
    if ((await s.dir()) !== d) { await s.click(`.seg button[data-dir="${d}"]`); await s.settle(1500); }
  };
  /** Run the analysis from the start screen and wait for the result. */
  s.runToResult = async () => {
    await s.setMore("closed");
    await s.click("#run-btn");
    await page.waitForSelector("body[data-state=result]", { state: "attached", timeout: 60000 });
    await s.settle(1500);
  };
  return s;
}

// ---------- the full walk ----------

async function fullWalk(name, viewport, touch) {
  const s = await session(name, viewport, { touch });
  const { page } = s;
  const isPhone = async () => s.phone();
  await s.load();
  const phone = await isPhone();

  // B1–B3: the start, both directions.
  await s.shot("home-to-petaling-jaya");
  await s.expectHash("", "home");
  await s.expectBody({ state: "home", view: "summary" }, "home");
  if (phone) {
    await s.setMore("open");
    await s.shot("home-more-detail-open");
    await s.setMore("closed");
    // D7: a tap on the grabber does what More detail does.
    await s.click("#grab");
    await s.settle(600);
    s.check((await s.body("more")) === "open", "grabber tap did not open the sheet");
    await s.shot("home-grabber-tapped-open");
    await s.setMore("closed");
  }
  await s.setDir("outbound");
  await s.shot("home-to-kota-kemuning");
  await s.expectHash("#outbound", "home, outbound");
  await s.setDir("inbound");

  // B4: a confirmed jam before the analysis: lanes only, nothing computed yet.
  if (await s.openCardAt(await s.nearTags("Confirmed jam"), "home-jam-card")) await s.shot("home-jam-card");
  await s.closeCard();

  // B5–B9: the other cards before the analysis, and their details.
  const homeAt = await s.sweep(["lane-drop", "lane-drop-guessed", "road"], async (kind) => {
    await s.shot(`home-card-${kind}`);
    if (kind === "lane-drop-guessed") return;
    await page.locator(".card button", { hasText: "View details" }).click();
    await s.settle(1500);
    await s.shot(`home-detail-${kind}`);
    if (kind === "road") await s.expectHash("", "road detail before the analysis");
    else s.check((await s.hash()).startsWith("#inbound/drop-"), "lane drop detail: address has no drop");
    await s.back();
  });
  // B10: pointing at a marker previews it (desktop, with a mouse).
  if (!touch && homeAt["lane-drop"]) {
    await page.mouse.move(homeAt["lane-drop"].x, homeAt["lane-drop"].y);
    await page.waitForTimeout(500);
    if (s.check(await s.cardOpen(), "hover on a lane drop opened no preview")) await s.shot("home-hover-preview");
    await page.mouse.move(5, 5);
    await page.waitForTimeout(600);
    s.check(!(await s.cardOpen()), "hover preview stayed after the pointer left");
  }

  // G5, H4: the list and How this works before the analysis.
  await s.setMore("open");
  await s.click("#open-list");
  await s.settle(1500);
  await s.shot("home-lane-drops-list");
  await s.expectHash("#inbound/lane-drops", "list");
  await s.back();
  await s.setMore("open");
  await s.click('[data-open="how"]');
  await s.settle(800);
  await s.shot("home-how-this-works");
  await s.expectHash("#inbound/how", "how");
  await s.back();
  await s.setMore("closed");

  // C2, C7: the search starts, then the direction changes under it.
  await s.click("#run-btn");
  await page.waitForSelector("body[data-state=running]", { state: "attached", timeout: 5000 });
  await page.waitForTimeout(150);
  await s.shot("running-first-route");
  await s.click('.seg button[data-dir="outbound"]');
  await s.settle(1500);
  await s.expectBody({ state: "home" }, "direction switched while running");
  await s.shot("running-direction-switched-home");
  await s.setDir("inbound");

  // C1, C8, C3: the search for real, to the end.
  await s.click("#run-btn");
  await page.waitForSelector("body[data-state=running]", { state: "attached", timeout: 5000 });
  await page.waitForTimeout(1800);
  await s.shot("running-analysis");
  const jam = await s.nearTags("Confirmed jam");
  if (jam.length) {
    await page.mouse.click(jam[0].x, jam[0].y);
    await page.waitForTimeout(300);
    s.check(!(await s.cardOpen()), "a map tap while running opened a card");
  }
  s.check(await page.evaluate(() => document.activeElement?.id === "skip-btn"), "focus is not on Skip while running");
  const ended = await page.waitForFunction((t) => document.getElementById("run-caption")?.textContent === t
    && document.body.dataset.state === "running", ROUTES_DONE, { timeout: 30000, polling: 50 }).then(() => true, () => false);
  if (ended) await s.shot("running-all-routes-done");
  else s.miss("running-all-routes-done");
  await page.waitForSelector("body[data-state=result]", { state: "attached", timeout: 60000 });
  await s.settle(1500);

  // D1–D7: the result.
  await s.shot("result");
  await s.expectHash("#inbound/result", "result");
  if (phone) {
    await s.setMore("open");
    await s.shot("result-more-detail-open");
    await page.evaluate(() => { document.getElementById("panel").scrollTop = 400; });
    await s.settle(300);
    await s.shot("result-more-detail-scrolled");
    await page.evaluate(() => { document.getElementById("panel").scrollTop = 0; });
    await s.dragSheet(160, "result-dragging-down");
    await s.shot("result-dragged-closed");
    await s.dragSheet(-120);
    await s.shot("result-dragged-open");
  } else {
    // D10: the lower panel on the desktop.
    await page.evaluate(() => { document.getElementById("panel").scrollTop = 10000; });
    await s.settle(300);
    await s.shot("result-panel-scrolled");
    await page.evaluate(() => { document.getElementById("panel").scrollTop = 0; });
  }

  // D8, D9: flow off and on again. Under More detail on phones, open here.
  await s.click("#flow-btn");
  await s.settle(600);
  await s.shot("result-flow-hidden");
  await s.click("#flow-btn");
  await s.settle(600);
  s.check((await page.textContent("#flow-btn")) === "Hide flow", "flow button did not come back to Hide flow");
  await s.shot("result-flow-shown-again");
  await s.setMore("closed");

  // F1: the bottleneck's detail, the map zoomed to it.
  await s.click("#limit-btn");
  await s.settle(2000);
  await s.shot("bottleneck-detail");
  await s.expectBody({ view: "detail" }, "bottleneck detail");
  await s.expectHash("#inbound/result", "road detail");
  if (phone) {
    await s.dragSheet(160); // F9: down goes back, like Back
    s.check((await s.body("view")) === "summary", "dragging down from a detail did not go back");
    await s.shot("detail-dragged-back");
  } else await s.back();

  // E1, F2: its card on the map, then View details from the card.
  if (await s.openCardAt(await s.nearTags("Bottleneck"), "bottleneck-road-card")) {
    await s.shot("bottleneck-road-card");
    await page.locator(".card button", { hasText: "View details" }).click();
    await s.settle(1200);
    await s.shot("road-card-view-details");
    await s.back();
  }

  // E2, E6, E11, E12: a confirmed jam's card, drawing, closed with Escape, then replaced.
  const jams = await s.nearTags("Confirmed jam");
  if (await s.openCardAt(jams.slice(0, 1), "jam-card-drawing")) {
    await s.shot("jam-card-drawing"); // the lane diagram mid-draw
    await s.settle(900);
    await s.shot("jam-card");
    await page.keyboard.press("Escape");
    await s.settle(400);
    s.check(!(await s.cardOpen()), "Escape did not close the card");
    s.check((await s.body("view")) === "summary", "Escape on a card also left the summary");
  } else if (await s.openCardAt(jams.slice(1), "jam-card")) {
    await s.shot("jam-card");
  }
  await s.closeCard();

  // E3–E5, F3, F4: the other cards after the analysis.
  const resultAt = await s.sweep(["nearly-full", "room-to-spare", "lane-drop", "lane-drop-guessed"], async (kind) => {
    await s.shot(`result-card-${kind}`);
    if (kind !== "nearly-full" && kind !== "room-to-spare") return;
    await page.locator(".card button", { hasText: "View details" }).click();
    await s.settle(1500);
    await s.shot(`result-detail-${kind}`);
    await s.back();
  });
  const first = jams[0], second = resultAt["lane-drop"] ?? resultAt["lane-drop-guessed"];
  if (first && second) {
    await page.mouse.click(first.x, first.y);
    await s.settle(600);
    await page.mouse.click(second.x, second.y);
    await s.settle(600);
    const kind = await s.cardKind();
    s.check((await page.locator(".card").count()) === 1 && kind !== "jam", `second tap did not replace the card (${kind})`);
    await s.shot("card-replaced-by-next-tap");
    await s.closeCard();
  }

  if (phone) {
    // E7–E9: with the sheet open the map showing is too small for a card, so the
    // quick look takes the sheet's place; dragging down or Close takes it away.
    await s.setMore("open");
    if (await s.openCardAt(await s.nearTags("Confirmed jam"), "jam-card-in-sheet")) {
      await s.shot("jam-card-in-sheet");
      await s.dragSheet(160);
      await s.shot("jam-card-dragged-away");
    }
    await s.setMore("open");
    if (await s.openCardAt(await s.nearTags("Confirmed jam"), "jam-card-in-sheet-close")) {
      await page.locator("#peek .card button", { hasText: "Close" }).click();
      await s.settle(500);
      s.check(!(await s.cardOpen()), "Close in the sheet did not close the card");
      await s.shot("jam-card-in-sheet-closed");
    }
    await s.setMore("closed");
  } else {
    // E10: a window made short moves an open card into the panel.
    if (await s.openCardAt(await s.nearTags("Bottleneck"), "card-moved-into-panel")) {
      await page.setViewportSize({ width: viewport.width, height: 440 });
      await s.settle(900);
      s.check((await s.body("peek")) === "on", "short window: the card did not move into the panel");
      await s.shot("card-moved-into-panel");
      await s.closeCard();
      await page.setViewportSize(viewport);
      await s.settle(1200);
    }
  }

  // G1–G4, F5: the lane drops list, then one drop.
  await s.setMore("open");
  await s.click("#open-list");
  await s.settle(1500);
  await s.shot("lane-drops-list");
  await s.expectHash("#inbound/lane-drops", "list after the analysis");
  if (phone) {
    await page.evaluate(() => { document.getElementById("panel").scrollTop = 900; });
    await s.settle(300);
    await s.shot("lane-drops-list-scrolled"); // Back stays at the top of the sheet
    await page.evaluate(() => { document.getElementById("panel").scrollTop = 0; });
  } else {
    await page.locator("#list button").first().hover();
    await s.settle(400);
    await s.shot("lane-drops-row-hovered");
    // A marker under the pointer lights its row.
    let lit = false;
    for (const p of (await s.probe()).slice(0, 40)) {
      await page.mouse.move(p.x, p.y);
      await page.waitForTimeout(80);
      if (await page.locator("#list button.hot").count()) { lit = true; break; }
    }
    if (lit) await s.shot("lane-drops-marker-hovered");
    else s.miss("lane-drops-marker-hovered");
    await page.mouse.move(5, 5);
  }
  const opener = page.locator("#list button").first();
  await opener.click();
  await s.settle(2000);
  await s.shot("lane-drop-detail");
  s.check((await s.hash()).startsWith("#inbound/drop-"), "lane drop detail: address has no drop");
  await s.back();

  // H1–H3: How this works.
  await s.setMore("open");
  await s.click('[data-open="how"]');
  await s.settle(800);
  await s.shot("how-this-works");
  await page.evaluate(() => {
    const p = document.getElementById("panel");
    if (p) p.scrollTop = p.scrollHeight;
    document.querySelector("[data-v=how]")?.scrollIntoView({ block: "end" });
  });
  await s.settle(400);
  await s.shot("how-this-works-end");
  if (phone) {
    await s.dragSheet(160); // down goes back, like the Back button
    await s.shot("how-dragged-back");
  } else await s.back();

  // D11, C6: replay, then skip.
  await s.setMore("open");
  await s.click("#replay-btn");
  await page.waitForSelector("body[data-state=running]", { state: "attached", timeout: 5000 });
  await page.waitForTimeout(1000);
  await s.shot("replay-running");
  await s.click("#skip-btn");
  await s.settle(800);
  await s.expectBody({ state: "result" }, "skip");
  await s.shot("skipped-to-result");

  // D2, D12, C4, C5, G6, F10: the other direction.
  await s.setMore("closed");
  await s.setDir("outbound");
  await s.settle(500);
  await s.shot("result-to-kota-kemuning");
  await s.expectHash("#outbound/result", "result, outbound");
  await s.setMore("open");
  if (!phone) await page.evaluate(() => { document.getElementById("panel").scrollTop = 10000; });
  await s.settle(400);
  await s.shot("result-to-kota-kemuning-detail");
  await page.evaluate(() => { document.getElementById("panel").scrollTop = 0; });
  await s.click("#replay-btn");
  await page.waitForSelector("body[data-state=running]", { state: "attached", timeout: 5000 });
  await page.waitForTimeout(1800);
  await s.shot("running-to-kota-kemuning");
  const takesBack = await page.waitForFunction(() => /by moving part of an earlier route/.test(document.getElementById("run-caption")?.textContent ?? ""),
    null, { timeout: 20000, polling: 50 }).then(() => true, () => false);
  if (takesBack) await s.shot("running-route-moves-earlier-flow");
  else s.miss("running-route-moves-earlier-flow (none in this direction's replay?)");
  await page.waitForSelector("body[data-state=result]", { state: "attached", timeout: 60000 });
  await s.settle(1200);
  await s.setMore("open");
  await s.click("#open-list");
  await s.settle(1500);
  await s.shot("lane-drops-list-to-kota-kemuning");
  await s.expectHash("#outbound/lane-drops", "list, outbound");
  await s.back();
  await s.setMore("closed");
  await s.click("#limit-btn");
  await s.settle(1500);
  await s.click('.seg button[data-dir="inbound"]');
  await s.settle(1500);
  s.check((await s.body("view")) === "summary", "switching direction on a detail did not go back to the summary");
  await s.shot("detail-direction-switched");

  // K1, K2: closer in, the other markers appear, then lanes are drawn one by one.
  await s.setMore("closed");
  const zoomIn = page.locator(".maplibregl-ctrl-zoom-in");
  for (let i = 0; i < 2; i++) { await zoomIn.click(); await page.waitForTimeout(400); }
  await s.settle(1200);
  await s.shot("map-zoomed-in");
  for (let i = 0; i < 2; i++) { await zoomIn.click(); await page.waitForTimeout(400); }
  await s.settle(1500);
  await s.shot("map-zoomed-to-lanes");

  await s.done();
}

// ---------- the short walk, for the other viewports ----------

async function shortWalk(name, viewport, { touch = false, reduced = false } = {}) {
  const s = await session(name, viewport, { touch, reduced });
  const { page } = s;
  await s.load();
  const phone = await s.phone();
  await s.shot("home");
  if (phone) {
    await s.setMore("open");
    await s.shot("home-more-detail-open");
    await s.setMore("closed");
  }
  if (await s.openCardAt(await s.nearTags("Confirmed jam"), "jam-card")) {
    await page.waitForTimeout(150);
    await s.shot(`jam-card${(await s.body("peek")) === "on" ? "-in-panel" : ""}`);
  }
  await s.closeCard();

  await s.click("#run-btn");
  if (reduced) {
    // Reduced motion: straight to the result, no running screen.
    await page.waitForTimeout(100);
    await s.expectBody({ state: "result" }, "reduced motion: Run");
  } else {
    await page.waitForSelector("body[data-state=running]", { state: "attached", timeout: 5000 });
    await page.waitForTimeout(1800);
    await s.shot("running-analysis");
  }
  await page.waitForSelector("body[data-state=result]", { state: "attached", timeout: 60000 });
  await s.settle(1500);
  await s.shot("result");
  if (phone) {
    await s.setMore("open");
    await s.shot("result-more-detail-open");
    await s.setMore("closed");
  }
  if (await s.openCardAt(await s.nearTags("Bottleneck"), "bottleneck-road-card")) {
    await s.shot(`bottleneck-road-card${(await s.body("peek")) === "on" ? "-in-panel" : ""}`);
  }
  await s.closeCard();
  await s.click("#limit-btn");
  await s.settle(2000);
  await s.shot("bottleneck-detail");
  await s.back();
  await s.setMore("open");
  await s.click("#open-list");
  await s.settle(1500);
  await s.shot("lane-drops-list");
  await s.done();
}

// ---------- deep links: a fresh page each ----------

async function linksPass(name, viewport, touch) {
  const s = await session(name, viewport, { touch });
  const { page } = s;
  const cases = [
    ["I1", "#outbound", "home-to-kota-kemuning", { state: "home", view: "summary" }, "outbound", "#outbound"],
    ["I2", "#inbound/result", "result", { state: "result", view: "summary" }, "inbound", "#inbound/result"],
    ["I3", "#outbound/result", "result-to-kota-kemuning", { state: "result", view: "summary" }, "outbound", "#outbound/result"],
    ["I4", "#outbound/lane-drops", "list-to-kota-kemuning", { view: "list" }, "outbound", "#outbound/lane-drops"],
    ["I5", "#inbound/how", "how-this-works", { view: "how" }, "inbound", "#inbound/how"],
    ["I8", "#result", "old-link-result", { state: "result", view: "summary" }, "inbound", "#inbound/result"],
    ["I8", "#lane-drops", "old-link-lane-drops", { view: "list" }, "inbound", "#inbound/lane-drops"],
    ["I8", "#how", "old-link-how", { view: "how" }, "inbound", "#inbound/how"],
    ["I9", "#nonsense", "unknown-link-home", { state: "home", view: "summary" }, "inbound", ""],
    ["I9", "#inbound/drop-n0", "unknown-drop-home", { state: "home", view: "summary" }, "inbound", ""],
  ];
  // F6–F8 and I6, I7: one lane drop of each sort, straight from a link.
  for (const [what, sl] of Object.entries(LINKED)) {
    if (!sl) { s.miss(`a lane drop that is ${what}`); continue; }
    cases.push(["I6", `#inbound/drop-${sl}`, `drop-${what}`, { view: "detail" }, "inbound", `#inbound/drop-${sl}`]);
  }
  if (LINKED.jamNode) cases.push(["I8", `#drop-${LINKED.jamNode}`, "old-link-drop", { view: "detail" }, "inbound", `#inbound/drop-${LINKED.jamNode}`]);

  for (const [id, hash, label, want, dir, after] of cases) {
    await s.load(hash);
    await s.settle(800);
    await s.shot(label);
    await s.expectBody(want, `${id} ${hash}`);
    s.check((await s.dir()) === dir, `${id} ${hash}: direction ${await s.dir()}, expected ${dir}`);
    await s.expectHash(after, `${id} ${hash} (address after load)`);
    if (want.view === "detail") {
      s.check((await page.locator("#detail-title").textContent())?.trim().length > 0, `${id} ${hash}: detail has no title`);
    }
  }

  // I10: a link followed on a loaded page, then Back in the browser.
  await s.load("#inbound/result");
  await page.evaluate(() => { window.location.hash = "#outbound/how"; });
  await s.settle(1200);
  await s.expectBody({ view: "how" }, "I10 hashchange to #outbound/how");
  s.check((await s.dir()) === "outbound", "I10 hashchange: direction did not change");
  await s.shot("hashchange-to-how-outbound");
  await page.goBack();
  await s.settle(1200);
  await s.expectBody({ view: "summary", state: "result" }, "I10 browser Back");
  s.check((await s.dir()) === "inbound", "I10 browser Back: direction did not come back");
  await s.shot("hashchange-back");
  await s.done();
}

// ---------- data that does not load ----------

async function failurePass(name, viewport, touch) {
  const s = await session(name, viewport, { touch, expectLoadError: true });
  const { page } = s;

  // A1: still loading.
  let release;
  const held = new Promise((r) => { release = r; });
  await page.route("**/edges.geojson", async (route) => { await held; await route.continue(); });
  await page.goto(url, { waitUntil: "commit" });
  await page.waitForTimeout(1500);
  await s.expectBody({ load: "loading" }, "A1 loading");
  await s.shot("loading");
  release();
  await page.waitForSelector("body[data-load=ready]");
  await page.unroute("**/edges.geojson");

  // A2: the road data fails.
  await page.route("**/edges.geojson", (route) => route.abort());
  await page.goto(url);
  await page.waitForSelector("body[data-load=failed]", { timeout: 10000 }).catch(() => {});
  await s.expectBody({ load: "failed" }, "A2 road data failed");
  s.check(await page.locator("#failed").isVisible(), "A2: the failure message is not showing");
  await s.shot("road-data-failed");
  await page.unroute("**/edges.geojson");

  // A3: only the lane drops fail.
  await page.route("**/lane_drops.json", (route) => route.abort());
  await s.load();
  s.check((await page.textContent("#counts"))?.includes("did not load"), "A3: counts do not say the lane drops did not load");
  s.check(await page.locator("#open-list").isHidden(), "A3: See all lane drops is still offered");
  await s.shot("lane-drops-failed");
  await page.unroute("**/lane_drops.json");

  // A4: the basemap fails; our roads carry the OSM credit.
  await page.route("https://tiles.openfreemap.org/**", (route) => route.abort());
  await s.load();
  const credit = await page.locator(".maplibregl-ctrl-attrib").textContent().catch(() => "");
  s.check(/OpenStreetMap/.test(credit ?? ""), "A4: no OpenStreetMap credit without the basemap");
  await s.shot("basemap-failed");
  await s.done();
}

// ---------- keyboard and focus (desktop) ----------

async function keyboardPass() {
  const s = await session("keyboard", { width: 1440, height: 900 });
  const { page } = s;
  const focused = () => page.evaluate(() => {
    const a = document.activeElement;
    return a ? `${a.tagName.toLowerCase()}${a.id ? `#${a.id}` : ""}${a.hasAttribute("data-back") ? "[data-back]" : ""}${a.classList.contains("skip") ? ".skip" : ""}` : "";
  });
  await s.load();
  // J1: the skip link shows on the first Tab.
  await page.keyboard.press("Tab");
  await page.waitForTimeout(300);
  s.check((await focused()) === "a.skip", `J1: first Tab focused ${await focused()}`);
  await s.shot("skip-link-focused");
  // J2, J3: a view focuses its Back; Back returns focus to what opened it.
  await page.locator('[data-open="how"]').focus();
  await page.keyboard.press("Enter");
  await s.settle(800);
  s.check((await focused()) === "button[data-back]", `J2: How this works focused ${await focused()}`);
  await s.shot("how-back-focused");
  await page.keyboard.press("Enter");
  await s.settle(800);
  s.check((await s.body("view")) === "summary", "J3: Enter on Back did not go back");
  s.check(await page.evaluate(() => document.activeElement?.getAttribute("data-open") === "how"), `J3: focus after Back is ${await focused()}`);
  await s.shot("back-focus-returned");
  // J5: Escape leaves a view.
  await page.locator("#open-list").focus();
  await page.keyboard.press("Enter");
  await s.settle(800);
  await page.keyboard.press("Escape");
  await s.settle(800);
  s.check((await s.body("view")) === "summary", "J5: Escape did not leave the list");
  // J4: Skip has focus while it runs.
  await page.locator("#run-btn").focus();
  await page.keyboard.press("Enter");
  await page.waitForSelector("body[data-state=running]", { state: "attached", timeout: 5000 });
  await page.waitForTimeout(500);
  s.check((await focused()) === "button#skip-btn", `J4: focus while running is ${await focused()}`);
  await s.shot("running-skip-focused");
  await page.keyboard.press("Enter");
  await s.settle(800);
  s.check((await s.body("state")) === "result", "J4: Enter on Skip did not show the result");
  await s.shot("result-after-skip");
  await s.done();
}

// ---------- run them ----------

const only = process.env.SHOTS_ONLY?.split(",").filter(Boolean); // e.g. SHOTS_ONLY=desktop,links-mobile
const want = (name) => !only || only.includes(name);
const passes = [
  ["desktop", () => fullWalk("desktop", { width: 1440, height: 900 }, false)],
  ["mobile", () => fullWalk("mobile", { width: 390, height: 844 }, true)],
  ["mobile-short", () => fullWalk("mobile-short", { width: 375, height: 667 }, true)],
  ["links-desktop", () => linksPass("links-desktop", { width: 1440, height: 900 }, false)],
  ["links-mobile", () => linksPass("links-mobile", { width: 390, height: 844 }, true)],
  ["failure-desktop", () => failurePass("failure-desktop", { width: 1440, height: 900 }, false)],
  ["failure-mobile", () => failurePass("failure-mobile", { width: 390, height: 844 }, true)],
  ["keyboard", () => keyboardPass()],
  ["small-phone", () => shortWalk("small-phone", { width: 320, height: 568 }, { touch: true })],
  ["tablet", () => shortWalk("tablet", { width: 768, height: 1024 }, { touch: true })],
  ["desktop-short", () => shortWalk("desktop-short", { width: 1280, height: 640 })],
  ["phone-landscape", () => shortWalk("phone-landscape", { width: 844, height: 390 }, { touch: true })],
  ["reduced-desktop", () => shortWalk("reduced-desktop", { width: 1440, height: 900 }, { reduced: true })],
  ["reduced-mobile", () => shortWalk("reduced-mobile", { width: 390, height: 844 }, { touch: true, reduced: true })],
];
for (const [name, run] of passes) {
  if (!want(name)) continue;
  try {
    await run();
  } catch (e) {
    report.push(`${name}: stopped: ${e.message.split("\n")[0]}`);
    failed = true;
  }
  for (const b of browsers) await b.close().catch(() => {});
  browsers.clear();
}
console.log(report.length ? `\n${report.join("\n")}` : "\nAll screens reached, all checks passed.");
if (failed) process.exitCode = 1;
