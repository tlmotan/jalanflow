// The orb: the logo beside the name. A sphere of ink dots, ordered and
// turning slowly at rest. While the analysis runs it scatters, then tightens
// as flow is found (disorder = 1 − found ÷ max). It never sits on the map,
// so it cannot be mistaken for a computed result (decisions 0011, 0013).
// Our own canvas code, ported from docs/design/project/light-look-mockup.html.

// Display constants (docs/design/project/DESIGN_BRIEF.md §10).
const POINTS = 190;
const GOLDEN_ANGLE = 2.399963; // radians; spreads points evenly over a sphere
const EASE = 0.05; // share of the gap to the target closed each frame
const SPIN_PER_MS = 0.0005; // radians, at rest
const SPIN_BOOST = 2.5; // extra spin at full disorder
const SCATTER = 0.5; // how far a dot strays, in sphere radii
const TILT = 0.42; // radians
const RADIUS_SHARE = 0.33; // sphere radius as a share of the canvas

interface Dot {
  x: number;
  y: number;
  z: number;
  sx: number;
  sy: number;
  sz: number;
  phase: number;
}

export interface Orb {
  /** 0 = ordered sphere, 1 = fully scattered. */
  setDisorder: (n: number) => void;
}

export function initOrb(canvas: HTMLCanvasElement): Orb {
  const ctx = canvas.getContext("2d");
  if (!ctx) return { setDisorder: () => {} };
  const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  const dots: Dot[] = [];
  for (let i = 0; i < POINTS; i++) {
    const y = 1 - (2 * (i + 0.5)) / POINTS;
    const r = Math.sqrt(1 - y * y);
    const th = i * GOLDEN_ANGLE;
    dots.push({
      x: r * Math.cos(th),
      y,
      z: r * Math.sin(th),
      sx: Math.random() * 2 - 1,
      sy: Math.random() * 2 - 1,
      sz: Math.random() * 2 - 1,
      phase: Math.random() * Math.PI * 2,
    });
  }

  let ink: string | undefined;
  let target = 0;
  let chaos = 0;
  let spin = 0;
  let last = 0;
  const ct = Math.cos(TILT);
  const st = Math.sin(TILT);

  const draw = (now: number): void => {
    const size = canvas.clientWidth;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    if (canvas.width !== size * dpr) canvas.width = canvas.height = size * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, size, size);
    // The ink of the place the orb sits in, not of the page: the text column
    // may carry its own colours.
    ink ??= getComputedStyle(canvas).getPropertyValue("--ink").trim();
    ctx.fillStyle = ink;
    const R = size * RADIUS_SHARE;
    const c = size / 2;
    const cs = Math.cos(spin);
    const sn = Math.sin(spin);
    for (const p of dots) {
      const w = chaos * (0.55 + 0.45 * Math.sin(now * 0.0022 + p.phase)) * SCATTER;
      const x = p.x + p.sx * w;
      const y = p.y + p.sy * w;
      const z = p.z + p.sz * w;
      const x1 = x * cs + z * sn;
      const z1 = -x * sn + z * cs;
      const y2 = y * ct - z1 * st;
      const z2 = y * st + z1 * ct;
      const depth = Math.min(1, Math.max(0, (z2 + 1.3) / 2.6)); // near dots are bigger and darker
      ctx.globalAlpha = 0.12 + 0.88 * depth;
      ctx.beginPath();
      ctx.arc(c + x1 * R, c + y2 * R, (size / 60) * (0.5 + 0.7 * depth), 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.globalAlpha = 1;
  };

  const tick = (now: number): void => {
    const dt = last ? Math.min(now - last, 100) : 16; // clamp after a hidden tab
    last = now;
    chaos += (target - chaos) * EASE;
    spin += dt * SPIN_PER_MS * (1 + chaos * SPIN_BOOST);
    draw(now);
    // requestAnimationFrame stops by itself while the tab is hidden.
    requestAnimationFrame(tick);
  };

  if (still) {
    draw(0);
    new ResizeObserver(() => draw(0)).observe(canvas);
  } else {
    requestAnimationFrame(tick);
  }

  return {
    setDisorder(n) {
      // Reduced motion: the orb stays an ordered, still sphere.
      if (!still) target = Math.min(1, Math.max(0, n));
    },
  };
}
