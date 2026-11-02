import { describe, expect, it } from "vitest";
import { splitTokens } from "./split-words";

const flat = (text: string) =>
  splitTokens(text)
    .flatMap((t) => t.chars)
    .join("");
const words = (text: string) =>
  splitTokens(text)
    .filter((t) => !t.space)
    .map((t) => t.chars.join(""));

describe("splitTokens", () => {
  it("keeps every character, in order", () => {
    for (const text of [
      "一个地址，接入所有大模型",
      "One address for every model",
      "Hello, world.",
      "一つのアドレスで、すべてのモデルへ",
      "use (foo) and “bar”",
    ]) {
      expect(flat(text)).toBe(text);
    }
  });

  it("never breaks a Latin word", () => {
    expect(words("One address for every model")).toEqual([
      "One",
      "address",
      "for",
      "every",
      "model",
    ]);
  });

  it("marks only whitespace as breakable", () => {
    const tokens = splitTokens("a b");
    expect(tokens.map((t) => t.space)).toEqual([false, true, false]);
  });

  it("glues trailing punctuation to the previous word", () => {
    expect(words("Hello, world.")).toEqual(["Hello,", "world."]);
    // 「，」不能被拆到下一行行首
    for (const w of words("一个地址，接入所有大模型")) {
      expect(w.startsWith("，")).toBe(false);
    }
  });

  it("glues opening brackets and quotes to the next word", () => {
    expect(words("use (foo) now")).toEqual(["use", "(foo)", "now"]);
    expect(words("say “bar”")).toEqual(["say", "“bar”"]);
  });

  // ICU 词典不认识「接入」会切成「接 / 入所 / 有」，所以文案用 U+200B 显式标出允许断行的位置。
  it("breaks CJK text only at U+200B hints when present", () => {
    const text = "一个地址，\u200b接入所有大模型";
    expect(words(text)).toEqual(["一个地址，", "接入所有大模型"]);
    expect(flat(text)).toBe(text);
  });

  it("keeps spaces breakable next to hinted CJK phrases", () => {
    expect(words("API \u200b网关")).toEqual(["API", "网关"]);
  });

  it("represents a hint as a breakable zero-width token", () => {
    const tokens = splitTokens("甲\u200b乙");
    expect(tokens).toEqual([
      { space: false, chars: ["甲"] },
      { space: true, chars: ["\u200b"] },
      { space: false, chars: ["乙"] },
    ]);
  });

  it("returns nothing for empty text", () => {
    expect(splitTokens("")).toEqual([]);
  });
});
