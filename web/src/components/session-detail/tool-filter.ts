import type { ToolItem } from "@/lib/types";

/** 按工具名做不区分大小写的子串匹配；空白查询返回原列表 */
export function filterToolsByName(tools: ToolItem[], query: string): ToolItem[] {
  const q = query.trim().toLowerCase();
  if (!q) return tools;
  return tools.filter((item) => (item.tool.name ?? "").toLowerCase().includes(q));
}
