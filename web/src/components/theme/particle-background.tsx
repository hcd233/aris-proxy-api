"use client";

import { useEffect, useRef } from "react";
import { useTheme } from "@/lib/theme";

const STAR_COUNT = 40;
const POINTER_RADIUS = 120;
/* Moonlit neutrals: white / warm moonlight / mid gray — no blue-violet. */
const STAR_COLORS = ["#FFFFFF", "#FFEED9", "#CFCFCF"];
const PARALLAX_RANGE = 40;

interface Star {
  nx: number; // normalized x, 0..1
  ny: number; // normalized y, 0..1
  r: number;
  vx: number; // px per frame
  vy: number;
  phase: number;
  color: string;
}

/**
 * Moon scene background. Visibility + animation are driven by the active
 * theme (only moonshot renders the scene) via useTheme(), so this
 * component re-renders on toggle. The effect below depends on `theme`
 * and tears down / rebuilds listeners + rAF when it changes.
 *
 * Composition follows moonshot.ai: a moon disc + god rays (pure CSS)
 * over a sparse twinkling-star canvas, all on a pure black field.
 */
export function ParticleBackground() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const { theme } = useTheme();

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;

    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const dpr = Math.min(window.devicePixelRatio || 1, 2);

    let raf = 0;
    let width = window.innerWidth;
    let height = window.innerHeight;
    let running = false;
    const pointer = { x: -9999, y: -9999 };

    const stars: Star[] = [];
    for (let i = 0; i < STAR_COUNT; i++) {
      stars.push({
        nx: Math.random(),
        ny: Math.random(),
        r: 0.6 + Math.random() * 1.2,
        vx: (Math.random() - 0.5) * 0.15,
        vy: (Math.random() - 0.5) * 0.15,
        phase: Math.random() * Math.PI * 2,
        color: STAR_COLORS[i % STAR_COLORS.length],
      });
    }

    const resize = () => {
      width = window.innerWidth;
      height = window.innerHeight;
      canvas.width = Math.floor(width * dpr);
      canvas.height = Math.floor(height * dpr);
      canvas.style.width = `${width}px`;
      canvas.style.height = `${height}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    window.addEventListener("resize", resize);

    const drawStar = (x: number, y: number, s: Star, alpha: number, glow: number) => {
      ctx.globalAlpha = Math.min(alpha, 1);
      ctx.fillStyle = s.color;
      ctx.beginPath();
      ctx.arc(x, y, s.r * glow, 0, Math.PI * 2);
      ctx.fill();
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
        /* Dimmer than a classic starfield: stars accompany the moon,
         * they don't compete with it. */
        drawStar(x, y, s, 0.22 + 0.3 * twinkle * twinkle + boost * 0.25, 1 + boost * 0.8);
      }
      ctx.globalAlpha = 1;
      raf = requestAnimationFrame(frame);
    };

    const drawStatic = () => {
      ctx.clearRect(0, 0, width, height);
      for (const s of stars) {
        drawStar(s.nx * width, s.ny * height, s, 0.75, 1);
      }
      ctx.globalAlpha = 1;
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
      } else if (theme === "moonshot" && !reduced && !running) {
        raf = requestAnimationFrame(frame);
        running = true;
      }
    };

    const stop = () => {
      cancelAnimationFrame(raf);
      running = false;
      window.removeEventListener("pointermove", onPointerMove);
      document.removeEventListener("pointerout", onPointerOut);
      window.removeEventListener("scroll", onScroll, true);
      canvas.style.transform = "";
      ctx.clearRect(0, 0, width, height);
    };

    const start = () => {
      stop();
      if (theme !== "moonshot") return;
      if (reduced) {
        drawStatic();
        return;
      }
      window.addEventListener("pointermove", onPointerMove, { passive: true });
      document.addEventListener("pointerout", onPointerOut);
      window.addEventListener("scroll", onScroll, { capture: true, passive: true });
      raf = requestAnimationFrame(frame);
      running = true;
    };

    document.addEventListener("visibilitychange", onVisibility);
    start();

    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      stop();
      window.removeEventListener("resize", resize);
    };
  }, [theme]);

  return (
    <div aria-hidden="true" className="particle-bg">
      <canvas ref={canvasRef} />
      <div className="moon-ray moon-ray-b" />
      <div className="moon-ray moon-ray-a" />
      <div className="moon-ray moon-ray-c" />
      <div className="moon" />
      <div className="scene-fade" />
    </div>
  );
}
