"use client";

import { matchByDataKey } from "recharts";
import { useTheme } from "@/lib/theme";
import { useMotionTier } from "@/lib/motion/tier-store";

export interface ChartMotion {
  isAnimationActive: boolean;
  animationDuration: number;
  animationEasing: "ease-in-out" | "ease-out";
  animationMatchBy: ReturnType<typeof matchByDataKey>;
}

const CHART_DRAW_MS = 600;

/**
 * 刷新补间按 `time` 配对新旧点位（recharts 默认按下标配对）：时间窗口滑动时
 * 已有的点原地过渡、新点从右侧滑入、过期点从左侧滑出，而不是整条线扭动重画。
 * 使用本 hook 的图表数据行都必须带 `time` 字段。
 */
const MATCH_BY_TIME = matchByDataKey("time");

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
    animationMatchBy: MATCH_BY_TIME,
  };
}
