import { describe, expect, it } from "vitest";
import { buildChatBody, extractDeltaContent, selectableModels } from "../playground-logic";

describe("buildChatBody", () => {
  it("剔除空消息行并组装基础请求体", () => {
    const body = buildChatBody(
      [
        { role: "user", content: "hi" },
        { role: "user", content: "   " },
        { role: "assistant", content: "hello" },
      ],
      { model: "gpt-4", stream: false },
    );
    expect(body).toEqual({
      model: "gpt-4",
      messages: [
        { role: "user", content: "hi" },
        { role: "assistant", content: "hello" },
      ],
      stream: false,
    });
  });

  it("可选参数仅在有效时携带", () => {
    const withParams = buildChatBody([{ role: "user", content: "hi" }], {
      model: "gpt-4",
      stream: true,
      temperature: 0.7,
      maxTokens: 100,
    });
    expect(withParams.temperature).toBe(0.7);
    expect(withParams.max_tokens).toBe(100);

    const without = buildChatBody([{ role: "user", content: "hi" }], {
      model: "gpt-4",
      stream: false,
      temperature: undefined,
      maxTokens: 0,
    });
    expect("temperature" in without).toBe(false);
    expect("max_tokens" in without).toBe(false);
  });
});

describe("selectableModels", () => {
  it("仅保留 enabled 项并按 alias 去重", () => {
    const got = selectableModels([
      { alias: "a", enabled: true },
      { alias: "b", enabled: false },
      { alias: "a", enabled: true },
      { alias: "c", enabled: true },
    ]);
    expect(got.map((m) => m.alias)).toEqual(["a", "c"]);
  });
});

describe("extractDeltaContent", () => {
  it("提取内容增量", () => {
    expect(extractDeltaContent(`data: {"choices":[{"delta":{"content":"hel"}}]}`)).toBe("hel");
    expect(extractDeltaContent(`data: {"choices":[{"delta":{"role":"assistant"}}]}`)).toBe("");
  });

  it("忽略 [DONE]、空数据与非 data 行", () => {
    expect(extractDeltaContent("data: [DONE]")).toBe("");
    expect(extractDeltaContent("data:")).toBe("");
    expect(extractDeltaContent("event: message")).toBe("");
    expect(extractDeltaContent("")).toBe("");
    expect(extractDeltaContent("data: {not-json")).toBe("");
  });
});
