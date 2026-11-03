import { formatDecimal, formatDurationMs } from "./utils";
import type {
  DashboardFailure,
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

/** 可用性状态条的固定桶数：由前端经 overview 的 query 传给后端，后端决定桶宽 */
export const UPTIME_BUCKETS = 32;

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

export function successRate(s: Pick<DashboardStats, "requests" | "success">): number | null {
  return s.requests > 0 ? s.success / s.requests : null;
}

/** 失败状态码展示名：-1 / 0 是网关内部约定码，需要 i18n 文案；其余直接显示 HTTP 状态码 */
export function failureCodeLabel(statusCode: number, t: (key: string) => string): string {
  if (statusCode === -1) return t("dashboard.failure.connection");
  if (statusCode === 0) return t("dashboard.failure.unknown");
  return String(statusCode);
}

/**
 * 按可容纳行数决定展示几个状态码：全部放得下就全展示，否则留一行给「其余」。
 * 行数只受容器实测高度约束（不设人为上限，长尾也能铺开）；至少返回 0（只剩「其余」一行）。
 */
export function fitFailureTopK(codes: number, capacityRows: number): number {
  const maxRows = Math.max(1, capacityRows);
  return codes <= maxRows ? codes : maxRows - 1;
}

/** 取前 k 个失败状态码（后端已按 count 降序），其余合并为 rest（保留明细供浮层展示）；rest 为 null 表示无剩余 */
export function topFailures(
  failures: DashboardFailure[],
  k = failures.length,
): { top: DashboardFailure[]; rest: { items: DashboardFailure[]; count: number } | null } {
  const top = failures.slice(0, k);
  const items = failures.slice(k);
  if (items.length === 0) return { top, rest: null };
  return { top, rest: { items, count: items.reduce((s, f) => s + f.count, 0) } };
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

export const MODEL_METRICS: readonly ModelMetric[] = ["cost", "requests", "tokens"];

/** 持久化值可能是脏数据：只接受三种指标，其余回落到费用 */
export function resolveModelMetric(value: unknown): ModelMetric {
  return MODEL_METRICS.find((m) => m === value) ?? "cost";
}

export interface PointModelEntry {
  key: string;
  value: number;
}

/**
 * 每个时间点独立取 Top n 模型（只计正值，零值不展示）；其余合计为 OTHER_KEY 并恒置末尾，不参与排名。
 * 供图例 / tooltip 使用：与堆叠图的全局 Top n 互不影响。返回 时间戳(ms) → 有序条目。
 */
export function rankModelsPerPoint(
  rows: DashboardModelPoint[],
  metric: ModelMetric,
  n = 5,
): Map<number, PointModelEntry[]> {
  const byTime = new Map<number, Map<string, number>>();
  for (const r of rows) {
    const t = new Date(r.time).getTime();
    let models = byTime.get(t);
    if (!models) {
      models = new Map();
      byTime.set(t, models);
    }
    models.set(r.modelId, (models.get(r.modelId) ?? 0) + r[metric]);
  }
  const out = new Map<number, PointModelEntry[]>();
  for (const [t, models] of byTime) {
    const ranked = [...models.entries()].filter(([, v]) => v > 0).sort((a, b) => b[1] - a[1]);
    const entries: PointModelEntry[] = ranked.slice(0, n).map(([key, value]) => ({ key, value }));
    const rest = ranked.slice(n).reduce((s, [, v]) => s + v, 0);
    if (rest > 0) entries.push({ key: OTHER_KEY, value: rest });
    out.set(t, entries);
  }
  return out;
}

/** 状态条分档：失败率 < 1% ok、1–5% warn、≥ 5% bad，无请求 idle（与 Endpoint 健康点同一阈值） */
export type UptimeLevel = "ok" | "warn" | "bad" | "idle";

export function uptimeLevel(seg: { requests: number; success: number }): UptimeLevel {
  if (seg.requests <= 0) return "idle";
  const errorRate = (seg.requests - seg.success) / seg.requests;
  if (errorRate >= 0.05) return "bad";
  if (errorRate >= 0.01) return "warn";
  return "ok";
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

/** 环比百分比上限：上一周期基数极小时比值会失真（3K → 2.9M 读作 ▲91,814.9%），
 *  超过 999% 一律归并为「999%+」，避免天文数字抢走卡片视线——它已经表达完了
 *  「涨很多」这个信息，多余位数不提供任何额外判断。 */
const DELTA_PCT_CAP = 999;

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
  const pct = abs * 100;
  if (pct > DELTA_PCT_CAP) return `${arrow} ${DELTA_PCT_CAP}%+`;
  return `${arrow} ${formatDecimal(pct, 1)}%`;
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
