/** 门户页的常量与纯逻辑：组件只负责渲染，这里集中决定链接与文案 key。 */
import { DASHBOARD_HOME_PATH } from "./nav";

export const GITHUB_REPO_URL = "https://github.com/hcd233/aris-proxy-api";

export const LOGIN_PATH = "/login/";

export type PortalCtaLabelKey = "portal.cta.login" | "portal.cta.console";

/**
 * 首屏主按钮：认证加载中一律按未登录展示，避免按钮在 "登录 / 进入控制台" 间闪烁；
 * 加载完成且已登录才指向仪表盘。
 */
export function portalPrimaryCta(state: { isLoading: boolean; hasUser: boolean }): {
  href: string;
  labelKey: PortalCtaLabelKey;
} {
  if (!state.isLoading && state.hasUser) {
    return { href: DASHBOARD_HOME_PATH, labelKey: "portal.cta.console" };
  }
  return { href: LOGIN_PATH, labelKey: "portal.cta.login" };
}

export type PortalPillarKey = "access" | "curation" | "governance";

export const PORTAL_PILLARS: readonly { key: PortalPillarKey; items: readonly string[] }[] = [
  {
    key: "access",
    items: [1, 2, 3, 4].map((n) => `portal.pillar.access.item_${n}`),
  },
  {
    key: "curation",
    items: [1, 2, 3, 4].map((n) => `portal.pillar.curation.item_${n}`),
  },
  {
    key: "governance",
    items: [1, 2, 3, 4].map((n) => `portal.pillar.governance.item_${n}`),
  },
];

/** 链路示意两端节点：产品专名直接展示不翻译，泛称走 i18n key。 */
export const PORTAL_FLOW = {
  clients: ["Claude Code", "Codex", "OpenCode"],
  upstreams: ["OpenAI · Anthropic", "portal.flow.maas", "portal.flow.subscription"],
} as const;
