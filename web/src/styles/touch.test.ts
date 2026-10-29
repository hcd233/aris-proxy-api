import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 触屏尺寸层契约（spec docs/superpowers/specs/2026-10-29-mobile-tablet-adapt-design.md §5.2）：
 * 所有规则必须包在 `@media (pointer: coarse)` 内，桌面鼠标模式零影响；
 * 小控件（开关/复选/单选）只扩点击区域，不放大外观。
 */

const css = readFileSync(fileURLToPath(new URL("./touch.css", import.meta.url)), "utf8");
const code = css.replace(/\/\*[\s\S]*?\*\//g, "");

/** 取出顶层块：返回 [前导选择器/at-rule, 块体] 列表 */
function topLevelBlocks(src: string): Array<[string, string]> {
  const out: Array<[string, string]> = [];
  let depth = 0;
  let start = 0;
  let head = "";
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (ch === "{") {
      if (depth === 0) {
        head = src.slice(start, i).trim();
        start = i + 1;
      }
      depth++;
    } else if (ch === "}") {
      depth--;
      if (depth === 0) {
        out.push([head, src.slice(start, i)]);
        start = i + 1;
      }
    }
  }
  return out;
}

const blocks = topLevelBlocks(code);
const body = blocks.map(([, b]) => b).join("\n");

describe("touch.css", () => {
  it("所有规则都在 pointer: coarse 媒体查询内", () => {
    expect(blocks.length).toBeGreaterThan(0);
    for (const [head] of blocks) expect(head).toBe("@media (pointer: coarse)");
    expect(code.replace(/@media \(pointer: coarse\)\s*\{[\s\S]*\}/, "").trim()).toBe("");
  });

  it("覆盖全部可交互 slot", () => {
    for (const slot of [
      "button",
      "toggle-group-item",
      "toggle",
      "input",
      "input-group",
      "textarea",
      "select-trigger",
      "combobox-trigger",
      "dropdown-menu-item",
      "select-item",
      "combobox-item",
      "sidebar-menu-button",
      "sidebar-group-label",
      "filter-bar",
      "switch",
      "checkbox",
      "radio-group-item",
      "slider-thumb",
    ]) {
      expect(body, slot).toContain(`[data-slot="${slot}"]`);
    }
  });

  it("触屏下侧栏图标栏加宽到能容纳 44px 导航项", () => {
    expect(body).toMatch(
      /\[data-slot="sidebar-wrapper"\]\s*\{[^}]*--sidebar-width-icon:\s*3\.75rem\s*!important/,
    );
    expect(body).toMatch(/\[data-slot="sidebar-menu-button"\][^{]*\{[^}]*min-width:\s*44px/);
  });

  it("小控件只扩点击区域不放大外观", () => {
    for (const slot of ["switch", "checkbox", "radio-group-item", "slider-thumb"]) {
      const sel = `[data-slot="${slot}"]`;
      const occurrences = body.split(sel).length - 1;
      const afterOccurrences = body.split(`${sel}::after`).length - 1;
      expect(occurrences, slot).toBeGreaterThan(0);
      expect(afterOccurrences, slot).toBe(occurrences);
    }
    const rule = body.match(/\[data-slot="switch"\]::after[^{]*\{([^}]*)\}/);
    expect(rule?.[1]).toContain("min-width: 44px");
    expect(rule?.[1]).toContain("min-height: 44px");
  });
});
