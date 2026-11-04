/** 工具调用输出超过该行数时，额外提供侧板查看全文（官方 Tool · Long output 模式） */
export const LONG_OUTPUT_LINES = 200;

/** 工具输入 / 输出的展示形态：可解析 JSON 美化并标 json 高亮，否则原样纯文本 */
export function formatToolPayload(raw: string): { code: string; language: "json" | "" } {
  if (!raw) return { code: "", language: "" };
  try {
    return { code: JSON.stringify(JSON.parse(raw), null, 2), language: "json" };
  } catch {
    return { code: raw, language: "" };
  }
}

/** 折叠行描述：首个参数 `key: value`（字符串值加引号）；非 JSON 或空对象返回空串 */
export function previewFirstArg(argsJSON: string): string {
  if (!argsJSON) return "";
  try {
    const parsed = JSON.parse(argsJSON) as Record<string, unknown>;
    const entries = Object.entries(parsed ?? {});
    if (entries.length === 0) return "";
    const [key, value] = entries[0];
    const valStr = typeof value === "string" ? `"${value}"` : String(value);
    return `${key}: ${valStr}`;
  } catch {
    return "";
  }
}

/** 行数（忽略末尾换行）；空串为 0 */
export function countLines(s: string): number {
  if (!s) return 0;
  return s.replace(/\n$/, "").split("\n").length;
}
