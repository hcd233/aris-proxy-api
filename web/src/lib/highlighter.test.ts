import { describe, expect, it } from "vitest";
import { resolveLanguage, shouldHighlight } from "./highlighter";

describe("resolveLanguage", () => {
  it.each([
    ["ts", "typescript"],
    ["golang", "go"],
    ["RS", "rust"],
    ["dockerfile", "dockerfile"],
    ["json", "json"],
    ["ansi", "ansi"],
  ])("%s → %s", (input, want) => {
    expect(resolveLanguage(input)).toBe(want);
  });

  it("未收录语言与空语言返回 null（按纯文本渲染）", () => {
    expect(resolveLanguage("foo")).toBeNull();
    expect(resolveLanguage("")).toBeNull();
  });
});

describe("shouldHighlight", () => {
  it("超过 200_000 字符不高亮，避免同步分词卡死主线程", () => {
    expect(shouldHighlight("a".repeat(200_000))).toBe(true);
    expect(shouldHighlight("a".repeat(200_001))).toBe(false);
  });
});
