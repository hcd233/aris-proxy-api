/**
 * 源码中允许出现的 <12px 任意值字号类。手机（<768px）下由 app/globals.css 统一提升到 12px，
 * 见 spec docs/superpowers/specs/2026-10-29-mobile-tablet-adapt-design.md §5.3。
 * 新增 <12px 的 text-[…] 类时必须登记在这里（app/mobile-type.test.ts 守护），并同步 globals.css。
 */
export const TINY_TEXT_CLASSES: readonly string[] = [
  "text-[8px]",
  "text-[9px]",
  "text-[10px]",
  "text-[11px]",
  "text-[11.5px]",
  "text-[0.65rem]",
];
