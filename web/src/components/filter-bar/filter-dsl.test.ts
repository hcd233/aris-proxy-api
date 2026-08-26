import { describe, expect, it } from "vitest";
import {
  migrateLegacyTokens,
  parseFilterString,
  serializeTokens,
  type FilterToken,
} from "./filter-dsl";

describe("serializeTokens", () => {
  it("空数组返回 undefined", () => {
    expect(serializeTokens([])).toBeUndefined();
  });

  it("自由文本 token 不进入 DSL", () => {
    expect(serializeTokens([{ key: null, value: "退款" }])).toBeUndefined();
  });

  it("单 token 序列化", () => {
    expect(serializeTokens([{ key: "score", value: "5" }])).toBe("score:5");
  });

  it("同 key 多 token 合并为 | 分隔", () => {
    const tokens: FilterToken[] = [
      { key: "score", value: "5" },
      { key: "score", value: "4" },
    ];
    expect(serializeTokens(tokens)).toBe("score:5|4");
  });

  it("多 key 以空格连接且保持首次出现顺序", () => {
    const tokens: FilterToken[] = [
      { key: "model", value: "gpt-5.2" },
      { key: "score", value: "5" },
      { key: "model", value: "kimi-k3" },
    ];
    expect(serializeTokens(tokens)).toBe("model:gpt-5.2|kimi-k3 score:5");
  });

  it("含空格的值整体加引号", () => {
    expect(serializeTokens([{ key: "model", value: "hello world" }])).toBe('model:"hello world"');
  });

  it("含 | 的值整体加引号（后端按单值字面量处理）", () => {
    expect(serializeTokens([{ key: "model", value: "a|b" }])).toBe('model:"a|b"');
  });

  it("含双引号的值降级为单引号（DSL 无转义机制，优于静默丢字符）", () => {
    expect(serializeTokens([{ key: "model", value: 'a"b' }])).toBe('model:"a\'b"');
  });

  it("range 桶值原样传递", () => {
    expect(serializeTokens([{ key: "messageCount", value: "0-10" }])).toBe("messageCount:0-10");
  });

  it("同 key 多值仅部分需转义时逐值处理", () => {
    const tokens: FilterToken[] = [
      { key: "model", value: "gpt-5.2" },
      { key: "model", value: "hello world" },
    ];
    expect(serializeTokens(tokens)).toBe('model:gpt-5.2|"hello world"');
  });
});

describe("parseFilterString", () => {
  const keys = new Set(["score", "model", "messageCount"]);

  it("空串与空白返回空数组", () => {
    expect(parseFilterString("", keys)).toEqual([]);
    expect(parseFilterString("   ", keys)).toEqual([]);
  });

  it("单值解析", () => {
    expect(parseFilterString("score:5", keys)).toEqual([{ key: "score", value: "5" }]);
  });

  it("多值 | 拆分为多 token", () => {
    expect(parseFilterString("score:5|4", keys)).toEqual([
      { key: "score", value: "5" },
      { key: "score", value: "4" },
    ]);
  });

  it("引号值去引号且不按 | 拆", () => {
    expect(parseFilterString('model:"a|b c"', keys)).toEqual([{ key: "model", value: "a|b c" }]);
  });

  it("未知 key 丢弃并保留其余", () => {
    expect(parseFilterString("unknown:1 score:5", keys)).toEqual([{ key: "score", value: "5" }]);
  });

  it("非法片段（无冒号/空 key）丢弃", () => {
    expect(parseFilterString("garbage :5 score:5", keys)).toEqual([{ key: "score", value: "5" }]);
  });

  it("UI 不生成的操作符片段（! > < = 开头值）丢弃", () => {
    expect(parseFilterString("score:!5 score:>3 score:5", keys)).toEqual([
      { key: "score", value: "5" },
    ]);
  });

  it("引号内空格不参与切分", () => {
    expect(parseFilterString('model:"hello world" score:5', keys)).toEqual([
      { key: "model", value: "hello world" },
      { key: "score", value: "5" },
    ]);
  });
});

describe("round-trip", () => {
  it("典型集合 parse(serialize) 还原", () => {
    const keys = new Set(["score", "model", "messageCount"]);
    const tokens: FilterToken[] = [
      { key: "score", value: "5" },
      { key: "score", value: "4" },
      { key: "model", value: "gpt-5.2" },
      { key: "messageCount", value: "0-10" },
    ];
    expect(parseFilterString(serializeTokens(tokens)!, keys)).toEqual(tokens);
  });

  it("含空格值 round-trip", () => {
    const keys = new Set(["model"]);
    const tokens: FilterToken[] = [{ key: "model", value: "hello world" }];
    expect(parseFilterString(serializeTokens(tokens)!, keys)).toEqual(tokens);
  });
});

describe("migrateLegacyTokens", () => {
  it("旧 string[] 与 string 混合迁移", () => {
    const tokens = migrateLegacyTokens({
      score: ["5", "4"],
      model: ["gpt-5.2"],
      keyword: "退款",
    });
    expect(tokens).toEqual([
      { key: "score", value: "5" },
      { key: "score", value: "4" },
      { key: "model", value: "gpt-5.2" },
      { key: null, value: "退款" },
    ]);
  });

  it("空旧值返回空数组", () => {
    expect(migrateLegacyTokens({ score: [], keyword: "" })).toEqual([]);
  });

  it("非预期类型安全忽略", () => {
    expect(migrateLegacyTokens({ score: 42, model: null })).toEqual([]);
  });
});
