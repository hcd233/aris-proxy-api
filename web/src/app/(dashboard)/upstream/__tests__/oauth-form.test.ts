import { describe, expect, it } from "vitest";

import { oauthBadgeKey, oauthCodeHintKey, parseOAuthCodeInput, quotaPercent } from "../oauth-form";

describe("parseOAuthCodeInput", () => {
  it("从完整回调 URL 提取 code 与 state（Codex 形态）", () => {
    expect(parseOAuthCodeInput("http://localhost:1455/auth/callback?code=ac_1&state=st_1")).toEqual(
      { code: "ac_1", state: "st_1" },
    );
  });

  it("按 # 切分 code#state（Claude 形态）", () => {
    expect(parseOAuthCodeInput("ac_2#st_2")).toEqual({ code: "ac_2", state: "st_2" });
  });

  it("纯 code 保留 state 为空", () => {
    expect(parseOAuthCodeInput("  ac_3  ")).toEqual({ code: "ac_3", state: "" });
  });

  it("空输入与缺少 code 的 URL 返回 null", () => {
    expect(parseOAuthCodeInput("   ")).toBeNull();
    expect(parseOAuthCodeInput("https://x/cb?state=st")).toBeNull();
    expect(parseOAuthCodeInput("#st")).toBeNull();
  });
});

describe("quotaPercent", () => {
  it("四舍五入并钳位", () => {
    expect(quotaPercent(12.6)).toBe(13);
    expect(quotaPercent(-1)).toBe(0);
    expect(quotaPercent(150)).toBe(100);
    expect(quotaPercent(Number.NaN)).toBe(0);
  });
});

describe("oauth 文案 key 选择", () => {
  it("按提供方与认证类型返回对应 key", () => {
    expect(oauthCodeHintKey("anthropic")).toBe("oauth.hint_claude");
    expect(oauthCodeHintKey("openai")).toBe("oauth.hint_codex");
    expect(oauthBadgeKey("oauth_anthropic")).toBe("oauth.badge_claude");
    expect(oauthBadgeKey("oauth_openai")).toBe("oauth.badge_codex");
    expect(oauthBadgeKey("api_key")).toBeNull();
  });
});
