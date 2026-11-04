import type { Granularity } from "@/lib/types";

export type TimeRangeKey = "1h" | "24h" | "7d" | "30d" | "custom";

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

/** 各粒度的桶宽（与后端 port.BuildBuckets 一致；week 按 7 天计） */
export const GRANULARITY_STEP_MS: Record<Granularity, number> = {
  minute: MINUTE_MS,
  "5minute": 5 * MINUTE_MS,
  "10minute": 10 * MINUTE_MS,
  "30minute": 30 * MINUTE_MS,
  hour: HOUR_MS,
  "4hour": 4 * HOUR_MS,
  day: DAY_MS,
  week: 7 * DAY_MS,
};

/** 由细到粗的粒度阶梯 */
const GRANULARITY_LADDER: Granularity[] = [
  "minute",
  "5minute",
  "10minute",
  "30minute",
  "hour",
  "4hour",
  "day",
  "week",
];

/**
 * 图表点数上限：取「桶数不超过它」的最细粒度。桶太宽曲线会被均值抹平——
 * 曾按 ≤1h/≤1d/≤30d 切 minute/hour/day，7d 只有 7 个点；现在 1h=60、24h=144、7d=168、30d=180 个点。
 */
export const MAX_CHART_POINTS = 200;

export function deriveGranularity(rangeMs: number): Granularity {
  return (
    GRANULARITY_LADDER.find((g) => rangeMs / GRANULARITY_STEP_MS[g] <= MAX_CHART_POINTS) ?? "week"
  );
}

/** 图表时间文案：日内桶在跨天区间带日期（否则 7d 的「14:00」分不清是哪天），日/周桶只显示日期 */
export function formatChartTime(
  time: string,
  key: TimeRangeKey,
  customStart?: string,
  customEnd?: string,
): string {
  const { startTime, endTime, granularity } = computeRange(key, customStart, customEnd);
  const d = new Date(time);
  if (GRANULARITY_STEP_MS[granularity] >= DAY_MS) {
    return d.toLocaleDateString([], { month: "2-digit", day: "2-digit" });
  }
  const spanMs = new Date(endTime).getTime() - new Date(startTime).getTime();
  if (spanMs <= DAY_MS) {
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleString([], {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function computeRange(
  key: TimeRangeKey,
  customStart?: string,
  customEnd?: string,
): { startTime: string; endTime: string; granularity: Granularity } {
  const now = new Date();
  let start: Date;
  if (key === "custom") {
    start = customStart ? new Date(customStart) : new Date(now.getTime() - 24 * 60 * 60 * 1000);
    const end = customEnd ? new Date(customEnd) : now;
    return {
      startTime: start.toISOString(),
      endTime: end.toISOString(),
      granularity: deriveGranularity(end.getTime() - start.getTime()),
    };
  }
  start = new Date(now);
  if (key === "1h") start.setHours(start.getHours() - 1);
  else if (key === "24h") start.setHours(start.getHours() - 24);
  else if (key === "7d") start.setDate(start.getDate() - 7);
  else if (key === "30d") start.setDate(start.getDate() - 30);
  const rangeMs = now.getTime() - start.getTime();
  return {
    startTime: start.toISOString(),
    endTime: now.toISOString(),
    granularity: deriveGranularity(rangeMs),
  };
}
