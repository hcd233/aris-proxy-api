import { describe, expect, it } from "vitest";
import { balanceEmphasis } from "./markdown";

describe("balanceEmphasis", () => {
  it("keeps balanced bold markers untouched", () => {
    expect(balanceEmphasis("前 **重点** 后")).toBe("前 **重点** 后");
  });

  it("drops the dangling marker of a truncated message", () => {
    expect(balanceEmphasis("核心目标：**尽量拦掉无效请求、控制回源并发")).toBe(
      "核心目标：尽量拦掉无效请求、控制回源并发",
    );
  });

  it("only drops the last unpaired marker when several paragraphs are involved", () => {
    expect(balanceEmphasis("**甲**\n\n**乙")).toBe("**甲**\n\n乙");
  });

  it("leaves markers inside fenced code alone", () => {
    expect(balanceEmphasis("说明 **粗**\n\n```\na ** b\n```")).toBe(
      "说明 **粗**\n\n```\na ** b\n```",
    );
  });
});
