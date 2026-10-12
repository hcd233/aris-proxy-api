"use client";

import { useTheme } from "@/lib/theme";
import { useMotionTier } from "@/lib/motion/tier-store";

export interface ChartMotion {
  isAnimationActive: boolean;
  animationDuration: number;
  animationEasing: "ease-in-out" | "ease-out";
}

const CHART_DRAW_MS = 600;

/**
 * recharts timing per Motion World (spec §7.3): atelier draws like a pen
 * (ease-in-out), lunar "ignites" (ease-out; the glow is CSS, see
 * styles/motion-signature.css). recharts animates in JS, so the CSS
 * reduced-motion guard cannot reach it — the `still` tier turns it off here.
 */
export function useChartMotion(): ChartMotion {
  const { theme } = useTheme();
  const tier = useMotionTier();
  return {
    isAnimationActive: tier !== "still",
    animationDuration: CHART_DRAW_MS,
    animationEasing: theme === "moonshot" ? "ease-out" : "ease-in-out",
  };
}
