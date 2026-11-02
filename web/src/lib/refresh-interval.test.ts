import { describe, expect, it } from "vitest";

import {
  DASHBOARD_REFRESH_INTERVALS,
  DEFAULT_DASHBOARD_REFRESH_MS,
  DEFAULT_MONITOR_REFRESH_MS,
  MONITOR_REFRESH_INTERVALS,
  formatRefreshInterval,
  normalizeRefreshInterval,
} from "./refresh-interval";

describe("refresh intervals", () => {
  it("keeps each page's options sorted and unique", () => {
    for (const options of [MONITOR_REFRESH_INTERVALS, DASHBOARD_REFRESH_INTERVALS]) {
      const values = options.map((option) => option.ms);
      expect(values).toEqual([...values].sort((a, b) => a - b));
      expect(new Set(values).size).toBe(values.length);
      expect(values.every((value) => value >= 1_000)).toBe(true);
    }
  });

  it("includes each page's default in its own options", () => {
    expect(MONITOR_REFRESH_INTERVALS.some((o) => o.ms === DEFAULT_MONITOR_REFRESH_MS)).toBe(true);
    expect(DASHBOARD_REFRESH_INTERVALS.some((o) => o.ms === DEFAULT_DASHBOARD_REFRESH_MS)).toBe(
      true,
    );
  });

  it("falls back for unknown or corrupted values", () => {
    expect(normalizeRefreshInterval(15_000, MONITOR_REFRESH_INTERVALS, 5_000)).toBe(15_000);
    expect(normalizeRefreshInterval(5_000, DASHBOARD_REFRESH_INTERVALS, 30_000)).toBe(30_000);
    expect(normalizeRefreshInterval("30000", DASHBOARD_REFRESH_INTERVALS, 30_000)).toBe(30_000);
    expect(normalizeRefreshInterval(0, MONITOR_REFRESH_INTERVALS, 5_000)).toBe(5_000);
    expect(normalizeRefreshInterval(null, MONITOR_REFRESH_INTERVALS, 5_000)).toBe(5_000);
    expect(normalizeRefreshInterval(undefined, DASHBOARD_REFRESH_INTERVALS, 30_000)).toBe(30_000);
  });

  it("formats seconds and minutes for the status label", () => {
    expect(formatRefreshInterval(5_000)).toBe("5s");
    expect(formatRefreshInterval(30_000)).toBe("30s");
    expect(formatRefreshInterval(60_000)).toBe("1m");
    expect(formatRefreshInterval(900_000)).toBe("15m");
  });
});
