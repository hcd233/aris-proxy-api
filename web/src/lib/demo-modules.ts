import type { DemoModule } from "./types";

/** 全部合法 Demo 模块（顺序即配置卡片展示顺序） */
export const DEMO_MODULES: readonly DemoModule[] = [
  "dashboard",
  "sessions",
  "audit",
  "upstream",
  "trigger",
  "monitor",
  "cron",
  "cron_audit",
];

/** 旧模块名 → 新模块名（endpoints/models 已合并为 upstream） */
const LEGACY_MODULE_MAP: Record<string, DemoModule> = {
  endpoints: "upstream",
  models: "upstream",
};

/**
 * 规范化 demo 开放模块列表：旧模块名迁移 + 过滤非法值 + 去重（保持原有顺序）。
 * 防御存量数据中的旧模块名（endpoints/models）导致保存校验失败或导航权限错乱。
 */
export function normalizeDemoModules(modules: string[] | null | undefined): DemoModule[] {
  if (!modules) return [];
  const valid = new Set<string>(DEMO_MODULES);
  const seen = new Set<string>();
  const result: DemoModule[] = [];
  for (const raw of modules) {
    const normalized = LEGACY_MODULE_MAP[raw] ?? raw;
    if (!valid.has(normalized) || seen.has(normalized)) continue;
    seen.add(normalized);
    result.push(normalized as DemoModule);
  }
  return result;
}
