// Records short motion clips of the site as GIFs: screenshots in a loop,
// each stamped with the time it was taken, stitched by ffmpeg at real speed.
// Usage: from screenshots/: node motion.mjs <baseUrl> <outDir>   (needs ffmpeg; Playwright is in screenshots/package.json)
// MOTION_ONLY=card,sheet records only the clips whose names contain one of
// those words (the steps between still run, so each clip starts where it should).
// Exits non-zero if the page threw an error.
//
// Every motion on the site has a clip (docs/USERFLOW.md §6): the search
// replay, flow dots and cars in lanes, the direction switch, the camera moving
// between views, a map card fading in and its lane diagram drawing, a hover
// preview coming and going, and on phones the sheet opening by button and by
// drag, a card in the sheet, and dragging back from a detail.
import { chromium } from "playwright";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";

const [base = "http://localhost:5173", outArg = "./motion"] = process.argv.slice(2);
const out = resolve(outArg);
const url = `${base}/`;
const FPS_CAP = 20; // frames a second in the GIF
const GIF_WIDTH = 900; // px; GIFs get big fast
const TAG_GAP = 14; // px between a map label and its feature (main.ts, pin offsets)
const only = process.env.MOTION_ONLY?.split(",").filter(Boolean);
const errors = [];

/**
 * Grab frames until `done()` is true or `maxMs` passes, then write <name>.gif.
 * Chrome's screencast pushes a JPEG each time the page paints, with its time,
 * which keeps up with the map far better than calling screenshot in a loop.
 */
async function record(page, name, { maxMs = 4000, done = async () => false, tailMs = 0, width = GIF_WIDTH } = {}) {
  if (only && !only.some((w) => name.includes(w))) {
    // Not wanted this time: let the clip's own motion run out, unrecorded.
    const t0 = Date.now();
    while (Date.now() - t0 < maxMs && !(await done())) await page.waitForTimeout(100);
    return;
  }
  const dir = join(tmpdir(), `jf-${name}-${Date.now()}`);
  mkdirSync(dir, { recursive: true });
  const frames = [];
  const cdp = await page.context().newCDPSession(page);
  let first = null;
  cdp.on("Page.screencastFrame", ({ data, metadata, sessionId }) => {
    const ts = metadata.timestamp * 1000;
    if (first === null) first = ts;
    const file = join(dir, `${String(frames.length).padStart(4, "0")}.jpg`);
    writeFileSync(file, Buffer.from(data, "base64"));
    frames.push({ file, t: ts - first });
    cdp.send("Page.screencastFrameAck", { sessionId }).catch(() => {});
  });
  const t0 = Date.now();
  await cdp.send("Page.startScreencast", { format: "jpeg", quality: 90, everyNthFrame: 1 });
  let doneAt = null;
  while (Date.now() - t0 < maxMs) {
    if (doneAt === null && (await done())) doneAt = Date.now();
    if (doneAt !== null && Date.now() - doneAt >= tailMs) break;
    await page.waitForTimeout(100);
  }
  await cdp.send("Page.stopScreencast");
  await cdp.detach();
  // Each frame lasts until the next one was taken: real speed, uneven capture.
  const total = Math.max(frames.at(-1).t + 1000 / FPS_CAP, Date.now() - t0 - 0);
  const lines = frames.map((f, i) => {
    const next = i + 1 < frames.length ? frames[i + 1].t : total;
    return `file '${f.file}'\nduration ${((next - f.t) / 1000).toFixed(3)}`;
  });
  lines.push(`file '${frames.at(-1).file}'`); // concat needs the last frame twice
  const list = join(dir, "list.txt");
  writeFileSync(list, lines.join("\n"));
  const gif = join(out, `${name}.gif`);
  execFileSync("ffmpeg", [
    "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", list,
    "-vf", `fps=${FPS_CAP},scale=${width}:-1:flags=lanczos,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4`,
    "-loop", "0", gif,
  ]);
  rmSync(dir, { recursive: true, force: true });
  console.log(`${name}.gif  ${frames.length} frames over ${(total / 1000).toFixed(1)}s (${(frames.length / (total / 1000)).toFixed(1)} fps)`);
}

