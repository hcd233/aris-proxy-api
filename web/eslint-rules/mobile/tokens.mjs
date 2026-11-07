/**
 * 移动端规则共用的 Tailwind v4 类名解析。
 *
 * 只做本组规则需要的最小解析：拆变体、识别「手机上会生效」的变体、把尺寸换算成 px。
 * 无法静态换算的值（CSS 变量、calc、百分比……）返回 null，调用方一律放行。
 */

import { classAttributes, collectStringLiterals } from "../class-literals.mjs";

/** 规范的设计基线：360 视口 − 左右各 16px 页面内边距 ≈ 328，取整到 w-80。 */
export const PHONE_CONTENT_MAX_PX = 320;
/** 触控目标下限（WCAG 2.2 SC 2.5.8 AA）。 */
export const MIN_TOUCH_TARGET_PX = 24;
/** iOS Safari 聚焦字号 < 16px 的输入框时会自动放大页面。 */
export const MIN_INPUT_FONT_PX = 16;

/** 在方括号/圆括号外按 `:` 拆分：`md:[&>a]:hover:opacity-100` → 变体 + 工具类。 */
export function parseToken(raw) {
  const parts = [];
  let depth = 0;
  let cur = "";
  for (const ch of raw) {
    if (ch === "[" || ch === "(") depth++;
    else if (ch === "]" || ch === ")") depth--;
    if (ch === ":" && depth === 0) {
      parts.push(cur);
      cur = "";
    } else cur += ch;
  }
  const utility = cur.replace(/^!|!$/g, "");
  return { raw, variants: parts, utility };
}

/** 元素上所有 className 类属性里静态可见的类名 token。 */
export function elementTokens(opening) {
  const literals = [];
  for (const attr of classAttributes(opening)) collectStringLiterals(attr.value, literals);
  return literals.flatMap((s) => s.split(/\s+/).filter(Boolean).map(parseToken));
}

export const isCoarse = (v) => v === "pointer-coarse" || v === "any-pointer-coarse";
export const isFine = (v) => v === "pointer-fine" || v === "any-pointer-fine";
export const isHover = (v) => /^(group-|peer-)?hover(\/[\w-]+)?$/.test(v);
/** 视口/容器断点变体（含 max-* 与 @ 容器查询）。 */
export const isResponsive = (v) => /^(sm|md|lg|xl|2xl|(min|max)-.+|@.+)$/.test(v);

/**
 * 该 token 在 ≤360px 的触屏手机上会不会生效：无变体，或只带 max-* / @max-* / dark /
 * 粗指针变体。`sm:` / `md:` 等最小宽度断点在手机上不生效。
 */
export function appliesOnPhone(token) {
  return token.variants.every((v) => /^(@?max-.+)$/.test(v) || v === "dark" || isCoarse(v));
}

/** `320px` / `20rem` / `1.25em` → px；其它单位返回 null。 */
export function lengthToPx(value) {
  const m = /^(-?\d*\.?\d+)(px|rem|em)$/.exec(value.trim());
  if (!m) return null;
  return Number(m[1]) * (m[2] === "px" ? 1 : 16);
}

const CONTAINER_PX = {
  "3xs": 256,
  "2xs": 288,
  xs: 320,
  sm: 384,
  md: 448,
  lg: 512,
  xl: 576,
  "2xl": 672,
  "3xl": 768,
  "4xl": 896,
  "5xl": 1024,
  "6xl": 1152,
  "7xl": 1280,
};

/**
 * 尺寸类的值 → px：间距刻度 `80` → 320、`px` → 1、任意值 `[20rem]` → 320；
 * `allowContainer` 时额外识别 `w-sm` 一类容器刻度。其余返回 null。
 */
export function sizeValueToPx(value, { allowContainer = false } = {}) {
  if (/^\d*\.?\d+$/.test(value)) return Number(value) * 4;
  if (value === "px") return 1;
  const arb = /^\[(.+)\]$/.exec(value);
  if (arb) return lengthToPx(arb[1]);
  if (allowContainer && value in CONTAINER_PX) return CONTAINER_PX[value];
  return null;
}

const FONT_PX = { xs: 12, sm: 14, base: 16, lg: 18, xl: 20, "2xl": 24, "3xl": 30, "4xl": 36 };

/** `text-sm` / `text-sm/6` / `text-[15px]` → px；颜色等非字号类返回 null。 */
export function fontSizePx(utility) {
  const m = /^text-([^/]+)(\/.+)?$/.exec(utility);
  if (!m) return null;
  if (m[1] in FONT_PX) return FONT_PX[m[1]];
  if (/^[5-9]xl$/.test(m[1])) return 48;
  const arb = /^\[(.+)\]$/.exec(m[1]);
  return arb ? lengthToPx(arb[1]) : null;
}
