import { describe, expect, it } from "vitest";
import { cronJobLabel } from "./utils";

/** 模拟 i18n：命中时返回文案，未命中时回退 fallback（与 useI18n().t 同语义）。 */
const translate = (key: string, fallback?: string): string => {
  const table: Record<string, string> = {
    "cron.job_oauth_maintenance": "OAuth 维护",
    "cron.job_session_terminal_cleanup": "会话终结清理",
    "cron.job_soft_delete_purge": "软删清理",
    "cron.job_think_extract": "思考提取",
    "cron.job_trigger_hit_sync": "触发器命中同步",
  };
  return table[key] ?? fallback ?? key;
};

describe("cronJobLabel", () => {
  it("derives the i18n suffix from backend module names without a hand-maintained map", () => {
    expect(cronJobLabel("OAuthMaintenanceCron", translate)).toBe("OAuth 维护");
    expect(cronJobLabel("SessionTerminalCleanupCron", translate)).toBe("会话终结清理");
    expect(cronJobLabel("SoftDeletePurgeCron", translate)).toBe("软删清理");
    expect(cronJobLabel("ThinkExtractCron", translate)).toBe("思考提取");
    expect(cronJobLabel("TriggerHitSyncCron", translate)).toBe("触发器命中同步");
  });

  it("falls back to the raw module name for jobs added by the backend", () => {
    expect(cronJobLabel("BrandNewMaintenanceCron", translate)).toBe("BrandNewMaintenanceCron");
  });
});
