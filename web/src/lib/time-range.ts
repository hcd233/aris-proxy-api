import type { Granularity } from "@/lib/types";

export type TimeRangeKey = "1h" | "24h" | "7d" | "30d" | "custom";

export const TIME_RANGE_LABELS: Record<TimeRangeKey, string> = {
  "1h": "Last 1 hour",
  "24h": "Last 24 hours",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  custom: "Custom",
};

export const TIME_RANGE_PRESETS: TimeRangeKey[] = ["1h", "24h", "7d", "30d"];

export function deriveGranularity(rangeMs: number): Granularity {
  const oneHour = 60 * 60 * 1000;
  const oneDay = 24 * oneHour;
  const sevenDays = 7 * oneDay;
  const thirtyDays = 30 * oneDay;
  if (rangeMs <= oneHour) return "minute";
  if (rangeMs <= sevenDays) return "hour";
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
