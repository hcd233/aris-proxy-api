import { readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 源码契约：`<Table fit>`（table-layout: fixed + td overflow-hidden）里操作列的宽度预算。
 *
 * 回归背景（#198 引入、2026-10-19 修复）：操作列原为 7.5rem，装不下
 * 「编辑 + 定价 两个 icon-sm 按钮 + 带文字的 DeleteButton」，而容器给所有 td 加了
 * `overflow: hidden`；flex 又是 justify-end，溢出部分从**左侧**被裁掉，
 * 于是最左侧的「编辑」按钮直接消失（无省略号、无滚动条，肉眼只看到按钮没了）。
 *
 * 为什么是文本断言而不是渲染断言：jsdom 没有布局引擎（`getBoundingClientRect()` 恒为 0），
 * 单元格裁剪这类回归只能在真实浏览器里量（线上 DOM 实测）。这里退而守护
 * 「已实测给足的宽度不被改小」，改按钮组合时需同步更新本文件的预算。
 */
const MIN_ACTIONS_COL_REM = 10.5; // 实测：zh 需 10rem、en（"Delete" 文本更宽）需 10.5rem

const DIR = fileURLToPath(new URL("..", import.meta.url));
const VIEWS = ["grouped-view.tsx", "flat-view.tsx"] as const;

/** 取表头行里操作列的 `w-[Nrem]`；未声明宽度返回 0（同样不合格） */
function actionsColumnRem(view: string): number {
  const line = readFileSync(join(DIR, view), "utf8")
    .split("\n")
    .find((l) => l.includes('t("upstream.col_actions")'));
  return Number(line?.match(/w-\[([\d.]+)rem\]/)?.[1] ?? 0);
}

describe("upstream 表格操作列宽度预算", () => {
  for (const view of VIEWS) {
    it(`${view} 的操作列不窄于 ${MIN_ACTIONS_COL_REM}rem`, () => {
      expect(actionsColumnRem(view)).toBeGreaterThanOrEqual(MIN_ACTIONS_COL_REM);
    });
  }
});
