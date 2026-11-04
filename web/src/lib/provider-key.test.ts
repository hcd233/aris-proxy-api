import { describe, expect, it } from "vitest";
import { findProviderKey, groupModels, providerGroupOf } from "./provider-key";

describe("findProviderKey", () => {
  it.each([
    ["claudecode", "claudecode"],
    ["codex", "codex"],
    ["opencode", "opencode"],
    ["pi", "pi"],
    ["claude", "claude"],
    ["deepseek", "deepseek"],
    ["hunyuan", "hunyuan"],
    ["zai", "zai"],
  ])("%s -> %s", (input, key) => expect(findProviderKey(input)).toBe(key));

  it("does not let 'claude' swallow 'claudecode'", () =>
    expect(findProviderKey("claudecode")).not.toBe("claude"));

  it("still maps model ids by prefix", () =>
    expect(findProviderKey("deepseek-v4-pro")).toBe("deepseek"));
});

describe("providerGroupOf", () => {
  it.each([
    ["gpt-4o", "openai"],
    ["o3-mini", "openai"],
    ["GPT-4O", "openai"],
    ["claude-sonnet-4-5", "anthropic"],
    ["gemini-2.5-pro", "google"],
    ["glm-4.6", "zhipu"],
    ["kimi-k2", "moonshot"],
    ["hy3", "tencent"],
    ["deepseek-v3", "deepseek"],
    ["my-local-model", "other"],
  ])("%s -> %s", (alias, group) => expect(providerGroupOf(alias)).toBe(group));
});

describe("groupModels", () => {
  it("组按模型数降序，other 恒在最后，组内按别名字母序", () => {
    const groups = groupModels([
      { alias: "gpt-b" },
      { alias: "zzz" },
      { alias: "claude-x" },
      { alias: "gpt-a" },
    ]);
    expect(groups.map((g) => g.key)).toEqual(["openai", "anthropic", "other"]);
    expect(groups[0].label).toBe("OpenAI");
    expect(groups[0].items.map((m) => m.alias)).toEqual(["gpt-a", "gpt-b"]);
  });

  it("同数量的组按 key 字母序；未登记显示名的组首字母大写", () => {
    const groups = groupModels([{ alias: "qwen-max" }, { alias: "deepseek-v3" }]);
    expect(groups.map((g) => g.label)).toEqual(["Deepseek", "Qwen"]);
  });
});
