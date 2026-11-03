import { describe, expect, it } from "vitest";
import {
  avgValue,
  errorLevel,
  errorRatioSeries,
  groupStatusClasses,
  mergeInstances,
  mergePoints,
  peakValue,
  podSumLatest,
  serverErrorRatio,
  shortPodNames,
  statusClassOf,
  statusTotals,
  sumKeyedSeries,
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
      dead: { goroutines: pts([1, 1]), heapMB: [], cpuPercent: [], threads: [] },
    };
    const out = mergeInstances(prev, { live: { goroutines: pts([2, 5]) } }, 0);
    expect(Object.keys(out)).toEqual(["live"]);
    expect(out.live.goroutines).toEqual(pts([2, 5]));
    expect(out.live.heapMB).toEqual([]);
  });
});

describe("pod aggregation", () => {
  const instances: Record<string, InstanceState> = {
    a: { goroutines: pts([60, 10], [120, 12]), heapMB: [], cpuPercent: pts([60, 5]), threads: [] },
    b: { goroutines: pts([120, 8]), heapMB: [], cpuPercent: pts([120, 7]), threads: [] },
    stale: { goroutines: pts([60, 100]), heapMB: [], cpuPercent: [], threads: [] },
  };

  it("sums latest values of active pods only", () => {
    expect(podSumLatest(instances, "goroutines", 120, 30)).toBe(20);
  });

  it("sums per timestamp across pods", () => {
    expect(sumPodSeries(instances, "goroutines")).toEqual(pts([60, 110], [120, 20]));
  });

  it("sums keyed series per timestamp", () => {
    expect(sumKeyedSeries({ x: pts([1, 2], [2, 3]), y: pts([2, 4]) })).toEqual(pts([1, 2], [2, 7]));
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

describe("status classes", () => {
  it("classifies codes", () => {
    expect(statusClassOf("200")).toBe("2xx");
    expect(statusClassOf("302")).toBe("3xx");
    expect(statusClassOf("429")).toBe("4xx");
    expect(statusClassOf("502")).toBe("5xx");
    expect(statusClassOf("-1")).toBe("other");
    expect(statusClassOf("0")).toBe("other");
    expect(statusClassOf("101")).toBe("other");
  });

  const codes = {
    "200": pts([1, 90], [2, 80]),
    "201": pts([1, 5], [2, 0]),
    "429": pts([1, 3], [2, 0]),
    "500": pts([1, 1], [2, 0]),
    "502": pts([1, 1], [2, 20]),
    "304": pts([1, 0], [2, 0]),
  };

  it("folds codes into classes and omits classes that never had traffic", () => {
    const grouped = groupStatusClasses(codes);
    expect(Object.keys(grouped)).toEqual(["2xx", "4xx", "5xx"]);
    expect(grouped["2xx"]).toEqual(pts([1, 95], [2, 80]));
    expect(grouped["5xx"]).toEqual(pts([1, 2], [2, 20]));
  });

  it("totals window requests and computes 5xx ratio", () => {
    const totals = statusTotals(codes);
    expect(totals).toEqual({ total: 200, c4xx: 3, c5xx: 22 });
    expect(serverErrorRatio(totals)).toBeCloseTo(0.11);
    expect(serverErrorRatio({ total: 0, c4xx: 0, c5xx: 0 })).toBeNull();
  });

  it("builds a per-bucket 5xx ratio and treats empty buckets as zero", () => {
    const series = errorRatioSeries({ "200": pts([1, 98], [2, 0]), "500": pts([1, 2], [2, 0]) });
    expect(series).toEqual(pts([1, 0.02], [2, 0]));
  });

  it("grades the error ratio", () => {
    expect(errorLevel(null)).toBe("ok");
    expect(errorLevel(0.005)).toBe("ok");
    expect(errorLevel(0.01)).toBe("warn");
    expect(errorLevel(0.05)).toBe("bad");
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
