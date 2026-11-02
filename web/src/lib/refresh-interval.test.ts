import { describe, expect, it } from "vitest";
import {
  DASHBOARD_REFRESH_INTERVALS,
  MONITOR_REFRESH_INTERVALS,
  REFRESH_OFF,
  formatRefreshInterval,
  normalizeRefreshInterval,
} from "./refresh-interval";

describe("refresh intervals", () => {
  it("keeps options sorted, unique and above 1s", () => {
    for (const options of [DASHBOARD_REFRESH_INTERVALS, MONITOR_REFRESH_INTERVALS]) {
      expect([...options]).toEqual([...options].sort((a, b) => a - b));
      expect(new Set(options).size).toBe(options.length);
      expect(options.every((ms) => ms >= 1_000)).toBe(true);
    }
  });

  it("accepts off or a listed interval and falls back otherwise", () => {
    expect(normalizeRefreshInterval(REFRESH_OFF, MONITOR_REFRESH_INTERVALS, 5_000)).toBe(0);
    expect(normalizeRefreshInterval(30_000, MONITOR_REFRESH_INTERVALS, 5_000)).toBe(30_000);
    expect(normalizeRefreshInterval(5_000, DASHBOARD_REFRESH_INTERVALS, 0)).toBe(5_000);
    expect(normalizeRefreshInterval(3_000, DASHBOARD_REFRESH_INTERVALS, 0)).toBe(0);
    expect(normalizeRefreshInterval("30000", DASHBOARD_REFRESH_INTERVALS, 0)).toBe(0);
    expect(normalizeRefreshInterval(null, MONITOR_REFRESH_INTERVALS, 5_000)).toBe(5_000);
  });

  it("formats short labels", () => {
    expect(formatRefreshInterval(5_000)).toBe("5s");
    expect(formatRefreshInterval(30_000)).toBe("30s");
    expect(formatRefreshInterval(60_000)).toBe("1m");
    expect(formatRefreshInterval(300_000)).toBe("5m");
  });
});
