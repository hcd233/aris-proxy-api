"use client";

import { useEffect, useRef } from "react";
import { useMotionTier } from "@/lib/motion/tier-store";
import { shouldRunLoop } from "@/lib/portal-motion";

/**
 * 演示循环：用 setTimeout 链而非 setInterval，每轮重新检查档位与可见性，
 * 切回前台时不会补跑积压的回调。interval 可为函数，按当前状态决定下一拍时长。
 */
export function usePortalLoop(step: () => void, interval: number | (() => number)): void {
  const tier = useMotionTier();
  const stepRef = useRef(step);
  const intervalRef = useRef(interval);

  useEffect(() => {
    stepRef.current = step;
    intervalRef.current = interval;
  });

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const delay = () => {
      const v = intervalRef.current;
      return typeof v === "function" ? v() : v;
    };
    const schedule = () => {
      clearTimeout(timer);
      if (!shouldRunLoop(tier, document.hidden)) return;
      timer = setTimeout(() => {
        stepRef.current();
        schedule();
      }, delay());
    };
    schedule();
    document.addEventListener("visibilitychange", schedule);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", schedule);
    };
  }, [tier]);
}
