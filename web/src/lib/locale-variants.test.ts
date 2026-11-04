import { describe, expect, it } from "vitest";
import en from "../locales/en.json";
import zh from "../locales/zh.json";
import ja from "../locales/ja.json";
import { localeVariants } from "./locale-variants";

describe("localeVariants", () => {
  it("returns every locale's translation of the same key, whichever locale the text is in", () => {
    const key = "apikeys.create";
    const all = [en[key], zh[key], ja[key]];
    for (const text of all) {
      expect(localeVariants(text)).toEqual(expect.arrayContaining([...new Set(all)]));
    }
  });

  it("covers every key sharing the text, so the reserve is never too narrow", () => {
    // "Retry" is the English text of several keys whose zh/ja differ.
    const keys = (Object.keys(en) as (keyof typeof en)[]).filter((k) => en[k] === "Retry");
    expect(keys.length).toBeGreaterThan(1);
    const variants = localeVariants("Retry");
    for (const k of keys) {
      expect(variants).toEqual(expect.arrayContaining([zh[k], ja[k]]));
    }
  });

  it("returns the same group for every text linked through a shared translation", () => {
    // regression: "Create Key" and "创建密钥" used to yield different sets,
    // so the button still resized between en and zh
    const group = localeVariants(zh["apikeys.create"]);
    expect(group).not.toBeNull();
    for (const text of group!) expect(new Set(localeVariants(text))).toEqual(new Set(group));
  });

  it("returns null for text that is not a translation", () => {
    expect(localeVariants("tester@example.com")).toBeNull();
  });
});
