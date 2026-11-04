import { describe, expect, it } from "vitest";
import { countLines, formatToolPayload, previewFirstArg } from "./tool-payload";

describe("formatToolPayload", () => {
  it("空串保持空、无语言", () => {
    expect(formatToolPayload("")).toEqual({ code: "", language: "" });
  });

  it("可解析 JSON 美化为 2 空格缩进并标 json", () => {
    expect(formatToolPayload('{"a":1}')).toEqual({ code: '{\n  "a": 1\n}', language: "json" });
  });

  it("非 JSON 原样、无语言", () => {
    expect(formatToolPayload("plain text")).toEqual({ code: "plain text", language: "" });
  });
});

describe("previewFirstArg", () => {
  it("取首个参数键值，字符串加引号", () => {
    expect(previewFirstArg('{"path":"/x","n":2}')).toBe('path: "/x"');
  });

  it("非 JSON 或空对象返回空串", () => {
    expect(previewFirstArg("not json")).toBe("");
    expect(previewFirstArg("{}")).toBe("");
  });
});

describe("countLines", () => {
  it("忽略末尾换行", () => {
    expect(countLines("a\nb\n")).toBe(2);
    expect(countLines("")).toBe(0);
  });
});
