/** 定时刷新档位：ms 为实际间隔，label 为语言中立的短标签（5s / 1m）。 */
export interface RefreshIntervalOption {
  readonly ms: number;
  readonly label: string;
}

/** Monitor 是秒级运行时指标，最小档位 5s。 */
export const MONITOR_REFRESH_INTERVALS: readonly RefreshIntervalOption[] = [
  { ms: 5_000, label: "5s" },
  { ms: 15_000, label: "15s" },
  { ms: 30_000, label: "30s" },
  { ms: 60_000, label: "1m" },
  { ms: 300_000, label: "5m" },
];

/** Dashboard 是聚合查询，最小档位 30s，避免默认就给后端加压。 */
export const DASHBOARD_REFRESH_INTERVALS: readonly RefreshIntervalOption[] = [
  { ms: 30_000, label: "30s" },
  { ms: 60_000, label: "1m" },
  { ms: 300_000, label: "5m" },
  { ms: 900_000, label: "15m" },
];

export const DEFAULT_MONITOR_REFRESH_MS = 5_000;
export const DEFAULT_DASHBOARD_REFRESH_MS = 30_000;

/** localStorage 可能残留旧档位或脏值，非当前档位一律回落到默认值。 */
export function normalizeRefreshInterval(
  value: unknown,
  options: readonly RefreshIntervalOption[],
  fallback: number,
): number {
  return options.some((option) => option.ms === value) ? (value as number) : fallback;
}

/** 状态文案用：5000 → "5s"，60000 → "1m"，900000 → "15m"。 */
export function formatRefreshInterval(ms: number): string {
  if (ms < 60_000) return `${Math.round(ms / 1_000)}s`;
  return `${Math.round(ms / 60_000)}m`;
}
