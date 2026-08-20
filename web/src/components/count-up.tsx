"use client";

import { useEffect, useRef } from "react";

const DURATION_MS = 700;

function easeOutCubic(p: number): number {
  return 1 - Math.pow(1 - p, 3);
}

/**
 * Animated integer counter. Writes textContent directly via rAF (same
 * DOM-manipulation pattern as LocaleFade) so no setState-in-effect and no
 * re-renders during the animation. Honors prefers-reduced-motion by
 * jumping straight to the final value.
 */
export function CountUp({ value, className }: { value: number; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const displayedRef = useRef(0);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const from = displayedRef.current;
    const to = value;

    const settle = () => {
      displayedRef.current = to;
      el.textContent = String(to);
    };

    if (from === to || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      settle();
      return;
    }

    let raf = 0;
    const start = performance.now();
    const tick = (now: number) => {
      const p = Math.min((now - start) / DURATION_MS, 1);
      const current = Math.round(from + (to - from) * easeOutCubic(p));
      displayedRef.current = current;
      el.textContent = String(current);
      if (p < 1) {
        raf = requestAnimationFrame(tick);
      } else {
        settle();
      }
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [value]);

  return (
    <span ref={ref} className={className}>
      {value}
    </span>
  );
}
