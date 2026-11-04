import { describe, expect, it } from "vitest";
import { formatCharCount } from "./reasoning-format";

describe("formatCharCount", () => {
  it("千以下原样，千以上换算 k 并去掉末尾 .0", () => {
    expect(formatCharCount(980)).toBe("980");
    expect(formatCharCount(1200)).toBe("1.2k");
    expect(formatCharCount(12000)).toBe("12k");
  });
});
