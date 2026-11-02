/** 门户页的常量与纯逻辑：组件只负责渲染，这里集中决定链接、文案 key 与演示数据。 */
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

export const PORTAL_SECTION_IDS = {
  flow: "flow",
  capabilities: "capabilities",
  getStarted: "get-started",
} as const;

/** 专名 label 原样展示；有 labelKey 时优先 t(labelKey)。icon 是 ProviderIcon 的 protocol。 */
export type PortalEntity = { id: string; icon: string; label: string; labelKey?: string };

export const PORTAL_CLIENTS: readonly PortalEntity[] = [
  { id: "claudecode", icon: "claudecode", label: "Claude Code" },
  { id: "codex", icon: "codex", label: "Codex" },
  { id: "opencode", icon: "opencode", label: "OpenCode" },
  { id: "pi", icon: "pi", label: "Pi" },
  { id: "deepseek-harness", icon: "deepseek", label: "DeepSeek Harness" },
];

export const PORTAL_VENDORS: readonly PortalEntity[] = [
  { id: "openai", icon: "openai", label: "OpenAI" },
  { id: "anthropic", icon: "anthropic", label: "Anthropic" },
  { id: "deepseek", icon: "deepseek", label: "DeepSeek" },
  { id: "qwen", icon: "qwen", label: "Qwen" },
  { id: "hy", icon: "hunyuan", label: "Hy" },
  { id: "minimax", icon: "minimax", label: "MiniMax" },
  { id: "glm", icon: "zai", label: "GLM" },
];

const CODEX_SUB: PortalEntity = {
  id: "codex-sub",
  icon: "codex",
  label: "Codex",
  labelKey: "portal.upstream.codex_sub",
};
const CLAUDECODE_SUB: PortalEntity = {
  id: "claudecode-sub",
  icon: "claudecode",
  label: "Claude Code",
  labelKey: "portal.upstream.claudecode_sub",
};

export const PORTAL_UPSTREAMS: readonly PortalEntity[] = [
  ...PORTAL_VENDORS,
  CODEX_SUB,
  CLAUDECODE_SUB,
];

export const PORTAL_GATEWAY_STAGES = [
  "portal.flow.stage.auth",
  "portal.flow.stage.route",
  "portal.flow.stage.breaker",
  "portal.flow.stage.audit",
] as const;

/** 演示用模型名：取自 models.dev 2026-11-02 各厂商最新型号，一律小写。 */
export const PORTAL_MODELS = {
  sonnet: "claude-sonnet-5-5",
  opus: "claude-opus-5-5",
  gpt: "gpt-6.1-sol",
  deepseek: "deepseek-v4-pro",
  qwen: "qwen3.8-max",
  minimax: "minimax-m3.1-flash",
  glm: "glm-5.3",
  hy: "hy4-preview",
} as const;

export type PortalFeatureKey =
  "single_endpoint" | "sessions" | "export_cli" | "usage" | "self_heal" | "dataset";

export const PORTAL_FEATURES: readonly PortalFeatureKey[] = [
  "single_endpoint",
  "sessions",
  "export_cli",
  "usage",
  "self_heal",
  "dataset",
];

export type PortalCapabilityKey = "access" | "curation" | "governance" | "resilience";

const chips = (key: PortalCapabilityKey, n: number) =>
  Array.from({ length: n }, (_, i) => `portal.cap.${key}.chip_${i + 1}`);

export const PORTAL_CAPABILITIES: readonly {
  key: PortalCapabilityKey;
  chips: readonly string[];
}[] = [
  { key: "access", chips: chips("access", 5) },
  { key: "curation", chips: chips("curation", 4) },
  { key: "governance", chips: chips("governance", 4) },
  { key: "resilience", chips: chips("resilience", 5) },
];

export type PortalAuditRow = {
  icon: string;
  model: string;
  tokens: number;
  firstTokenMs: number | null;
  status: number;
};

export const PORTAL_AUDIT_ROWS: readonly PortalAuditRow[] = [
  { icon: "anthropic", model: PORTAL_MODELS.sonnet, tokens: 12400, firstTokenMs: 410, status: 200 },
  { icon: "deepseek", model: PORTAL_MODELS.deepseek, tokens: 8100, firstTokenMs: 280, status: 200 },
  { icon: "qwen", model: PORTAL_MODELS.qwen, tokens: 21700, firstTokenMs: 530, status: 200 },
  { icon: "minimax", model: PORTAL_MODELS.minimax, tokens: 1200, firstTokenMs: null, status: 502 },
  { icon: "zai", model: PORTAL_MODELS.glm, tokens: 6300, firstTokenMs: 340, status: 200 },
  { icon: "openai", model: PORTAL_MODELS.gpt, tokens: 15900, firstTokenMs: 620, status: 200 },
  { icon: "hunyuan", model: PORTAL_MODELS.hy, tokens: 4400, firstTokenMs: 390, status: 200 },
  { icon: "claudecode", model: PORTAL_MODELS.opus, tokens: 31200, firstTokenMs: 720, status: 200 },
];

export type PortalRouteEndpoint = {
  icon: string;
  label: string;
  labelKey?: string;
  weight: number;
};
export type PortalRouteTier = { priority: 1 | 2; endpoints: readonly PortalRouteEndpoint[] };

export const PORTAL_ROUTE: { alias: "sonnet"; tiers: readonly PortalRouteTier[] } = {
  alias: "sonnet",
  tiers: [
    {
      priority: 1,
      endpoints: [
        { icon: "anthropic", label: "Anthropic", weight: 70 },
        { icon: "claudecode", label: "Claude Code", labelKey: CLAUDECODE_SUB.labelKey, weight: 30 },
      ],
    },
    {
      priority: 2,
      endpoints: [
        { icon: "deepseek", label: "DeepSeek", weight: 100 },
        { icon: "zai", label: "GLM", weight: 100 },
      ],
    },
  ],
};

export type PortalStepKey = "login" | "upstream" | "install" | "use";

export const PORTAL_STEPS: readonly PortalStepKey[] = ["login", "upstream", "install", "use"];

export type PortalTtyLine = {
  kind: "cmd" | "out" | "ask" | "ok" | "blank";
  text: string;
  ms: number;
};

/** 与真实 CLI 一致：install.sh 末尾 exec aris init；aris model export 无参数，交互多选客户端。 */
export const PORTAL_TTY_LINES: readonly PortalTtyLine[] = [
  { kind: "cmd", text: "curl -fsSL https://aris.example.com/install.sh | sh", ms: 400 },
  { kind: "out", text: "aris-darwin-arm64.tar.gz  sha256 ok", ms: 500 },
  { kind: "ask", text: "Server host  https://aris.example.com", ms: 450 },
  { kind: "ask", text: "API key      ak-7Qx2•••••••3f9a  verified", ms: 600 },
  { kind: "ok", text: "Configuration completed", ms: 500 },
  { kind: "blank", text: "", ms: 150 },
  { kind: "cmd", text: "aris model export", ms: 400 },
  {
    kind: "ask",
    text: "Targets  ◉ Claude Code  ◉ Codex  ○ OpenCode  ○ Pi  ○ DeepSeek Harness",
    ms: 650,
  },
  { kind: "ok", text: "6 models → ~/.claude/settings.json  (.bak saved)", ms: 450 },
  { kind: "ok", text: "6 models → ~/.codex/config.toml  (.bak saved)", ms: 450 },
];
