import { describe, expect, it } from "vitest";
import { findProviderKey } from "./provider-key";

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
