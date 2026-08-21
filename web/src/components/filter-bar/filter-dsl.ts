/**
 * filter DSL 序列化/解析，与 internal/common/filter/parser.go 对齐：
 *   field:value field2:a|b field3:"含空格值"
 * 同 key 多 token 序列化合并为 key:v1|v2；值含空格/|/" 时整体加双引号
 * （后端 splitValues 对整体引号包裹的段按单值字面量处理，不再按 | 拆分）。
 */

export interface FilterToken {
  /** facet key；自由文本（关键词）token 为 null，不进入 DSL */
  key: string | null;
  value: string;
}

function escapeValue(value: string): string {
  if (/[\s|"]/.test(value)) {
    return `"${value.replace(/"/g, "")}"`;
  }
  return value;
}

export function serializeTokens(tokens: FilterToken[]): string | undefined {
  const groups = new Map<string, string[]>();
  for (const token of tokens) {
    if (token.key === null) continue;
    const values = groups.get(token.key) ?? [];
    values.push(token.value);
    groups.set(token.key, values);
  }
  if (groups.size === 0) return undefined;
  return Array.from(groups, ([key, values]) => `${key}:${values.map(escapeValue).join("|")}`).join(
    " ",
  );
}

/** 按空格切分表达式，保留引号内空格（移植 Go splitExpression） */
function splitExpression(expr: string): string[] {
  const parts: string[] = [];
  let current = "";
  let inQuote = false;
  for (const ch of expr) {
    if (ch === '"') {
      inQuote = !inQuote;
      current += ch;
    } else if (ch === " " && !inQuote) {
      if (current) {
        parts.push(current);
        current = "";
      }
    } else {
      current += ch;
    }
  }
  if (current) parts.push(current);
  return parts;
}

function splitValues(raw: string): string[] {
  const trimmed = raw.trim();
  if (trimmed.length >= 2 && trimmed.startsWith('"') && trimmed.endsWith('"')) {
    return [trimmed.slice(1, -1)];
  }
  return trimmed
    .split("|")
    .map((p) => p.trim().replace(/^"|"$/g, ""))
    .filter((p) => p !== "");
}

/** UI 不生成的比较/取反操作符（值首字符命中即视为高级语法，恢复时丢弃） */
const OPERATOR_PREFIX = /^[!><=]/;

/**
 * 解析 filter DSL 为 token 列表（持久化恢复/容错用）。
 * 未知 key、非法片段、含比较操作符的片段直接丢弃，其余保留。
 */
export function parseFilterString(expr: string, knownKeys: ReadonlySet<string>): FilterToken[] {
  const tokens: FilterToken[] = [];
  for (const part of splitExpression(expr.trim())) {
    const idx = part.indexOf(":");
    if (idx <= 0) continue;
    const key = part.slice(0, idx);
    if (!knownKeys.has(key)) continue;
    const rawValue = part.slice(idx + 1);
    if (OPERATOR_PREFIX.test(rawValue)) continue;
    for (const value of splitValues(rawValue)) {
      tokens.push({ key, value });
    }
  }
  return tokens;
}

/**
 * 旧 localStorage 筛选数据迁移为 token 列表（纯函数）。
 * legacy 的 value 为 string[]（facet 值）或 string（关键词，key 用 "keyword"）。
 */
export function migrateLegacyTokens(legacy: Record<string, unknown>): FilterToken[] {
  const tokens: FilterToken[] = [];
  for (const [key, value] of Object.entries(legacy)) {
    if (key === "keyword") {
      if (typeof value === "string" && value !== "") tokens.push({ key: null, value });
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        if (typeof item === "string" && item !== "") tokens.push({ key, value: item });
      }
    }
  }
  return tokens;
}
