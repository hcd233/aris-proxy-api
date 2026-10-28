import { describe, expect, it } from "vitest";
import {
  cacheHitRate,
  deltaDiff,
  deltaPct,
  deltaTone,
  formatCostShort,
  formatDelta,
  foldTopModels,
  heatmapMatrix,
  KPI_SPECS,
  OTHER_KEY,
  peakBucket,
  rangeDays,
  successRate,
} from "./dashboard";
import type { DashboardModelPoint, DashboardStats, DashboardStatsPoint } from "./types";

function stats(partial: Partial<DashboardStats> = {}): DashboardStats {
  return {
    requests: 0,
    success: 0,
    failures: { rateLimit: 0, server: 0, connection: 0, other: 0 },
    tokens: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
    cost: 0,
    pricedSuccess: 0,
    latency: {},
    ...partial,
  };
}

describe("deltaPct", () => {
  it("returns null when both periods are empty", () => {
    expect(deltaPct(0, 0)).toBeNull();
  });
  it("returns null when previous period is zero", () => {
    expect(deltaPct(5, 0)).toBeNull();
  });
  it("computes relative change", () => {
    expect(deltaPct(112, 100)).toBeCloseTo(0.12);
  });
});

describe("deltaDiff", () => {
  it("returns null when either side is missing", () => {
    expect(deltaDiff(null, 3)).toBeNull();
    expect(deltaDiff(3, undefined)).toBeNull();
  });
  it("subtracts previous from current", () => {
    expect(deltaDiff(1820, 2030)).toBe(-210);
  });
});

describe("deltaTone", () => {
  it("treats rising cost as bad", () => {
    expect(deltaTone("cost", 0.2)).toBe("bad");
  });
  it("treats falling success rate as bad", () => {
    expect(deltaTone("successRate", -0.003)).toBe("bad");
  });
  it("treats falling latency as good", () => {
    expect(deltaTone("latency", -210)).toBe("good");
  });
  it("treats rising requests as good", () => {
    expect(deltaTone("requests", 0.1)).toBe("good");
  });
  it("is neutral without a delta", () => {
    expect(deltaTone("requests", null)).toBe("neutral");
    expect(deltaTone("cost", 0)).toBe("neutral");
  });
});

describe("rates", () => {
  it("computes cache hit rate over total prompt tokens", () => {
    expect(cacheHitRate({ input: 60, output: 5, cacheRead: 40, cacheWrite: 0 })).toBe(0.4);
  });
  it("returns null hit rate without prompt tokens", () => {
    expect(cacheHitRate({ input: 0, output: 0, cacheRead: 0, cacheWrite: 0 })).toBeNull();
  });
  it("returns null success rate without requests", () => {
    expect(successRate(stats())).toBeNull();
    expect(successRate(stats({ requests: 4, success: 3 }))).toBe(0.75);
  });
});

describe("rangeDays / peakBucket", () => {
  it("measures range in days with a one-hour floor", () => {
    expect(rangeDays("2026-10-21T00:00:00Z", "2026-10-28T00:00:00Z")).toBe(7);
    expect(rangeDays("2026-10-28T00:00:00Z", "2026-10-28T00:10:00Z")).toBeCloseTo(1 / 24);
  });
  it("finds the busiest bucket", () => {
    const series = [3, 9, 4].map((n, i): DashboardStatsPoint => ({
      time: `2026-10-2${i}T00:00:00Z`,
      ...stats({ requests: n }),
    }));
    expect(peakBucket(series)).toBe(9);
    expect(peakBucket([])).toBe(0);
  });
});

describe("foldTopModels", () => {
  const times = ["2026-10-21T00:00:00Z", "2026-10-22T00:00:00Z"];
  const rows: DashboardModelPoint[] = [];
  for (let m = 0; m < 7; m++) {
    for (const time of times) {
      rows.push({ modelId: `m${m}`, time, requests: 10 - m, tokens: m * 100, cost: m });
    }
  }

  it("keeps top N and folds the rest into other", () => {
    const { keys, data } = foldTopModels(rows, "cost", 5);
    expect(keys).toHaveLength(6);
    expect(keys[keys.length - 1]).toBe(OTHER_KEY);
    expect(keys[0]).toBe("m6");
    const rawTotal = rows.filter((r) => r.time === times[0]).reduce((s, r) => s + r.cost, 0);
    const point = data[0];
    const folded = keys.reduce((s, k) => s + (point[k] ?? 0), 0);
    expect(folded).toBe(rawTotal);
  });

  it("re-ranks when metric changes", () => {
    expect(foldTopModels(rows, "requests", 5).keys[0]).toBe("m0");
  });

  it("omits other when models fit", () => {
    const { keys } = foldTopModels(rows.slice(0, 4), "cost", 5);
    expect(keys).not.toContain(OTHER_KEY);
  });
});

describe("heatmapMatrix", () => {
  it("builds a 7x24 matrix with zero fill", () => {
    const m = heatmapMatrix([{ weekday: 6, hour: 23, count: 4 }]);
    expect(m).toHaveLength(7);
    expect(m.every((row) => row.length === 24)).toBe(true);
    expect(m[6][23]).toBe(4);
    expect(m[0][0]).toBe(0);
  });
  it("ignores out-of-range cells", () => {
    const m = heatmapMatrix([{ weekday: 9, hour: 30, count: 1 }]);
    expect(m.flat().reduce((a, b) => a + b, 0)).toBe(0);
  });
});

describe("formatDelta", () => {
  it("formats relative change with an arrow", () => {
    expect(formatDelta("requests", 0.124)).toBe("▲ 12.4%");
    expect(formatDelta("cost", -0.031)).toBe("▼ 3.1%");
  });
  it("formats rate changes as percentage points", () => {
    expect(formatDelta("successRate", -0.003)).toBe("▼ 0.3pt");
    expect(formatDelta("cacheHitRate", 0.042)).toBe("▲ 4.2pt");
  });
  it("formats latency changes as durations", () => {
    expect(formatDelta("latency", -210)).toBe("▼ 210ms");
    expect(formatDelta("latency", 1500)).toBe("▲ 1.5s");
  });
  it("renders a dash without a delta", () => {
    expect(formatDelta("requests", null)).toBe("—");
  });
});

describe("formatCostShort", () => {
  it("keeps two decimals at or above one unit", () => {
    expect(formatCostShort(184.318273, "USD")).toBe("$184.32");
  });
  it("keeps up to four significant decimals below one unit", () => {
    expect(formatCostShort(0.012345, "USD")).toBe("$0.0123");
  });
  it("uses thousands separators", () => {
    expect(formatCostShort(12345.6, "USD")).toBe("$12,345.6");
  });
});

describe("KPI_SPECS", () => {
  it("covers all six KPIs with labels present in every locale", async () => {
    const { readFileSync } = await import("node:fs");
    const { join } = await import("node:path");
    expect(KPI_SPECS.map((k) => k.kind)).toEqual([
      "requests",
      "tokens",
      "cost",
      "successRate",
      "cacheHitRate",
      "latency",
    ]);
    for (const loc of ["zh", "en", "ja"]) {
      const dict = JSON.parse(readFileSync(join(__dirname, "../locales", `${loc}.json`), "utf8"));
      for (const k of KPI_SPECS) expect(dict[k.labelKey], `${loc}:${k.labelKey}`).toBeTruthy();
    }
  });
});