/**
 * Record while doing something: the gesture runs inside the clip, after a
 * short lead-in, and the clip runs on for tailMs after it ends so what it
 * caused (a sheet settling, a camera move) is in the clip. maxMs is a cap.
 */
async function during(page, name, opts, act, leadMs = 300) {
  let finished = false;
  const clip = record(page, name, { tailMs: 900, ...opts, maxMs: (opts.maxMs ?? 4000) + 6000, done: async () => finished });
  await page.waitForTimeout(leadMs);
  await act();
  finished = true;
  await clip;
}

async function open(viewport, isMobile) {
  const browser = await chromium.launch({ executablePath: process.env.CHROME_BIN });
  const ctx = await browser.newContext({ viewport, deviceScaleFactor: 1, isMobile, hasTouch: isMobile, colorScheme: "dark" });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto(url);
  await page.waitForSelector("body[data-load=ready]");
  await page.waitForLoadState("networkidle").catch(() => {});
  await page.waitForTimeout(2500);
  return { browser, page };
}

const isResult = (page) => async () => (await page.getAttribute("body", "data-state")) === "result";
const settle = async (page, ms = 800) => { await page.waitForLoadState("networkidle").catch(() => {}); await page.waitForTimeout(ms); };

/**
 * A point on a confirmed jam, from its "Confirmed jam" label (14 px to its
 * right), in the part of the map still showing (above the phone sheet).
 * `which` picks the jam: 0 the lower one on screen, 1 the other.
 */
async function jamPoint(page, which = 0) {
  return page.evaluate(({ gap, which }) => {
    const m = document.getElementById("map").getBoundingClientRect();
    const p = document.getElementById("panel");
    const bottom = getComputedStyle(p).position === "fixed" ? Math.min(m.bottom, p.getBoundingClientRect().top) : m.bottom;
    const pts = [...document.querySelectorAll("#map .tag")]
      .filter((t) => t.textContent === "Confirmed jam")
      .map((t) => { const r = t.getBoundingClientRect(); return { x: r.left - gap, y: r.top + r.height / 2 }; })
      .filter((q) => q.x > m.left + 4 && q.x < m.right - 4 && q.y > m.top + 4 && q.y < bottom - 4)
      .sort((a, b) => b.y - a.y);
    return pts[which] ?? pts[0] ?? null;
  }, { gap: TAG_GAP, which });
}

/** Drag the phone sheet's grabber by dy px (negative is up), slowly enough to see the sheet follow. */
async function dragSheet(page, dy, ms = 500) {
  const g = await page.locator("#grab").boundingBox();
  const x = g.x + g.width / 2, y = g.y + g.height / 2;
  const steps = 16;
  await page.mouse.move(x, y);
  await page.mouse.down();
  for (let i = 1; i <= steps; i++) {
    await page.mouse.move(x, y + (dy * i) / steps);
    await page.waitForTimeout(ms / steps);
  }
  await page.mouse.up();
}

const closeCard = async (page) => {
  if ((await page.locator(".card").count()) > 0) await page.keyboard.press("Escape");
  await page.waitForTimeout(400);
};

mkdirSync(out, { recursive: true });

