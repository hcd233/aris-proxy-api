import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 源码契约：`<Table fit>` 的声明列宽必须放得下「最宽语言表头 + 排序图标 + 内边距」。
 *
 * 回归背景（2026-11-05）：sessions 的 MESSAGES / TOOLS 列声明 3.5rem（56px），
 * 英文表头实测 65.3 / 40.8px、日文 76.8 / 46.1px，而 `Table fit` 只裁 td 不裁 th，
 * 表头直接压到相邻列。时间戳列同理：`YYYY/MM/DD HH:mm:ss` 实测 175.6px，
 * 10.5rem（168px）会把秒静默裁掉。
 *
 * 为什么是文本断言：jsdom 没有布局引擎，表头溢出只能在真实浏览器里量。这里用
 * Maple Mono CN 的等宽度量（ASCII 0.6125em / CJK 1.225em / letter-spacing 0.08em，
 * 与 `TableHead` 的 text-xs + tracking-[0.08em] 及 px-3 对齐）复算三语宽度，
 * 再和源码里的 `w-*` 声明比较。实测值与公式的偏差在 1px 内，容差 2px。
 *
 * 已知漏报范围（按「宁漏报不误报」接受）：
 * - 非 `t("key")` 的模板字符串 / 动态表头（如 `t(\`dashboard.dim.${dim}\`)`）；
 * - 表头内含自定义 chrome（Tooltip pill、Badge）时只按纯文本估，可能低估；
 * - `sr-only` 操作列表头（宽度由按钮组合决定，见 upstream 的专项预算测试）。
 */
const WEB_ROOT = fileURLToPath(new URL("../..", import.meta.url));
const SRC_DIR = join(WEB_ROOT, "src");
const LOCALES = ["en", "zh", "ja"] as const;
type Locale = (typeof LOCALES)[number];

const TRANSLATIONS: Record<Locale, Record<string, string>> = Object.fromEntries(
  LOCALES.map((locale) => [
    locale,
    JSON.parse(readFileSync(join(WEB_ROOT, "src/locales", `${locale}.json`), "utf8")),
  ]),
) as Record<Locale, Record<string, string>>;

/** 表头数据里「日期时间」列的最小宽度：`YYYY/MM/DD HH:mm:ss` 实测 175.6px + 2px 容差 */
const DATETIME_COL_PX = 176;
const DATETIME_KEYS = new Set([
  "sessions.time",
  "audit.time",
  "demo_access_audit.time",
  "cron.updated_at",
]);

/** 排序箭头（ArrowUp/Down size-3 + gap-1）占的横向空间 */
const SORT_ICON_PX = 16;
/**
 * flat-view 的 `SortableHead` 用 Button 承载表头：按钮 px-1 / 图标 gap-1 /
 * icon 12px / pr-1，再加 `-ml-2` 的 4px 左移与 th 的 px-3 右内边距，
 * 折算成「文字之外」的 42px。
 */
const SORT_BUTTON_CHROME_PX = 42;

const LATIN_EM = 0.6125;
const CJK_EM = 1.225;
const LETTER_SPACING_EM = 0.08;

function isCjk(char: string): boolean {
  return /[\u2e80-\u9fff\uf900-\ufaff\uff00-\uffef\u3040-\u30ff]/.test(char);
}

/** 与浏览器实测对齐的等宽字体宽度（含 letter-spacing） */
function textWidth(text: string, fontSizePx: number): number {
  const latin = fontSizePx * LATIN_EM;
  const cjk = fontSizePx * CJK_EM;
  const spacing = fontSizePx * LETTER_SPACING_EM;
  const chars = [...text];
  return chars.reduce((width, char, index) => {
    const advance = isCjk(char) ? cjk : latin;
    return width + advance + (index < chars.length - 1 ? spacing : 0);
  }, 0);
}

function collectTsxFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...collectTsxFiles(full));
    } else if (entry.name.endsWith(".tsx")) {
      out.push(full);
    }
  }
  return out;
}

/** 从 `<Tag` 的起始位置找到开标签的 `>`（跳过引号/花括号内的 `>`） */
function openingTagEnd(source: string, start: number): number {
  let quote: string | null = null;
  let depth = 0;
  for (let i = start; i < source.length; i += 1) {
    const char = source[i];
    if (quote) {
      if (char === "\\") i += 1;
      else if (char === quote) quote = null;
      continue;
    }
    if (char === '"' || char === "'" || char === "`") quote = char;
    else if (char === "{" || char === "(") depth += 1;
    else if (char === "}" || char === ")") depth -= 1;
    else if (char === ">" && depth === 0) return i;
  }
  return -1;
}

interface ParsedHeader {
  line: number;
  tag: string;
  content: string;
  /** `TableHead`（普通文本表头）或 `SortableHead`（Button + 排序图标） */
  kind: "plain" | "button";
}

function parseHeaders(source: string): ParsedHeader[] {
  const headers: ParsedHeader[] = [];
  const patterns: { regex: RegExp; kind: ParsedHeader["kind"]; closing: string }[] = [
    { regex: /<TableHead\b/g, kind: "plain", closing: "</TableHead>" },
    { regex: /<SortableHead\b/g, kind: "button", closing: "</SortableHead>" },
  ];
  for (const { regex, kind, closing } of patterns) {
    let match: RegExpExecArray | null;
    while ((match = regex.exec(source))) {
      const tagEnd = openingTagEnd(source, match.index);
      if (tagEnd === -1) continue;
      const tag = source.slice(match.index, tagEnd + 1);
      const closeIndex = source.indexOf(closing, tagEnd);
      const content = closeIndex === -1 ? "" : source.slice(tagEnd + 1, closeIndex);
      // 自闭合的 TableHead 没有子节点，宽度不在这里声明；SortableHead 的调用点
      // 恰好是自闭合的，label / className 都在属性里，保留下来
      if (kind === "plain" && tag.trimEnd().endsWith("/>")) continue;
      headers.push({
        line: source.slice(0, match.index).split("\n").length,
        tag,
        content,
        kind,
      });
    }
  }
  return headers;
}

