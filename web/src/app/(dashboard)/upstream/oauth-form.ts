// OAuth 订阅账号授权的前端纯函数与表单类型（无 @/ 别名依赖，可被 vitest 直接断言）。

import type { EndpointAuthType, OAuthProviderChoice } from "@/lib/types";

export interface OAuthAuthorizeForm {
  provider: OAuthProviderChoice;
  code: string;
  state: string;
}

export const emptyOAuthAuthorizeForm: OAuthAuthorizeForm = {
  provider: "anthropic",
  code: "",
  state: "",
};

export interface ParsedOAuthCodeInput {
  code: string;
  state: string;
}

/**
 * 归一化用户回贴的授权码输入（与后端 ParseOAuthCode 同规则的三形态）：
 * 1. 完整回调 URL（Codex 浏览器跳 localhost 打不开，从地址栏复制整段）→ 取 query 的 code/state；
 * 2. "code#state"（Claude 授权页展示格式）→ 按首个 # 切分；
 * 3. 纯 code → state 留空（提交时由服务端 flow 记录兜底）。
 * 非法输入返回 null（前端预校验，服务端仍会复验）。
 */
export function parseOAuthCodeInput(input: string): ParsedOAuthCodeInput | null {
  const trimmed = input.trim();
  if (!trimmed) {
    return null;
  }
  if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) {
    try {
      const url = new URL(trimmed);
      const code = (url.searchParams.get("code") ?? "").trim();
      const state = (url.searchParams.get("state") ?? "").trim();
      return code ? { code, state } : null;
    } catch {
      return null;
    }
  }
  const hashIndex = trimmed.indexOf("#");
  if (hashIndex >= 0) {
    const code = trimmed.slice(0, hashIndex).trim();
    const state = trimmed.slice(hashIndex + 1).trim();
    return code ? { code, state } : null;
  }
  return { code: trimmed, state: "" };
}

/** 额度窗口百分比（四舍五入并钳位到 0~100，用于进度条宽度） */
export function quotaPercent(usedPercent: number): number {
  if (!Number.isFinite(usedPercent)) {
    return 0;
  }
  return Math.min(100, Math.max(0, Math.round(usedPercent)));
}

/** 授权面板的粘贴提示 i18n key（按提供方区分文案） */
export function oauthCodeHintKey(provider: OAuthProviderChoice): string {
  return provider === "anthropic" ? "oauth.hint_claude" : "oauth.hint_codex";
}

/** 端点认证类型 → 订阅账号角标 i18n key；api_key 端点无角标 */
export function oauthBadgeKey(authType: string): string | null {
  switch (authType) {
    case "oauth_anthropic":
      return "oauth.badge_claude";
    case "oauth_openai":
      return "oauth.badge_codex";
    default:
      return null;
  }
}

/**
 * 端点弹窗的认证类型展示值：
 * - oauthMode=null → api_key 表单；
 * - reauth → 沿用既有端点类型；
 * - create → 由所选 provider 推导。
 */
export function authSelectionForDialog(
  mode: "create" | "reauth" | null,
  provider: OAuthProviderChoice,
  reauthAuthType: EndpointAuthType | null,
): EndpointAuthType {
  if (mode === null) {
    return "api_key";
  }
  if (mode === "reauth" && reauthAuthType) {
    return reauthAuthType;
  }
  return provider === "openai" ? "oauth_openai" : "oauth_anthropic";
}
