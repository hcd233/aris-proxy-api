/* Canvas 2D starfield: lunar's `lite` (animated) and `still` (one static
 * frame) form, migrated from the former ParticleBackground. The caller
 * decides when it runs; canvas size comes from CSS (.ambient-stage canvas). */

const STAR_COUNT = 70;
const POINTER_RADIUS = 120;
const PARALLAX_RANGE = 40;
const MAX_DPR = 2;

interface Star {
  nx: number; // normalized x, 0..1
  ny: number; // normalized y, 0..1
  r: number;
  vx: number; // px per frame
  vy: number;
  phase: number;
}

export function startStarfield2D(canvas: HTMLCanvasElement, animate: boolean): () => void {
  const ctx = canvas.getContext("2d");
  if (!ctx) return () => {};

  const dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);
  let width = window.innerWidth;
  let height = window.innerHeight;
  let raf = 0;
  let running = false;
  const pointer = { x: -9999, y: -9999 };

  const stars: Star[] = Array.from({ length: STAR_COUNT }, () => ({
    nx: Math.random(),
    ny: Math.random(),
    r: 0.6 + Math.random() * 1.2,
    vx: (Math.random() - 0.5) * 0.15,
    vy: (Math.random() - 0.5) * 0.15,
    phase: Math.random() * Math.PI * 2,
  }));

  const markReady = () => {
    if (!canvas.dataset.ready) canvas.dataset.ready = "1";
  };

  const drawStar = (x: number, y: number, s: Star, alpha: number, glow: number) => {
    ctx.globalAlpha = Math.min(alpha, 1);
    ctx.fillStyle = "#FFFFFF";
    ctx.beginPath();
    ctx.arc(x, y, s.r * glow, 0, Math.PI * 2);
    ctx.fill();
  };

  const drawStatic = () => {
    ctx.clearRect(0, 0, width, height);
    for (const s of stars) drawStar(s.nx * width, s.ny * height, s, 0.75, 1);
    ctx.globalAlpha = 1;
    markReady();
  };

  const frame = (t: number) => {
    ctx.clearRect(0, 0, width, height);
    for (const s of stars) {
      s.nx += s.vx / width;
      s.ny += s.vy / height;
      if (s.nx < -0.01) s.nx = 1.01;
      else if (s.nx > 1.01) s.nx = -0.01;
      if (s.ny < -0.01) s.ny = 1.01;
      else if (s.ny > 1.01) s.ny = -0.01;

      let x = s.nx * width;
      let y = s.ny * height;
      let boost = 0;
      const dx = pointer.x - x;
      const dy = pointer.y - y;
      const dist = Math.hypot(dx, dy);
      if (dist < POINTER_RADIUS && dist > 0.1) {
        const f = 1 - dist / POINTER_RADIUS;
        s.nx += (dx * f * 0.02) / width;
        s.ny += (dy * f * 0.02) / height;
        x = s.nx * width;
        y = s.ny * height;
        boost = f;
      }
      const twinkle = Math.sin(t / 900 + s.phase);
      drawStar(x, y, s, 0.35 + 0.4 * twinkle * twinkle + boost * 0.25, 1 + boost * 0.8);
    }
    ctx.globalAlpha = 1;
    markReady();
    raf = requestAnimationFrame(frame);
  };

  const resize = () => {
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    if (!animate) drawStatic();
  };

  const onPointerMove = (e: PointerEvent) => {
    pointer.x = e.clientX;
    pointer.y = e.clientY;
  };
  const onPointerOut = () => {
    pointer.x = -9999;
    pointer.y = -9999;
  };
  const onScroll = () => {
    const y = document.scrollingElement?.scrollTop ?? 0;
    canvas.style.transform = `translateY(${-(y * 0.05) % PARALLAX_RANGE}px)`;
  };
  const onVisibility = () => {
    if (document.hidden) {
      cancelAnimationFrame(raf);
      running = false;
    } else if (animate && !running) {
      raf = requestAnimationFrame(frame);
      running = true;
    }
  };

  resize();
  window.addEventListener("resize", resize);
  document.addEventListener("visibilitychange", onVisibility);
  if (animate) {
    window.addEventListener("pointermove", onPointerMove, { passive: true });
    document.addEventListener("pointerout", onPointerOut);
    window.addEventListener("scroll", onScroll, { capture: true, passive: true });
    raf = requestAnimationFrame(frame);
    running = true;
  }

  return () => {
    cancelAnimationFrame(raf);
    running = false;
    window.removeEventListener("resize", resize);
    document.removeEventListener("visibilitychange", onVisibility);
    window.removeEventListener("pointermove", onPointerMove);
    document.removeEventListener("pointerout", onPointerOut);
    window.removeEventListener("scroll", onScroll, true);
    canvas.style.transform = "";
    ctx.clearRect(0, 0, width, height);
  };
}
