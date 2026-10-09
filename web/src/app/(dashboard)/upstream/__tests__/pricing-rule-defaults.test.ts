import { describe, expect, it } from "vitest";
import { emptyPricingRule, isDefaultRule } from "../pricing-rule";

describe("emptyPricingRule", () => {
  it("初始化 1h 缓存创建价为 0（回落 5m 档语义）", () => {
    expect(emptyPricingRule().cache_creation_1h_price).toBe(0);
    expect(emptyPricingRule().cache_creation_price).toBe(0);
  });

  it("初始规则仍是无条件默认规则", () => {
    expect(isDefaultRule(emptyPricingRule())).toBe(true);
  });

  it("四价 + 1h 价齐全", () => {
    const r = emptyPricingRule();
    expect(r.input_price).toBe(0);
    expect(r.output_price).toBe(0);
    expect(r.cache_read_price).toBe(0);
    expect(Object.keys(r).sort()).toEqual(
      [
        "cache_creation_1h_price",
        "cache_creation_price",
        "cache_read_price",
        "context_max",
        "context_min",
        "input_price",
        "output_price",
        "time_windows",
      ].sort(),
    );
  });
});
