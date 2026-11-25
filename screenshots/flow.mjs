// Walks the site's user flow and saves a JPG per screen.
// Usage: node flow.mjs <baseUrl> <outDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

const [base = "http://localhost:5176", out = "./out"] = process.argv.slice(2);
const url = `${base}/`;

async function settle(page, ms = 1200) {
  await page.waitForLoadState("networkidle").catch(() => {});
  await page.waitForTimeout(ms);
}

async function flow(device, viewport, isMobile) {
  const dir = join(out, device);
  mkdirSync(dir, { recursive: true });
  const browser = await chromium.launch({ executablePath: process.env.CHROME_BIN });
  const ctx = await browser.newContext({ viewport, deviceScaleFactor: 2, isMobile, hasTouch: isMobile, colorScheme: "dark" });
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  let n = 0;
  const shot = async (name) => {
    n++;
    const file = join(dir, `${String(n).padStart(2, "0")}-${name}.jpg`);
    await page.screenshot({ path: file, type: "jpeg", quality: 88 });
    console.log(file);
  };
  const click = (sel) => page.locator(sel).first().click();
  const setMore = async (want) => {
    if (!isMobile) return;
    const now = await page.getAttribute("body", "data-more");
    if (now !== want) { await click("#more-btn"); await settle(page, 500); }
  };
  const closeCard = async () => {
    const close = page.locator("#peek .card button", { hasText: "Close" });
    if (await close.count()) await close.click();
    else await page.keyboard.press("Escape");
    await settle(page, 600);
  };
  const mapCenter = async () => {
    const b = await page.locator("#map").boundingBox();
    return { x: b.x + b.width / 2, y: b.y + b.height / 2 };
  };

  await page.goto(url);
  await page.waitForSelector("body[data-load=ready]");
  await settle(page, 2500);
  await shot("home-to-petaling-jaya");

  if (isMobile) {
    await setMore("open");
    await shot("home-more-detail-open");
    await setMore("closed");
  }

  await click('.seg button[data-dir="outbound"]');
  await settle(page, 1500);
  await shot("home-to-kota-kemuning");
  await click('.seg button[data-dir="inbound"]');
  await settle(page, 1500);

  // Tap a road before the analysis: the quick look says nothing computed yet.
  const c = await mapCenter();
  await page.mouse.click(c.x, c.y);
  await settle(page, 600);
  if (await page.locator(".card").count()) await shot("home-road-card");
  await closeCard();

  await click("#run-btn");
  await page.waitForSelector("body[data-state=running]", { state: "attached", timeout: 5000 });
  await page.waitForTimeout(1800);
  await shot("running-analysis");
  await page.waitForSelector("body[data-state=result]", { state: "attached", timeout: 60000 });
  await settle(page, 1500);
  await shot("result");

  if (isMobile) {
    await setMore("open");
    await shot("result-more-detail-open");
  }

  await click("#flow-btn");
  await settle(page, 600);
  await shot("result-flow-hidden");
  await click("#flow-btn");
  await settle(page, 300);
  if (isMobile) {
    await click("#more-btn");
    await settle(page, 400);
  }

  // The bottleneck: the panel's detail and the map zoomed to it.
  await click("#limit-btn");
  await settle(page, 2000);
  await shot("bottleneck-detail");

  // Its quick-look card on the map (the map is centred on it now).
  await click("[data-v=detail] [data-back]");
  await settle(page, 800);
  const c2 = await mapCenter();
  await page.mouse.click(c2.x, c2.y);
  await settle(page, 700);
  if (await page.locator(".card").count()) {
    await shot("bottleneck-road-card");
    await page.locator(".card button", { hasText: "View details" }).click();
    await settle(page, 1200);
    await shot("road-card-view-details");
    await click("[data-v=detail] [data-back]");
    await settle(page, 600);
  } else {
    await closeCard();
  }

  // Lane drops list, then one drop.
  await setMore("open");
  await click("#open-list");
  await settle(page, 1500);
  await shot("lane-drops-list");
  await page.locator("#list button").first().click();
  await settle(page, 2000);
  await shot("lane-drop-detail");

  // The drop's quick-look card (the map is centred on the drop).
  await click("[data-v=detail] [data-back]");
  await settle(page, 800);
  const c3 = await mapCenter();
  await page.mouse.click(c3.x, c3.y);
  await settle(page, 700);
  if (await page.locator(".card").count()) await shot("lane-drop-card");
  await closeCard();

  // How this works.
  await setMore("open");
  await click('[data-open="how"]');
  await settle(page, 800);
  await shot("how-this-works");
  await page.evaluate(() => {
    const s = document.querySelector("[data-v=how]");
    s?.scrollIntoView({ block: "end" });
    const p = document.getElementById("panel");
    if (p) p.scrollTop = p.scrollHeight;
  });
  await settle(page, 400);
  await shot("how-this-works-end");
  await click("[data-v=how] [data-back]");
  await settle(page, 800);

  // Outbound result.
  await click('.seg button[data-dir="outbound"]');
  await settle(page, 2000);
  await shot("result-to-kota-kemuning");

  if (errors.length) console.log(`${device} page errors:`, errors);
  await browser.close();
}

await flow("desktop", { width: 1440, height: 900 }, false);
await flow("mobile", { width: 390, height: 844 }, true);
