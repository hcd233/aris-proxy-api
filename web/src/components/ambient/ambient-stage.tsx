"use client";

import { useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { useTheme } from "@/lib/theme";
import { downgradeMotionTier, useMotionTier } from "@/lib/motion/tier-store";
import {
  PRESENCE_BUSY,
  bindAttention,
  createAttention,
  markActive,
  stepAttention,
} from "@/lib/motion/attention";
import { createGovernor, governorTick, resetGovernorWindow } from "@/lib/motion/fps-governor";
import { emitAmbientPulse, onAmbientPulse, type AmbientPulse } from "@/lib/motion/pulse";
import { createRenderer, type SceneProgram } from "./gl";
import { startStarfield2D } from "./fallback-2d";
import { atelierScene } from "./scenes/atelier";
import { lunarScene } from "./scenes/lunar";
import { PULSE_KIND_CODE, PULSE_SLOTS, type AmbientScene, type SceneId } from "./scenes/common";

const SCENES: Record<SceneId, AmbientScene> = { atelier: atelierScene, lunar: lunarScene };
const RENDER_SCALE = 0.75;
const REDUCED_SCALE = 0.5;
const MAX_DPR = 2;
const AMBIENT_FRAME_MS = 1000 / 30;
/** Pulses keep the stage at full frame rate this long (atelier's wet phase). */
const PULSE_HOT_S = 2.2;
const FOCUS_TAU_MS = 300;
/** A press this recent anchors the route pulse; otherwise viewport center. */
const ROUTE_PULSE_WINDOW_MS = 1000;
const EMPTY_PULSE_START = -1000;

function sceneOf(theme: string | undefined): SceneId {
  return theme === "moonshot" ? "lunar" : "atelier";
}

/** Expo-out, close to --ease-drift cubic-bezier(.16,1,.3,1). */
function easeDrift(p: number): number {
  return p >= 1 ? 1 : 1 - Math.pow(2, -10 * p);
}

/**
 * Ambient Stage (web/CONTEXT.md「Motion」): the single fixed backdrop under
 * all content. `immersive` renders the active skin's WebGL scene; `lite` /
 * `still` fall back to CSS textures (atelier) or the Canvas 2D starfield
 * (lunar). Any GL failure downgrades the Motion Tier, which remounts the
 * canvas (key={mode}) for the fallback path.
 */
export function AmbientStage() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const { theme } = useTheme();
  const tier = useMotionTier();
  const pathname = usePathname();
  const mode = tier === "immersive" ? "gl" : "fallback";
  const sceneRef = useRef<SceneId>("atelier");
  const prevPathRef = useRef(pathname);
  const lastDownRef = useRef({ x: 0, y: 0, t: Number.NEGATIVE_INFINITY });

  // Follow the skin via <html data-theme> (written before setState), so the
  // very first effect already sees the real skin, not the hydration default.
  useEffect(() => {
    sceneRef.current = sceneOf(document.documentElement.dataset.theme);
  }, [theme]);

  useEffect(() => {
    const onDown = (e: PointerEvent) => {
      lastDownRef.current = { x: e.clientX, y: e.clientY, t: performance.now() };
    };
    window.addEventListener("pointerdown", onDown, { capture: true, passive: true });
    return () => window.removeEventListener("pointerdown", onDown, { capture: true });
  }, []);

  // Every route change pulses the stage at the press that caused it.
  useEffect(() => {
    if (prevPathRef.current === pathname) return;
    prevPathRef.current = pathname;
    const down = lastDownRef.current;
    const recent = performance.now() - down.t < ROUTE_PULSE_WINDOW_MS;
    emitAmbientPulse(recent ? { kind: "route", x: down.x, y: down.y } : { kind: "route" });
  }, [pathname]);

  // immersive: WebGL scene loop.
  useEffect(() => {
    if (mode !== "gl") return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const renderer = createRenderer(canvas);
    if (!renderer) {
      downgradeMotionTier("lite");
      return;
    }

    const t0 = performance.now();
    const pulses = new Float32Array(PULSE_SLOTS * 4);
    for (let i = 0; i < PULSE_SLOTS; i++) pulses[i * 4 + 2] = EMPTY_PULSE_START;
    const pointerTarget = { x: 0, y: 0 };
    const pointer = { x: 0, y: 0 };
    let pulseSlot = 0;
    let scale = RENDER_SCALE;
    let governor = createGovernor(t0);
    let attention = createAttention();
    let activeScene: SceneId | null = null;
    let program: SceneProgram | null = null;
    let introStart = t0;
    let introMs = 1;
    let scroll = 0;
    let focus = 0;
    let focusTarget = 0;
    let drift = 0;
    let lastFrame = t0;
    let lastRender = Number.NEGATIVE_INFINITY;
    let raf = 0;
    let alive = true;

    const fail = (reason: unknown) => {
      console.warn("[ambient] WebGL stage disabled:", reason);
      alive = false;
      downgradeMotionTier("lite");
    };

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);
      renderer.resize(
        Math.max(1, Math.floor(window.innerWidth * dpr * scale)),
        Math.max(1, Math.floor(window.innerHeight * dpr * scale)),
      );
      lastRender = Number.NEGATIVE_INFINITY;
    };

    const switchScene = (id: SceneId) => {
      program = renderer.use(id, SCENES[id].fragment);
      activeScene = id;
      introStart = performance.now();
      introMs = SCENES[id].introMs;
      governor = resetGovernorWindow(governor, introStart);
    };

    const onPulse = (p: AmbientPulse) => {
      if (p.kind === "focus-on") {
        focusTarget = 1;
        return;
      }
      if (p.kind === "focus-off") {
        focusTarget = 0;
        return;
      }
      if (p.kind === "intro") {
        introStart = performance.now();
        return;
      }
      const o = pulseSlot * 4;
      pulses[o] = (p.x ?? window.innerWidth / 2) / window.innerWidth;
      pulses[o + 1] = 1 - (p.y ?? window.innerHeight / 2) / window.innerHeight;
      pulses[o + 2] = (performance.now() - t0) / 1000;
      pulses[o + 3] = PULSE_KIND_CODE[p.kind];
      pulseSlot = (pulseSlot + 1) % PULSE_SLOTS;
    };

    const onPointerMove = (e: PointerEvent) => {
      pointerTarget.x = (e.clientX / window.innerWidth) * 2 - 1;
      pointerTarget.y = 1 - (e.clientY / window.innerHeight) * 2;
    };
    const onScroll = (e: Event) => {
      const el = e.target instanceof Element ? e.target : document.scrollingElement;
      if (el) scroll = el.scrollTop / window.innerHeight;
    };
    const onLost = () => fail("context lost");

    const frame = (now: number) => {
      if (!alive) return;
      raf = requestAnimationFrame(frame);

      const verdict = governorTick(governor, now);
      governor = verdict.state;
      if (verdict.action === "reduce-scale") {
        scale = REDUCED_SCALE;
        resize();
      } else if (verdict.action === "downgrade") {
        fail("frame rate below budget");
        return;
      }

      const dt = Math.min(now - lastFrame, 100);
      lastFrame = now;
      attention = stepAttention(attention, now, dt);
      const presence = attention.presence;
      const presenceN = (presence - PRESENCE_BUSY) / (1 - PRESENCE_BUSY);

      const wanted = sceneRef.current;
      if (wanted !== activeScene) {
        try {
          switchScene(wanted);
        } catch (err) {
          fail(err);
          return;
        }
      }
      const scene = SCENES[wanted];

      // The atelier lamp stops following while the user is busy.
      if (!(wanted === "atelier" && presenceN < 0.5)) {
        const k = 1 - Math.exp(-dt / scene.pointerTauMs);
        pointer.x += (pointerTarget.x - pointer.x) * k;
        pointer.y += (pointerTarget.y - pointer.y) * k;
      }
      focus += (focusTarget - focus) * (1 - Math.exp(-dt / FOCUS_TAU_MS));
      drift += (dt / 1000) * (0.5 + 0.5 * presenceN) * (1 - 0.6 * focus);

      const t = (now - t0) / 1000;
      const introP = Math.min((now - introStart) / introMs, 1);
      let hot = introP < 1 || Math.abs(focusTarget - focus) > 0.01;
      for (let i = 0; i < PULSE_SLOTS && !hot; i++) hot = t - pulses[i * 4 + 2] < PULSE_HOT_S;
      if (!hot && now - lastRender < AMBIENT_FRAME_MS - 1) return;
      lastRender = now;

      if (!program) return;
      program.set2f("uResolution", canvas.width, canvas.height);
      program.set1f("uTime", t);
      program.set1f("uDrift", drift);
      program.set2f("uPointer", pointer.x, pointer.y);
      program.set1f("uScroll", scroll);
      program.set1f("uPresence", presence);
      program.set1f("uFocus", focus);
      program.set1f("uIntro", easeDrift(introP));
      program.set4fv("uPulse", pulses);
      renderer.draw();
      if (!canvas.dataset.ready) canvas.dataset.ready = "1";
    };

    const onVisibility = () => {
      cancelAnimationFrame(raf);
      if (document.hidden) return;
      const now = performance.now();
      governor = resetGovernorWindow(governor, now);
      lastFrame = now;
      raf = requestAnimationFrame(frame);
    };

    resize();
    window.addEventListener("resize", resize);
    window.addEventListener("pointermove", onPointerMove, { passive: true });
    window.addEventListener("scroll", onScroll, { capture: true, passive: true });
    document.addEventListener("visibilitychange", onVisibility);
    canvas.addEventListener("webglcontextlost", onLost);
    const unbindAttention = bindAttention(() => {
      attention = markActive(attention, performance.now());
    });
    const offPulse = onAmbientPulse(onPulse);
    raf = requestAnimationFrame(frame);

    return () => {
      alive = false;
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", resize);
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("scroll", onScroll, { capture: true });
      document.removeEventListener("visibilitychange", onVisibility);
      // Detach before dispose(): loseContext() fires `webglcontextlost`.
      canvas.removeEventListener("webglcontextlost", onLost);
      unbindAttention();
      offPulse();
      renderer.dispose();
    };
  }, [mode]);

  // lite / still: lunar uses the Canvas 2D starfield; atelier is pure CSS.
  useEffect(() => {
    if (mode !== "fallback") return;
    const canvas = canvasRef.current;
    if (!canvas || sceneOf(document.documentElement.dataset.theme) !== "lunar") return;
    return startStarfield2D(canvas, tier === "lite");
  }, [mode, theme, tier]);

  return (
    <div aria-hidden="true" className="ambient-stage">
      <canvas key={mode} ref={canvasRef} />
    </div>
  );
}
