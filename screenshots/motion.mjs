// Records short motion clips of the site as GIFs: screenshots in a loop,
// each stamped with the time it was taken, stitched by ffmpeg at real speed.
// Usage: CHROME_BIN=... node motion.mjs <baseUrl> <outDir>   (needs playwright + ffmpeg)
import { chromium } from "playwright";
import { mkdirSync, rmSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";

const [base = "http://localhost:5176", outArg = "./motion"] = process.argv.slice(2);
const out = resolve(outArg);
const url = `${base}/`;
const FPS_CAP = 20; // frames a second in the GIF
const GIF_WIDTH = 900; // px; GIFs get big fast

/**
 * Grab frames until `done()` is true or `maxMs` passes, then write <name>.gif.
 * Chrome's screencast pushes a JPEG each time the page paints, with its time,
 * which keeps up with the map far better than calling screenshot in a loop.
 */
async function record(page, name, { maxMs = 4000, done = async () => false, tailMs = 0, width = GIF_WIDTH } = {}) {
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

async function open(viewport, isMobile) {
  const browser = await chromium.launch({ executablePath: process.env.CHROME_BIN });
  const ctx = await browser.newContext({ viewport, deviceScaleFactor: 1, isMobile, hasTouch: isMobile, colorScheme: "dark" });
  const page = await ctx.newPage();
  await page.goto(url);
  await page.waitForSelector("body[data-load=ready]");
  await page.waitForLoadState("networkidle").catch(() => {});
  await page.waitForTimeout(2500);
  return { browser, page };
}

const isResult = (page) => async () => (await page.getAttribute("body", "data-state")) === "result";

mkdirSync(out, { recursive: true });

// Desktop
{
  const { browser, page } = await open({ width: 1280, height: 800 }, false);
  await page.click("#run-btn");
  await record(page, "desktop-run-analysis", { maxMs: 40000, done: isResult(page), tailMs: 1500 });

  await page.waitForTimeout(800);
  await record(page, "desktop-flow-dots", { maxMs: 4000 });

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
  const { browser, page } = await open({ width: 390, height: 844 }, true);
  await page.click("#run-btn");
  await record(page, "mobile-run-analysis", { maxMs: 40000, done: isResult(page), tailMs: 1500, width: 390 });
  await page.waitForTimeout(800);
  await record(page, "mobile-flow-dots", { maxMs: 4000, width: 390 });
  await browser.close();
}
