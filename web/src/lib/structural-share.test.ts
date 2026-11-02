import { describe, expect, it } from "vitest";
import { replaceEqualDeep } from "./structural-share";

describe("replaceEqualDeep", () => {
  it("returns prev when the whole tree is equal", () => {
    const prev = { a: [{ time: 1, v: 2 }], b: { c: "x" } };
    const next = { a: [{ time: 1, v: 2 }], b: { c: "x" } };
    expect(replaceEqualDeep(prev, next)).toBe(prev);
  });

  it("keeps unchanged subtrees and replaces only the changed branch", () => {
    const prev = { qps: [{ time: 1, v: 1 }], heap: [{ time: 1, v: 9 }] };
    const next = { qps: [{ time: 1, v: 1 }], heap: [{ time: 1, v: 10 }] };
    const out = replaceEqualDeep(prev, next);
    expect(out).not.toBe(prev);
    expect(out).toEqual(next);
    expect(out.qps).toBe(prev.qps);
    expect(out.heap).not.toBe(prev.heap);
  });

  it("reuses equal array items when the array grows", () => {
    const p0 = { time: 1, v: 1 };
    const prev = [p0];
    const out = replaceEqualDeep(prev, [
      { time: 1, v: 1 },
      { time: 2, v: 3 },
    ]);
    expect(out).not.toBe(prev);
    expect(out[0]).toBe(p0);
    expect(out).toHaveLength(2);
  });

  it("detects removed and added keys", () => {
    const prev: Record<string, number> = { a: 1, b: 2 };
    expect(replaceEqualDeep(prev, { a: 1 })).toEqual({ a: 1 });
    expect(replaceEqualDeep(prev, { a: 1, c: 2 })).toEqual({ a: 1, c: 2 });
    expect(replaceEqualDeep({ a: undefined }, { b: undefined })).toEqual({ b: undefined });
  });

  it("falls back to next for primitives and type changes", () => {
    expect(replaceEqualDeep(1, 2)).toBe(2);
    expect(replaceEqualDeep(undefined, [1])).toEqual([1]);
    expect(replaceEqualDeep([1], { 0: 1 })).toEqual({ 0: 1 });
    expect(replaceEqualDeep(Number.NaN, Number.NaN)).toBeNaN();
  });
});