// Desktop
{
  const { browser, page } = await open({ width: 1280, height: 800 }, false);

  // Before the analysis: a hover preview comes and goes, then a click keeps
  // the card and its lane diagram draws itself once.
  const jam = await jamPoint(page);
  if (jam) {
    await during(page, "desktop-card-hover", { maxMs: 2600 }, async () => {
      await page.mouse.move(jam.x - 120, jam.y - 60);
      await page.mouse.move(jam.x, jam.y, { steps: 12 });
      await page.waitForTimeout(1000);
      await page.mouse.move(jam.x - 160, jam.y - 120, { steps: 12 }); // across the map, so it sees the pointer leave
    });
    await page.waitForTimeout(500);
    await during(page, "desktop-card-open", { maxMs: 1800 }, () => page.mouse.click(jam.x, jam.y), 200);
    await closeCard(page);
  } else console.log("desktop-card-*: no confirmed jam on screen, skipped");

  await page.click("#run-btn");
  await record(page, "desktop-run-analysis", { maxMs: 40000, done: isResult(page), tailMs: 1500 });

  await page.waitForTimeout(800);
  await record(page, "desktop-flow-dots", { maxMs: 4000 });

  // The camera follows the view: the bottleneck, the whole drive, every lane drop.
  await during(page, "desktop-camera-views", { maxMs: 5200 }, async () => {
    await page.click("#limit-btn");
    await page.waitForTimeout(1300);
    await page.click("[data-v=detail] [data-back]");
    await page.waitForTimeout(1300);
    await page.click("#open-list");
    await page.waitForTimeout(1300);
    await page.click("[data-v=list] [data-back]");
  });
  await settle(page);

  // Zoom to the bottleneck, then further in: dots become cars in lanes.
  await page.click("#limit-btn");
  await page.waitForTimeout(2500);
  for (let i = 0; i < 2; i++) {
    await page.click(".maplibregl-ctrl-zoom-in");
    await page.waitForTimeout(900);
  }
  await page.waitForLoadState("networkidle").catch(() => {});
  await page.waitForTimeout(1000);
  await record(page, "desktop-cars-in-lanes", { maxMs: 4000 });

  await page.click("[data-v=detail] [data-back]");
  await page.waitForTimeout(800);
  await page.click('.seg button[data-dir="outbound"]');
  await record(page, "desktop-switch-direction", { maxMs: 3000 });
  await browser.close();
}

// Mobile
{
  const W = 390;
  const { browser, page } = await open({ width: W, height: 844 }, true);

  // The sheet: More detail opens and closes it, and so does the grabber.
  await during(page, "mobile-sheet-more-detail", { maxMs: 2800, width: W }, async () => {
    await page.click("#more-btn");
    await page.waitForTimeout(1100);
    await page.click("#more-btn");
  });
  await settle(page, 500);
  await during(page, "mobile-sheet-drag", { maxMs: 3400, width: W }, async () => {
    await dragSheet(page, -140);
    await page.waitForTimeout(900);
    await dragSheet(page, 220);
  });
  await settle(page, 500);

  await page.click("#run-btn");
  await record(page, "mobile-run-analysis", { maxMs: 40000, done: isResult(page), tailMs: 1500, width: W });
  await page.waitForTimeout(800);
  await record(page, "mobile-flow-dots", { maxMs: 4000, width: W });

  // A card on the map, where it has room; with the sheet open, the card takes
  // the sheet's place, and dragging down puts it away.
  const onMap = await jamPoint(page);
  if (onMap) {
    await during(page, "mobile-card-on-map", { maxMs: 1800, width: W }, () => page.mouse.click(onMap.x, onMap.y), 200);
    await closeCard(page);
  }
  await page.click("#more-btn");
  await settle(page, 600);
  const inSheet = await jamPoint(page);
  if (inSheet) {
    await during(page, "mobile-card-in-sheet", { maxMs: 3400, width: W }, async () => {
      await page.mouse.click(inSheet.x, inSheet.y);
      await page.waitForTimeout(1600);
      await dragSheet(page, 200);
    }, 200);
  } else console.log("mobile-card-in-sheet: no confirmed jam above the sheet, skipped");
  await closeCard(page);
  if ((await page.getAttribute("body", "data-more")) === "open") await page.click("#more-btn");
  await settle(page, 600);

  // A detail: the camera goes to the bottleneck; dragging the grabber down goes back.
  await during(page, "mobile-detail-drag-back", { maxMs: 3800, width: W }, async () => {
    await page.click("#limit-btn");
    await page.waitForTimeout(1600);
    await dragSheet(page, 200);
  });
  await settle(page, 600);

  await page.click('.seg button[data-dir="outbound"]');
  await record(page, "mobile-switch-direction", { maxMs: 3000, width: W });
  await browser.close();
}

if (errors.length) {
  console.log("page errors:", errors);
  process.exitCode = 1;
}
