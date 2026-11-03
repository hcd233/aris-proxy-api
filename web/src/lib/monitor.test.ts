import { describe, expect, it } from "vitest";
import {
  avgValue,
  mergeInstances,
  mergePoints,
  peakValue,
  podSumLatest,
  seriesMapChartData,
  shortPodNames,
  sumPodSeries,
  type InstanceState,
} from "./monitor";

const pts = (...pairs: Array<[number, number]>) => pairs.map(([time, value]) => ({ time, value }));

describe("mergePoints", () => {
  it("overrides by time, drops points before cutoff and sorts ascending", () => {
    const merged = mergePoints(pts([10, 1], [20, 2]), pts([20, 9], [30, 3], [5, 7]), 10);
    expect(merged).toEqual(pts([10, 1], [20, 9], [30, 3]));
  });
});

describe("mergeInstances", () => {
  it("drops instances missing from the incoming response", () => {
    const prev: Record<string, InstanceState> = {
      dead: { goroutines: pts([1, 1]), heapMB: [], cpuPercent: [] },
    };
    const out = mergeInstances(prev, { live: { goroutines: pts([2, 5]) } }, 0);
    expect(Object.keys(out)).toEqual(["live"]);
    expect(out.live.goroutines).toEqual(pts([2, 5]));
    expect(out.live.heapMB).toEqual([]);
  });
});

describe("pod aggregation", () => {
  const instances: Record<string, InstanceState> = {
    a: { goroutines: pts([60, 10], [120, 12]), heapMB: [], cpuPercent: pts([60, 5]) },
    b: { goroutines: pts([120, 8]), heapMB: [], cpuPercent: pts([120, 7]) },
    stale: { goroutines: pts([60, 100]), heapMB: [], cpuPercent: [] },
  };

  it("sums latest values of active pods only", () => {
    expect(podSumLatest(instances, "goroutines", 120, 30)).toBe(20);
  });

  it("sums per timestamp across pods", () => {
    expect(sumPodSeries(instances, "goroutines")).toEqual(pts([60, 110], [120, 20]));
  });
});

describe("seriesMapChartData", () => {
  it("lines up rx / tx on one time axis", () => {
    expect(seriesMapChartData({ rx: pts([1, 10], [2, 20]), tx: pts([2, 5]) })).toEqual([
      { time: 1, rx: 10 },
      { time: 2, rx: 20, tx: 5 },
    ]);
  });
});

describe("series stats", () => {
  it("handles empty input", () => {
    expect(peakValue([])).toBe(0);
    expect(avgValue([])).toBe(0);
  });
  it("computes peak and average", () => {
    expect(peakValue(pts([1, 2], [2, 6]))).toBe(6);
    expect(avgValue(pts([1, 2], [2, 6]))).toBe(4);
  });
});

describe("shortPodNames", () => {
  it("strips the shared prefix", () => {
    expect(shortPodNames(["api-7d9f-x2k4j", "api-7d9f-m8q1z"])).toEqual({
      "api-7d9f-x2k4j": "x2k4j",
      "api-7d9f-m8q1z": "m8q1z",
    });
  });
  it("keeps a single pod untouched", () => {
    expect(shortPodNames(["api-7d9f-x2k4j"])).toEqual({ "api-7d9f-x2k4j": "api-7d9f-x2k4j" });
  });
  it("never consumes a whole name", () => {
    expect(shortPodNames(["x-y-1", "x-z"])).toEqual({ "x-y-1": "y-1", "x-z": "z" });
  });
});
