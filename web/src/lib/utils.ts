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
  return new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(
    value,
    unit,
  );
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
 * 本地时间 `YYYY/MM/DD HH:mm:ss`。非法日期原样返回，避免渲染 "NaN/NaN/NaN"。
 */
export function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  const year = d.getFullYear();
  const month = d.getMonth() + 1;
  const day = d.getDate();
  const hours = String(d.getHours()).padStart(2, "0");
  const minutes = String(d.getMinutes()).padStart(2, "0");
  const seconds = String(d.getSeconds()).padStart(2, "0");
  return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
}
