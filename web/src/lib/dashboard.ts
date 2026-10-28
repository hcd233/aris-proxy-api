import { formatDecimal, formatDurationMs } from "./utils";
import type {
  DashboardHeatmapCell,
  DashboardModelPoint,
  DashboardStats,
  DashboardStatsPoint,
  DashboardTokens,
} from "./types";

/** 环比方向的好坏色调 */
export type DeltaTone = "good" | "bad" | "neutral";

export type KpiKind = "requests" | "tokens" | "cost" | "successRate" | "cacheHitRate" | "latency";

/** 6 张 KPI 的固定顺序与文案 key（数据缺失时也用它渲染标题，避免空白卡片） */
export const KPI_SPECS: ReadonlyArray<{ kind: KpiKind; labelKey: string }> = [
  { kind: "requests", labelKey: "dashboard.kpi.requests" },
  { kind: "tokens", labelKey: "dashboard.kpi.tokens" },
  { kind: "cost", labelKey: "dashboard.kpi.cost" },
  { kind: "successRate", labelKey: "dashboard.kpi.success_rate" },
  { kind: "cacheHitRate", labelKey: "dashboard.kpi.cache_hit" },
  { kind: "latency", labelKey: "dashboard.kpi.latency" },
];

/** 用量趋势可切换的指标 */
export type ModelMetric = "cost" | "requests" | "tokens";

/** foldTopModels 合并「其他」用的保留键 */
export const OTHER_KEY = "__other__";

const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;

/** 相对变化；上一周期 ≤ 0 时无意义，返回 null（展示「—」而不是 Infinity%） */
export function deltaPct(cur: number, prev: number): number | null {
  if (!(prev > 0)) return null;
  return (cur - prev) / prev;
}

/** 绝对差（用于比率的百分点差、延迟的毫秒差）；任一侧缺失返回 null */
export function deltaDiff(
  cur: number | null | undefined,
  prev: number | null | undefined,
): number | null {
  if (cur == null || prev == null) return null;
  return cur - prev;
}

/** 按「好坏」而非「涨跌」着色：费用上涨、延迟上涨、成功率下降为 bad */
export function deltaTone(kind: KpiKind, delta: number | null): DeltaTone {
  if (delta == null || delta === 0) return "neutral";
  const higherIsBad = kind === "cost" || kind === "latency";
  return delta > 0 === higherIsBad ? "bad" : "good";
}

export function successRate(s: DashboardStats): number | null {
  return s.requests > 0 ? s.success / s.requests : null;
}

/** 总输入 = 净输入 + 缓存读 + 缓存写（后端四维 token 互斥：input 落库时已归一化为净输入，不得再做 input − cacheRead 二次扣减） */
export function promptTokens(t: DashboardTokens): number {
  return t.input + t.cacheRead + t.cacheWrite;
}

export function cacheHitRate(t: DashboardTokens): number | null {
  const prompt = promptTokens(t);
  return prompt > 0 ? t.cacheRead / prompt : null;
}

export function totalTokens(t: DashboardTokens): number {
  return promptTokens(t) + t.output;
}

/** 区间长度（天），下限 1 小时，避免日均在极短区间里爆炸 */
export function rangeDays(startTime: string, endTime: string): number {
  const ms = new Date(endTime).getTime() - new Date(startTime).getTime();
  return Math.max(ms, HOUR_MS) / DAY_MS;
}

export function peakBucket(series: DashboardStatsPoint[]): number {
  return series.reduce((m, p) => Math.max(m, p.requests), 0);
}

/**
 * 按指标总量取前 n 个模型，其余合并为 OTHER_KEY；返回 recharts 堆叠所需的宽表。
 * keys 按总量降序，OTHER_KEY（若存在）恒在最后。
 */
export function foldTopModels(
  rows: DashboardModelPoint[],
  metric: ModelMetric,
  n = 5,
): { keys: string[]; data: Array<{ time: string } & Record<string, number>> } {
  const totals = new Map<string, number>();
  for (const r of rows) totals.set(r.modelId, (totals.get(r.modelId) ?? 0) + r[metric]);
  const ranked = [...totals.entries()].sort((a, b) => b[1] - a[1]).map(([k]) => k);
  const top = new Set(ranked.slice(0, n));
  const hasOther = ranked.length > n;

  const byTime = new Map<string, { time: string } & Record<string, number>>();
  for (const r of rows) {
    let point = byTime.get(r.time);
    if (!point) {
      point = { time: r.time } as { time: string } & Record<string, number>;
      byTime.set(r.time, point);
    }
    const key = top.has(r.modelId) ? r.modelId : OTHER_KEY;
    point[key] = (point[key] ?? 0) + r[metric];
  }
  const keys = ranked.slice(0, n);
  if (hasOther) keys.push(OTHER_KEY);
  const data = [...byTime.values()].sort((a, b) => a.time.localeCompare(b.time));
  return { keys, data };
}

/** 7×24 矩阵（行 = 周一…周日，列 = 0…23 时），缺失补 0，越界格子忽略 */
export function heatmapMatrix(cells: DashboardHeatmapCell[]): number[][] {
  const m = Array.from({ length: 7 }, () => Array<number>(24).fill(0));
  for (const c of cells) {
    if (c.weekday >= 0 && c.weekday < 7 && c.hour >= 0 && c.hour < 24)
      m[c.weekday][c.hour] += c.count;
  }
  return m;
}

const DASH = "\u2014";

/**
 * 环比文案：用量类为相对百分比，比率类为百分点（pt），延迟为时长差；无环比渲染「—」。
 * 箭头只表达涨跌方向，好坏由 deltaTone 着色。
 */
export function formatDelta(kind: KpiKind, delta: number | null): string {
  if (delta == null || !Number.isFinite(delta)) return DASH;
  const arrow = delta >= 0 ? "\u25B2" : "\u25BC";
  const abs = Math.abs(delta);
  if (kind === "latency") return `${arrow} ${formatDurationMs(abs)}`;
  if (kind === "successRate" || kind === "cacheHitRate") {
    return `${arrow} ${formatDecimal(abs * 100, 1)}pt`;
  }
  return `${arrow} ${formatDecimal(abs * 100, 1)}%`;
}

/** 0-1 比率 → 百分比文案（null 渲染「—」） */
export function formatRate(rate: number | null | undefined, digits = 1): string {
  if (rate == null || !Number.isFinite(rate)) return DASH;
  return `${formatDecimal(rate * 100, digits)}%`;
}

/** KPI 用短金额：≥1 保留 2 位小数，<1 保留 4 位；带千分位 */
export function formatCostShort(v: number | null | undefined, currency?: string): string {
  if (v == null || !Number.isFinite(v)) return DASH;
  const symbol = currency === "USD" ? "$" : "";
  return `${symbol}${formatDecimal(v, Math.abs(v) >= 1 ? 2 : 4)}`;
}
