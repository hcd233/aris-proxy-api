"use client";

import { useLayoutEffect, useRef } from "react";
import { useMotionTier } from "@/lib/motion/tier-store";
import { springAtRest, springStep, type SpringParams, type SpringState } from "@/lib/motion/spring";
import { ATELIER_SPRING, LUNAR_SPRING } from "@/lib/motion/tokens";

/** Lunar afterglow: a softer spring trailing the indicator like a comet tail. */
const TRAIL_SPRING: SpringParams = { stiffness: 90, damping: 14, mass: 1 };
const SWIPE_MS = 320;
const MAX_DT_S = 0.05;

/**
 * Top of `el` relative to `ancestor`'s padding box — the containing block of the
 * absolutely positioned indicator. Read from layout offsets on purpose: the rows
 * rise from `translateY(8px)` (atelier-rise, `backwards` fill) and
 * `getBoundingClientRect()` reports that first frame, while the indicator measures
 * once per `layoutKey` — the skewed read stuck 8px below the row for good.
 */
function layoutTopWithin(el: HTMLElement, ancestor: HTMLElement): number {
  let top = 0;
  let node: HTMLElement | null = el;
  while (node && node !== ancestor) {
    top += node.offsetTop;
    node = node.offsetParent as HTMLElement | null;
  }
  return top;
}

/**
 * Sidebar navigation indicator (spec §7.4): one element that travels to the
 * active item (`[data-nav-item][data-active]` inside the parent <nav>) on a
 * spring — critically damped highlighter in atelier, an underdamped glowing
 * pill with a trailing afterglow in lunar. `layoutKey` must change whenever
 * the active item may have moved (route, open groups, collapsed sidebar).
 */
export function NavIndicator({ layoutKey }: { layoutKey: string }) {
  const tier = useMotionTier();
  const ref = useRef<HTMLSpanElement>(null);
  const trailRef = useRef<HTMLSpanElement>(null);
  const placed = useRef(false);
  const pos = useRef<{ main: SpringState; trail: SpringState }>({
    main: { x: 0, v: 0 },
    trail: { x: 0, v: 0 },
  });

  // Layout effect: the first placement must land before paint so the active
  // item never flashes without its highlight.
  useLayoutEffect(() => {
    const el = ref.current;
    const trail = trailRef.current;
    const nav = el?.parentElement;
    if (!el || !trail || !nav) return;

    const active = nav.querySelector<HTMLElement>("[data-nav-item][data-active]");
    if (!active) {
      el.dataset.hidden = "";
      trail.dataset.hidden = "";
      placed.current = false;
      return;
    }
    const top = layoutTopWithin(active, nav);
    const height = active.offsetHeight;
    el.style.height = `${height}px`;
    trail.style.height = `${height}px`;
    delete el.dataset.hidden;
    delete trail.dataset.hidden;

    const write = () => {
      el.style.transform = `translateY(${pos.current.main.x.toFixed(2)}px)`;
      trail.style.transform = `translateY(${pos.current.trail.x.toFixed(2)}px)`;
    };
    if (!placed.current || tier === "still") {
      pos.current = { main: { x: top, v: 0 }, trail: { x: top, v: 0 } };
      placed.current = true;
      write();
      return;
    }

    const lunar = document.documentElement.dataset.theme === "moonshot";
    const spring = lunar ? LUNAR_SPRING : ATELIER_SPRING;
    if (!lunar) {
      // 荧光笔: the highlighter stroke is swiped in again on every arrival.
      el.animate(
        [{ clipPath: "inset(0 100% 0 0 round 8px)" }, { clipPath: "inset(0 0 0 0 round 8px)" }],
        { duration: SWIPE_MS, easing: "cubic-bezier(0.65, 0, 0.35, 1)" },
      );
    }

    let raf = 0;
    let last = performance.now();
    const step = (now: number) => {
      const dt = Math.min((now - last) / 1000, MAX_DT_S);
      last = now;
      const p = pos.current;
      p.main = springStep(p.main, top, dt, spring);
      p.trail = lunar ? springStep(p.trail, top, dt, TRAIL_SPRING) : p.main;
      write();
      if (!springAtRest(p.main, top, 0.05) || !springAtRest(p.trail, top, 0.05)) {
        raf = requestAnimationFrame(step);
      }
    };
    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  }, [layoutKey, tier]);

  return (
    <>
      <span ref={trailRef} aria-hidden="true" className="nav-indicator-trail" data-hidden="" />
      <span ref={ref} aria-hidden="true" className="nav-indicator" data-hidden="" />
    </>
  );
}
