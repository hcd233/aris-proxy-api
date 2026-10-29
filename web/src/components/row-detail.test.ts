import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it, vi } from "vitest";
import { priorityClass, rowDetailProps, shouldOpenRowDetail } from "../lib/row-detail";

/**
 * 表格列分级与行详情（spec 2026-10-29-mobile-tablet-adapt §5.1）。
 * 行详情只在 <1024px 打开；点行内交互元素（开关、按钮、链接）不应同时打开抽屉。
 */

type FakeEl = { closest: (sel: string) => FakeEl | null };
const el = (hit: FakeEl | null = null): FakeEl => ({ closest: () => hit });

describe("priorityClass", () => {
  it("不传时不输出任何类", () => {
    expect(priorityClass(undefined)).toBe("");
  });
  it("md 档在平板起显示", () => {
    expect(priorityClass("md")).toBe("hidden md:table-cell");
  });
  it("lg 档在桌面起显示", () => {
    expect(priorityClass("lg")).toBe("hidden lg:table-cell");
  });
});

describe("shouldOpenRowDetail", () => {
  const row = el();
  it("点在行内非交互区域时打开", () => {
    expect(shouldOpenRowDetail(el(null) as unknown as EventTarget, row as unknown as Element)).toBe(
      true,
    );
  });
  it("命中的交互祖先就是行本身时打开", () => {
    expect(shouldOpenRowDetail(el(row) as unknown as EventTarget, row as unknown as Element)).toBe(
      true,
    );
  });
  it("点在行内按钮等交互元素上时不打开", () => {
    const button = el();
    expect(
      shouldOpenRowDetail(el(button) as unknown as EventTarget, row as unknown as Element),
    ).toBe(false);
  });
  it("target 为空时打开", () => {
    expect(shouldOpenRowDetail(null, row as unknown as Element)).toBe(true);
  });
});

describe("rowDetailProps", () => {
  it("未启用时返回空对象，桌面行保持原样", () => {
    expect(rowDetailProps(false, () => {}, "x")).toEqual({});
  });

  it("启用时行可聚焦并带可读名称", () => {
    const p = rowDetailProps(true, () => {}, "查看详情: a");
    expect(p).toMatchObject({ role: "button", tabIndex: 0, "aria-label": "查看详情: a" });
  });

  it("Enter / 空格只在行自身聚焦时打开", () => {
    const onOpen = vi.fn();
    const p = rowDetailProps(true, onOpen, "x");
    const row = {};
    const preventDefault = vi.fn();
    const key = (k: string, target: unknown) =>
      p.onKeyDown!({ key: k, target, currentTarget: row, preventDefault } as never);
    key("Enter", row);
    key(" ", row);
    key("Enter", {});
    key("a", row);
    expect(onOpen).toHaveBeenCalledTimes(2);
    expect(preventDefault).toHaveBeenCalledTimes(2);
  });
});

describe("文案", () => {
  it.each(["en", "zh", "ja"])("%s 含 row_detail.view", (lang) => {
    const json = JSON.parse(
      readFileSync(fileURLToPath(new URL(`../locales/${lang}.json`, import.meta.url)), "utf8"),
    );
    expect(json["row_detail.view"]).toBeTruthy();
  });
});
