import { describe, expect, it } from "vitest";

import { GRANULARITY_STEP_MS, MAX_CHART_POINTS, deriveGranularity } from "./time-range";

const HOUR = 3_600_000;
const DAY = 24 * HOUR;

describe("deriveGranularity", () => {
  it("预设区间取不超过点数上限的最细桶，长区间不再只剩个位数个点", () => {
    expect(deriveGranularity(HOUR)).toBe("minute");
    expect(deriveGranularity(DAY)).toBe("10minute");
    expect(deriveGranularity(7 * DAY)).toBe("hour");
    expect(deriveGranularity(30 * DAY)).toBe("4hour");
  });

  it("自定义区间同样按点数上限挑档，超长区间回落到周", () => {
    expect(deriveGranularity(12 * HOUR)).toBe("5minute");
    expect(deriveGranularity(3 * DAY)).toBe("30minute");
    expect(deriveGranularity(90 * DAY)).toBe("day");
    expect(deriveGranularity(5 * 365 * DAY)).toBe("week");
  });

  it("任意区间的桶数都不超过上限（周档除外）", () => {
    for (const span of [HOUR, 6 * HOUR, DAY, 2 * DAY, 7 * DAY, 14 * DAY, 30 * DAY, 180 * DAY]) {
      const g = deriveGranularity(span);
      expect(span / GRANULARITY_STEP_MS[g]).toBeLessThanOrEqual(MAX_CHART_POINTS);
    }
  });
});
