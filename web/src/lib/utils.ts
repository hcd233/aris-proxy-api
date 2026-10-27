import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** 相对时间单位：按秒划分，用于 Intl.RelativeTimeFormat 分档。 */
const RELATIVE_TIME_UNITS: Array<[Intl.RelativeTimeFormatUnit, number]> = [
  ["minute", 60],
  ["hour", 60 * 60],
  ["day", 24 * 60 * 60],
  ["week", 7 * 24 * 60 * 60],
  ["month", 30.44 * 24 * 60 * 60],
  ["year", 365.25 * 24 * 60 * 60],
];

/**
 * 相对时间（"5 minutes ago" / "5分钟前"），跟随应用 locale 而非浏览器 locale。
 * <60s 时 Intl 返回 "now" / "现在"。
 */
export function formatRelativeTime(
  dateInput: string | number | Date,
  locale: string = "en",
): string {
  const date = new Date(dateInput);
  const diffSec = (date.getTime() - Date.now()) / 1000;
  const absSec = Math.abs(diffSec);

  let unit: Intl.RelativeTimeFormatUnit = "second";
  let value = Math.round(diffSec);
  for (const [u, secs] of RELATIVE_TIME_UNITS) {
    if (absSec < secs) break;
    unit = u;
    value = Math.round(diffSec / secs);
  }
  return new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(value, unit);
}

/**
 * 码点安全截断：超长文本截到 maxLen 个码点并追加省略号。
 * 用 Array.from 按 Unicode 码点切割，避免截断 emoji 等代理对字符。
 */
export function truncateText(text: string, maxLen: number): string {
  const chars = Array.from(text);
  if (chars.length <= maxLen) return text;
  return chars.slice(0, maxLen).join("") + "…";
}

/**
 * 占比格式化：value / total → "32.4%"（1 位小数）。total <= 0 返回 "0%"。
 */
export function formatPct(value: number, total: number): string {
  if (total <= 0) return "0%";
  return `${((value / total) * 100).toFixed(1)}%`;
}

/** 量级缩写档位：从大到小，满千进一级。 */
const COMPACT_UNITS = [
  { base: 1_000_000_000, suffix: "B" },
  { base: 1_000_000, suffix: "M" },
  { base: 1_000, suffix: "K" },
] as const;

/**
 * token 数量展示：按量级缩写为 K/M/B，最多 1 位小数并省略尾零
 * （`1_500 → "1.5K"`、`1_000_000 → "1M"`、`1_234_567_890 → "1.2B"）。
 * 四舍五入满千自动进一级（`999_960 → "1M"`，不会出现 `1000K`）；空值与非有限数渲染 `—`。
 */
export function formatTokenCount(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return "\u2014";
  const sign = value < 0 ? "-" : "";
  const abs = Math.abs(value);
  // 按四舍五入后的量级选档：999.5 会进到 K，不会输出没有单位的裸 "1000"
  const roundedAbs = Math.round(abs);
  if (roundedAbs < 1_000) return `${sign}${roundedAbs}`;

  const index = COMPACT_UNITS.findIndex((unit) => roundedAbs >= unit.base);
  const { base, suffix } = COMPACT_UNITS[index];
  const rounded = Math.round((abs / base) * 10) / 10;
  if (rounded >= 1_000 && index > 0) {
    const upper = COMPACT_UNITS[index - 1];
    return `${sign}${Math.round((abs / upper.base) * 10) / 10}${upper.suffix}`;
  }
  return `${sign}${rounded}${suffix}`;
}

// 数字分组在 en / zh / ja 下都是三位逗号，固定 locale 可避免 SSR/hydration 差异与切换语言时的宽度抖动
const INTEGER_FORMATTER = new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 });

/** 计数展示：千分位分组；null / undefined / 非有限数渲染 `—`。 */
export function formatInteger(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return "\u2014";
  return INTEGER_FORMATTER.format(value);
}

/** 小数展示：千分位 + 最多 maxDigits 位小数（默认 2），用于速率、占比等非计数数值。 */
export function formatDecimal(value: number, maxDigits = 2): string {
  if (!Number.isFinite(value)) return "\u2014";
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: maxDigits }).format(value);
}

/** 时长展示：<1s 用毫秒，≥1s 换算成秒（最多 2 位小数，省略尾零）；空值渲染 `—`。 */
export function formatDurationMs(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || !Number.isFinite(ms)) return "\u2014";
  const rounded = Math.round(ms);
  if (Math.abs(rounded) < 1_000) return `${rounded}ms`;
  return `${Math.round((ms / 1_000) * 100) / 100}s`;
}

/**
 * 本地日期 `YYYY/MM/DD`（月/日补零，跨页表格宽度可预测）。
 * 非法日期原样返回，避免渲染 "NaN/NaN/NaN"。
 */
export function formatDate(date: string | Date): string {
  const d = typeof date === "string" ? new Date(date) : date;
  if (Number.isNaN(d.getTime())) return typeof date === "string" ? date : "";
  const month = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}/${month}/${day}`;
}

/**
 * 本地时间 `YYYY/MM/DD HH:mm:ss`。非法日期原样返回，避免渲染 "NaN/NaN/NaN"。
 */
export function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  const hours = String(d.getHours()).padStart(2, "0");
  const minutes = String(d.getMinutes()).padStart(2, "0");
  const seconds = String(d.getSeconds()).padStart(2, "0");
  return `${formatDate(d)} ${hours}:${minutes}:${seconds}`;
}

/**
 * 分享过期时间：后端对「永久」使用 ~100 年后的哨兵值，直接显示 2126 年
 * 会让用户以为数据坏了；超过 50 年一律渲染 neverLabel。
 */
export function formatExpiry(dateStr: string, neverLabel: string): string {
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  if (d.getFullYear() > new Date().getFullYear() + 50) return neverLabel;
  return formatDateTime(dateStr);
}

/**
 * 定时任务展示名：后端存的是模块名（如 `ThinkExtractCron`），直接展示等于把内部标识符
 * 摆到界面上，这里按模块名推导 `cron.job_*` 文案。
 *
 * 后缀由模块名推导（`ThinkExtractCron` → `think_extract`），后端新增任务无需同步前端映射表；
 * 未命中的键由 t() 回退到原始模块名，不会渲染成空白。连续大写缩写（如 `APICleanupCron`）
 * 推导结果可能对不上文案，此时同样回退原文，不影响可用性。
 */
export function cronJobLabel(name: string, t: (key: string, fallback?: string) => string): string {
  const suffix = name
    .replace(/Cron$/, "")
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .toLowerCase();
  return suffix ? t(`cron.job_${suffix}`, name) : name;
}