function classLiterals(tag: string): string[] {
  return [...tag.matchAll(/"([^"]*)"|'([^']*)'|`([^`]*)`/g)].map(
    (match) => match[1] ?? match[2] ?? match[3] ?? "",
  );
}

function declaredWidthPx(tag: string): number | null {
  const literals = classLiterals(tag).join(" ");
  const arbitrary = literals.match(/(?<![-\w])w-\[([\d.]+)(rem|px)\]/);
  if (arbitrary) {
    const value = Number(arbitrary[1]);
    return arbitrary[2] === "rem" ? value * 16 : value;
  }
  const scale = literals.match(/(?<![-\w])w-(\d+(?:\.\d+)?)\b/);
  if (scale) return Number(scale[1]) * 4;
  return null;
}

function paddingPx(tag: string): number {
  const literals = classLiterals(tag).join(" ");
  const explicit = literals.match(/(?<![-\w])px-(\d+(?:\.\d+)?)\b/);
  if (explicit) return Number(explicit[1]) * 4;
  return tag.includes("DENSE_HEAD") ? 4 : 12;
}

function fontSizePx(tag: string): number {
  if (tag.includes("DENSE_HEAD")) return 11;
  const explicit = tag.match(/text-\[(\d+(?:\.\d+)?)px\]/);
  return explicit ? Number(explicit[1]) : 12;
}

function isSortable(header: ParsedHeader): boolean {
  if (header.kind === "button") return true;
  return /(?:^|\s)onClick=|cursor-pointer/.test(header.tag);
}

function labelKeys(text: string): string[] {
  return [...text.matchAll(/t\(\s*"([\w.]+)"\s*\)/g)].map((match) => match[1]);
}

interface Finding {
  file: string;
  line: number;
  width: number;
  need: number;
  text: string;
  key: string;
  locale: Locale;
}

function auditFile(file: string): Finding[] {
  const source = readFileSync(file, "utf8");
  if (!source.includes("<Table fit")) return [];
  const findings: Finding[] = [];
  for (const header of parseHeaders(source)) {
    const width = declaredWidthPx(header.tag);
    if (width === null) continue;

    // 可见文案只取表头自身 / 子排序按钮的 label；Tooltip 提示与 aria-label 不是列名
    const visibleContent = header.content
      .replace(/<TooltipContent[\s\S]*?<\/TooltipContent>/g, "")
      .replace(/\b(?:aria-label|alt|title|hint)=\{[^}]*\}/g, "")
      .replace(/\b(?:aria-label|alt|title|hint)="[^"]*"/g, "");
    const labelSource = `${header.tag} ${visibleContent}`;
    const keys = labelKeys(labelSource).filter(
      (key) => !key.endsWith("_aria") && !key.includes("select_") && !key.includes("sr_only"),
    );
    // 操作列用 sr-only 文本占位，列宽由按钮组合决定，另有专项预算
    if (header.content.includes("sr-only")) continue;
    if (keys.length === 0) continue;

    const fontSize = fontSizePx(header.tag);
    const padding = paddingPx(header.tag);
    // SortableHead 的 42px 已含 th 右内边距与 -ml-2 的偏移，不再叠加 padding
    const chrome =
      header.kind === "button" ? SORT_BUTTON_CHROME_PX : isSortable(header) ? SORT_ICON_PX : 0;
    const paddingTotal = header.kind === "button" ? 0 : padding * 2;

    for (const key of keys) {
      let worst: { loc: Locale; text: string; need: number } | null = null;
      for (const locale of LOCALES) {
        const text = TRANSLATIONS[locale][key];
        if (!text) continue;
        const need = textWidth(text, fontSize) + paddingTotal + chrome;
        if (!worst || need > worst.need) worst = { loc: locale, text, need };
      }
      if (!worst) continue;
      const need = Math.max(worst.need, DATETIME_KEYS.has(key) ? DATETIME_COL_PX : 0);
      // 2px 容差：等宽度量与 DOM 的亚像素舍入差
      if (width < need - 2) {
        findings.push({
          file,
          line: header.line,
          width,
          need,
          text: worst.text,
          key,
          locale: worst.loc,
        });
      }
    }
  }
  return findings;
}

describe("Table fit 表头列宽预算", () => {
  it("所有声明宽度的表头都放得下最宽语言（含排序图标与内边距）", () => {
    const findings = [
      ...collectTsxFiles(join(SRC_DIR, "app")),
      ...collectTsxFiles(join(SRC_DIR, "components")),
    ].flatMap(auditFile);

    const report = findings
      .map(
        (f) =>
          `${f.file.replace(`${WEB_ROOT}/`, "")}:${f.line}  "${f.text}" (${f.locale}) 需要 ${f.need.toFixed(1)}px，声明 ${f.width}px`,
      )
      .join("\n");

    expect(findings, `\n${report}\n`).toEqual([]);
  });
});
