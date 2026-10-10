import type { PricingRuleDTO } from "@/lib/types";

// 新增定价规则行的初值（cache_creation_1h_price=0 ⇔ 回落 5m 档价）
export const emptyPricingRule = (): PricingRuleDTO => ({
  time_windows: [],
  context_min: 0,
  context_max: 0,
  input_price: 0,
  output_price: 0,
  cache_creation_price: 0,
  cache_creation_1h_price: 0,
  cache_read_price: 0,
});

// 无条件默认规则：既无时段窗口也无上下文区间，是「这个模型多少钱」的代表档
export const isDefaultRule = (r: PricingRuleDTO): boolean =>
  (r.time_windows ?? []).length === 0 && !r.context_min && !r.context_max;
