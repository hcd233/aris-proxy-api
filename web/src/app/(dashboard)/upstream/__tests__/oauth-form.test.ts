import { describe, expect, it } from "vitest";

import {
  normalizeOAuthCodeInput,
  oauthBadgeKey,
  oauthCodeHintKey,
  quotaPercent,
} from "../oauth-form";

describe("normalizeOAuthCodeInput", () => {
  it("裁剪首尾空白后原样返回输入（形态解析由服务端负责）", () => {
    expect(normalizeOAuthCodeInput("  ac_3  ")).toBe("ac_3");
    // 回调 URL 与 code#state 不再在前端解析，整段交给服务端 ParseOAuthCode
    expect(
      normalizeOAuthCodeInput("http://localhost:1455/auth/callback?code=ac_1&state=st_1"),
    ).toBe("http://localhost:1455/auth/callback?code=ac_1&state=st_1");
    expect(normalizeOAuthCodeInput("ac_2#st_2")).toBe("ac_2#st_2");
  });

  it("空输入返回 null", () => {
    expect(normalizeOAuthCodeInput("")).toBeNull();
    expect(normalizeOAuthCodeInput("   ")).toBeNull();
  });
});

describe("quotaPercent", () => {
  it("四舍五入并钳位（上游已归一为 0~100 百分比）", () => {
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
