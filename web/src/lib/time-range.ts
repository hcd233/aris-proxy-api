import { addDays, addHours, addMinutes, addWeeks, isAfter } from "date-fns";
import type { Granularity } from "@/lib/types";

export type TimeRangeKey = "1h" | "24h" | "7d" | "30d" | "custom";

export const TIME_RANGE_PRESETS: TimeRangeKey[] = ["1h", "24h", "7d", "30d"];

export function deriveGranularity(rangeMs: number): Granularity {
  const oneHour = 60 * 60 * 1000;
  const oneDay = 24 * oneHour;
  const thirtyDays = 30 * oneDay;
  if (rangeMs <= oneHour) return "minute";
  if (rangeMs <= oneDay) return "hour";
  if (rangeMs <= thirtyDays) return "day";
  return "week";
}

export function formatChartTime(time: string, key: TimeRangeKey, customStart?: string, customEnd?: string): string {
  const { granularity } = computeRange(key, customStart, customEnd);
  const d = new Date(time);
  if (granularity === "minute" || granularity === "hour") {
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleDateString([], { month: "2-digit", day: "2-digit" });
}

// 生成 startTime 到 endTime 之间按 granularity 等间隔的时间点（含起点，最多 200 点，防自定义超长区间爆炸）。
// 用于后端无数据时仍渲染空坐标轴。
export function generateEmptyTimeline(
  startTime: string,
  endTime: string,
  granularity: Granularity,
): string[] {
  const start = new Date(startTime);
  const end = new Date(endTime);
  const step = {
    minute: addMinutes,
    hour: addHours,
    day: addDays,
    week: addWeeks,
  }[granularity];
  const points: string[] = [];
  const limit = 200;
  let cur = start;
  while (!isAfter(cur, end) && points.length < limit) {
    points.push(cur.toISOString());
    cur = step(cur, 1);
  }
  return points;
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
