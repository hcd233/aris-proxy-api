import { describe, expect, it } from "vitest";
import {
  cronJobLabel,
  formatDecimal,
  formatDurationMs,
  formatInteger,
  formatTokenCount,
} from "./utils";

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

describe("formatTokenCount", () => {
  it("keeps values under one thousand exact", () => {
    expect(formatTokenCount(0)).toBe("0");
    expect(formatTokenCount(999)).toBe("999");
    expect(formatTokenCount(999.4)).toBe("999");
  });

  it("promotes values that round up to one thousand", () => {
    expect(formatTokenCount(999.5)).toBe("1K");
  });

  it("abbreviates by magnitude with at most one decimal", () => {
    expect(formatTokenCount(1_000)).toBe("1K");
    expect(formatTokenCount(1_500)).toBe("1.5K");
    expect(formatTokenCount(1_049)).toBe("1K");
    expect(formatTokenCount(1_050)).toBe("1.1K");
    expect(formatTokenCount(65_536)).toBe("65.5K");
    expect(formatTokenCount(1_000_000)).toBe("1M");
    expect(formatTokenCount(1_048_576)).toBe("1M");
    expect(formatTokenCount(1_234_567)).toBe("1.2M");
    expect(formatTokenCount(1_000_000_000)).toBe("1B");
    expect(formatTokenCount(1_200_000_000)).toBe("1.2B");
  });

  it("rolls over to the next unit instead of printing 1000K", () => {
    expect(formatTokenCount(999_949)).toBe("999.9K");
    expect(formatTokenCount(999_960)).toBe("1M");
  });

  it("keeps the sign and degrades invalid values to an em dash", () => {
    expect(formatTokenCount(-1_500)).toBe("-1.5K");
    expect(formatTokenCount(Number.NaN)).toBe("\u2014");
    expect(formatTokenCount(null)).toBe("\u2014");
    expect(formatTokenCount(undefined)).toBe("\u2014");
  });
});

describe("formatInteger", () => {
  it("groups thousands and keeps zero", () => {
    expect(formatInteger(0)).toBe("0");
    expect(formatInteger(1_234_567)).toBe("1,234,567");
  });

  it("renders an em dash for missing values", () => {
    expect(formatInteger(null)).toBe("\u2014");
    expect(formatInteger(undefined)).toBe("\u2014");
  });
});

describe("formatDecimal", () => {
  it("caps fraction digits and groups thousands", () => {
    expect(formatDecimal(12.345)).toBe("12.35");
    expect(formatDecimal(12.345, 0)).toBe("12");
    expect(formatDecimal(1_234.5)).toBe("1,234.5");
  });
});

describe("formatDurationMs", () => {
  it("uses milliseconds below one second and seconds above", () => {
    expect(formatDurationMs(0)).toBe("0ms");
    expect(formatDurationMs(850)).toBe("850ms");
    expect(formatDurationMs(1_300)).toBe("1.3s");
    expect(formatDurationMs(1_250)).toBe("1.25s");
  });

  it("renders an em dash for missing values", () => {
    expect(formatDurationMs(undefined)).toBe("\u2014");
    expect(formatDurationMs(null)).toBe("\u2014");
  });
});
