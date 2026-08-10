import type { OAuth2Provider } from "./types";

export interface OAuthCallbackParams {
  code: string;
  state: string;
}

/** 从 OAuth2 回调 URL 解析 code/state；任一缺失返回 null。 */
export function parseOAuthCallbackParams(search: string): OAuthCallbackParams | null {
  const params = new URLSearchParams(search);
  const code = params.get("code");
  const state = params.get("state");
  if (!code || !state) return null;
  return { code, state };
}

/**
 * 解析 state 中的 provider 前缀（如 `provider:github:<random>`）。
 * 格式不符返回 null —— 调用方应显式报错，禁止静默降级到默认平台。
 */
export function resolveOAuthPlatform(state: string): OAuth2Provider | null {
  const match = state.match(/^provider:(github|google):/);
  return match ? (match[1] as OAuth2Provider) : null;
}
