"use client";

import { useEffect, useRef } from "react";
import { planDigits, type DigitPlan } from "@/lib/motion/metric-digits";
import { cn } from "@/lib/utils";

const ODOMETER_MS = 700;
const ODOMETER_STEP_MS = 30;
const ODOMETER_EASE = "cubic-bezier(0.22, 1, 0.36, 1)";
const DECODE_FIRST_MS = 700;
const DECODE_UPDATE_MS = 300;
const DECODE_TICK_MS = 40;
const LOCK_FLASH_MS = 320;

/** 里程表 (atelier): each changed digit is a 0–9 strip rolled into place,
 * right to left, critically damped. */
function runOdometer(
  el: HTMLElement,
  plan: DigitPlan,
  text: string,
  onSettled: () => void,
): () => void {
  const animations: Animation[] = [];
  const nodes = plan.cells.map((cell) => {
    if (!cell.changed) return document.createTextNode(cell.to);
    const box = document.createElement("span");
    box.className = "odo-cell";
    const strip = document.createElement("span");
    strip.className = "odo-strip";
    strip.textContent = "0\n1\n2\n3\n4\n5\n6\n7\n8\n9";
    box.appendChild(strip);
    return box;
  });
  el.replaceChildren(...nodes);

  let order = 0;
  for (let i = plan.cells.length - 1; i >= 0; i--) {
    const cell = plan.cells[i];
    if (!cell.changed) continue;
    const strip = (nodes[i] as HTMLElement).firstElementChild as HTMLElement;
    animations.push(
      strip.animate(
        [
          { transform: `translateY(-${Number(cell.from) * 10}%)` },
          { transform: `translateY(-${Number(cell.to) * 10}%)` },
        ],
        {
          duration: ODOMETER_MS,
          delay: order * ODOMETER_STEP_MS,
          easing: ODOMETER_EASE,
          fill: "both",
        },
      ),
    );
    order++;
  }
  // Settle back to plain text so the number stays selectable / copyable.
  Promise.all(animations.map((a) => a.finished))
    .then(() => {
      el.textContent = text;
      onSettled();
    })
    .catch(() => {});
  return () => {
    for (const a of animations) a.cancel();
    el.textContent = text;
  };
}

/** 解码锁定 (lunar): changed digits flicker through random values and lock
 * left to right, each lock flashing once. */
function runDecode(
  el: HTMLElement,
  plan: DigitPlan,
  text: string,
  first: boolean,
  onSettled: () => void,
): () => void {
  const spans = plan.cells.map((cell) => {
    const s = document.createElement("span");
    s.textContent = cell.changed ? cell.from : cell.to;
    return s;
  });
  el.replaceChildren(...spans);

  const total = first ? DECODE_FIRST_MS : DECODE_UPDATE_MS;
  const changed = plan.cells.flatMap((c, i) => (c.changed ? [i] : []));
  const lockAt = changed.map(
    (_, k) => total * 0.3 + (changed.length > 1 ? k / (changed.length - 1) : 1) * total * 0.7,
  );
  const locked = new Set<number>();
  const start = performance.now();
  let lastTick = -DECODE_TICK_MS;
  let raf = 0;
  let timer = 0;

  const frame = (now: number) => {
    const t = now - start;
    const tick = t - lastTick >= DECODE_TICK_MS;
    if (tick) lastTick = t;
    changed.forEach((i, k) => {
      if (locked.has(i)) return;
      const s = spans[i];
      if (t >= lockAt[k]) {
        locked.add(i);
        s.textContent = plan.cells[i].to;
        s.className = "decode-lock";
      } else if (tick) {
        s.textContent = String(Math.floor(Math.random() * 10));
      }
    });
    if (locked.size < changed.length) {
      raf = requestAnimationFrame(frame);
    } else {
      timer = window.setTimeout(() => {
        el.textContent = text;
        onSettled();
      }, LOCK_FLASH_MS);
    }
  };
  raf = requestAnimationFrame(frame);
  return () => {
    cancelAnimationFrame(raf);
    window.clearTimeout(timer);
    el.textContent = text;
  };
}

/**
 * Dashboard metric (spec §7.3), replacing CountUp: odometer roll in atelier,
 * decode-and-lock in lunar; only the digits that changed move. DOM is
 * written directly (no re-renders); screen readers read the sr-only value.
 * `still` tier shows the final value at once.
 */
export function MetricNumber({ value, className }: { value: number; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const shown = useRef<number | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const from = shown.current;
    shown.current = value;
    const text = String(value);
    const root = document.documentElement;
    const plan = planDigits(from, value);
    if (root.dataset.motionTier === "still" || plan.cells.every((c) => !c.changed)) {
      el.textContent = text;
      return;
    }
    let settled = false;
    const markSettled = () => {
      settled = true;
    };
    const stop =
      root.dataset.theme === "moonshot"
        ? runDecode(el, plan, text, from === null, markSettled)
        : runOdometer(el, plan, text, markSettled);
    return () => {
      stop();
      // Interrupted before it settled (strict-mode remount, or a new value
      // mid-roll): replay from the last settled value, not the half-shown one.
      if (!settled) shown.current = from;
    };
  }, [value]);

  return (
    <span className={cn("tabular-nums", className)}>
      <span className="sr-only">{value}</span>
      <span ref={ref} aria-hidden="true">
        {value}
      </span>
    </span>
  );
}
