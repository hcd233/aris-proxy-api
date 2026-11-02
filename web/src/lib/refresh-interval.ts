/** 关闭自动刷新的哨兵值；与档位共用同一个持久化字段。 */
export const REFRESH_OFF = 0;

/** Dashboard 是聚合查询，后端读缓存 5s（constant.ReadCacheDashboardTTL），档位与缓存粒度对齐。 */
export const DASHBOARD_REFRESH_INTERVALS: readonly number[] = [
  5_000, 10_000, 30_000, 60_000, 300_000,
];

/** Monitor 的最细聚合桶是 5s，再快没有新数据。 */
export const MONITOR_REFRESH_INTERVALS: readonly number[] = [
  5_000, 10_000, 30_000, 60_000, 300_000,
];

/**
 * localStorage 里的值可能是旧档位或脏数据：只接受「关闭」或当前页面的档位，
 * 其余回落到页面默认值。
 */
export function normalizeRefreshInterval(
  value: unknown,
  options: readonly number[],
  fallback: number,
): number {
  if (value === REFRESH_OFF) return REFRESH_OFF;
  return typeof value === "number" && options.includes(value) ? value : fallback;
}

/** 语言中立的短标签：5000 → "5s"，60000 → "1m"。 */
export function formatRefreshInterval(ms: number): string {
  return ms < 60_000 ? `${Math.round(ms / 1_000)}s` : `${Math.round(ms / 60_000)}m`;
}
