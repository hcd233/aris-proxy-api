/**
 * 金额展示工具。wire 层金额为展示单位浮点 + 币种；微单位只在后端与 DB。
 */

/** formatCost 金额展示：币种符号 + 去尾零（小数位上限 6），null/undefined 渲染 em dash。 */
export function formatCost(v: number | null | undefined, currency?: string): string {
  if (v === null || v === undefined) return "\u2014";
  const symbol = currency === "CNY" ? "\u00a5" : currency === "USD" ? "$" : "";
  const s = v.toFixed(6).replace(/\.?0+$/, "");
  return `${symbol}${s === "" ? "0" : s}`;
}
