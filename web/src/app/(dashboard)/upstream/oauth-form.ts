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

/**
 * 归一化用户回贴的授权码输入：只裁剪首尾空白并做非空校验。
 *
 * 三形态（完整回调 URL / "code#state" / 纯 code）的解析由服务端 ParseOAuthCode
 * 单一权威负责——前端重复实现会让两端规则漂移；这里只拦明显的空输入。
 * 返回 null 表示输入无效。
 */
export function normalizeOAuthCodeInput(input: string): string | null {
  const trimmed = input.trim();
  return trimmed.length > 0 ? trimmed : null;
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
