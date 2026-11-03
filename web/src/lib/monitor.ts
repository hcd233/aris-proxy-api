import type { RuntimeInstanceSeries, RuntimePoint } from "./types";

/* ============================================================
 * Monitor（运维监控）页面的纯逻辑：时序增量合并、跨 pod 聚合、状态码归类。
 * 与 Dashboard（业务指标）刻意分开：这里只处理 /metrics/runtime 的运维侧曲线。
 * ========================================================== */

export const RANGE_KEYS = ["15m", "1h", "6h", "24h"] as const;
export type RangeKey = (typeof RANGE_KEYS)[number];

/** 档位文案复用全站 time.* 键，与 TimeRangePicker 的选择器保持同一套 i18n 术语 */
export const RANGE_LABEL_KEYS: Record<RangeKey, string> = {
  "15m": "time.last_15m",
  "1h": "time.last_1h",
  "6h": "time.last_6h",
  "24h": "time.last_24h",
};

export const RANGE_WINDOW_SEC: Record<RangeKey, number> = {
  "15m": 900,
  "1h": 3600,
  "6h": 21600,
  "24h": 86400,
};

/** 各 range 档的聚合桶宽（与后端 metrics.ResolveRange 保持一致） */
export const RANGE_BUCKET_SEC: Record<RangeKey, number> = {
  "15m": 5,
  "1h": 60,
  "6h": 300,
  "24h": 900,
};

export type Pt = RuntimePoint;

export interface InstanceState {
  goroutines: Pt[];
  heapMB: Pt[];
  cpuPercent: Pt[];
  threads: Pt[];
}

export interface SeriesState {
  qps: Pt[];
  p95Ms: Pt[];
  sseActive: Record<string, Pt[]>;
  statusCodes: Record<string, Pt[]>;
  instances: Record<string, InstanceState>;
  /** 后端当前桶起点 unix 秒，用于判定活跃实例 */
  latestTime: number;
}

export const EMPTY_INSTANCE: InstanceState = {
  goroutines: [],
  heapMB: [],
  cpuPercent: [],
  threads: [],
};

export const EMPTY_STATE: SeriesState = {
  qps: [],
  p95Ms: [],
  sseActive: {},
  statusCodes: {},
  instances: {},
  latestTime: 0,
};

/** 增量合并：同一时间戳以新点为准，丢弃窗口外的旧点，按时间升序 */
export function mergePoints(prev: Pt[], incoming: Pt[], cutoff: number): Pt[] {
  const map = new Map<number, number>();
  for (const p of prev) map.set(p.time, p.value);
  for (const p of incoming) map.set(p.time, p.value);
  return [...map.entries()]
    .filter(([t]) => t >= cutoff)
    .sort((a, b) => a[0] - b[0])
    .map(([time, value]) => ({ time, value }));
}

/** 按 key（SSE provider / HTTP 状态码）逐曲线增量合并时序 */
export function mergeKeyedSeries(
  prev: Record<string, Pt[]>,
  incoming: Record<string, Pt[]>,
  cutoff: number,
): Record<string, Pt[]> {
  const keys = new Set([...Object.keys(prev), ...Object.keys(incoming)]);
  const out: Record<string, Pt[]> = {};
  for (const key of keys) {
    out[key] = mergePoints(prev[key] ?? [], incoming[key] ?? [], cutoff);
  }
  return out;
}

/**
 * 按 pod 名逐实例增量合并各指标曲线。
 * 后端只输出当前活跃实例（已下线实例不再返回），prev 中不在 incoming 的实例视为已消失，
 * 直接移除，避免滚动发布遗留的死实例永久残留。
 */
export function mergeInstances(
  prev: Record<string, InstanceState>,
  incoming: Record<string, RuntimeInstanceSeries>,
  cutoff: number,
): Record<string, InstanceState> {
  const out: Record<string, InstanceState> = {};
  for (const pod of Object.keys(incoming)) {
    const p = prev[pod] ?? EMPTY_INSTANCE;
    const inc = incoming[pod] ?? EMPTY_INSTANCE;
    out[pod] = {
      goroutines: mergePoints(p.goroutines, inc.goroutines ?? [], cutoff),
      heapMB: mergePoints(p.heapMB, inc.heapMB ?? [], cutoff),
      cpuPercent: mergePoints(p.cpuPercent, inc.cpuPercent ?? [], cutoff),
      threads: mergePoints(p.threads, inc.threads ?? [], cutoff),
    };
  }
  return out;
}

export function round2(v: number): number {
  return Math.round(v * 100) / 100;
}

export function lastValue(points: Pt[]): number {
  return points.at(-1)?.value ?? 0;
}

export function peakValue(points: Pt[]): number {
  let max = 0;
  for (const p of points) if (p.value > max) max = p.value;
  return max;
}

export function avgValue(points: Pt[]): number {
  if (points.length === 0) return 0;
  let sum = 0;
  for (const p of points) sum += p.value;
  return sum / points.length;
}

/** 曲线 → 图表行（单列 `value`） */
export function toChartData(points: Pt[]): Array<Record<string, number>> {
  return points.map((p) => ({ time: p.time, value: p.value }));
}

/** map<key, 时序点> → 同一时间轴上的多列（列名 = key），用于 SSE provider / 状态码类别 */
export function seriesMapChartData(
  seriesMap: Record<string, Pt[] | undefined>,
): Array<Record<string, number>> {
  const rows = new Map<number, Record<string, number>>();
  for (const [key, points] of Object.entries(seriesMap)) {
    for (const p of points ?? []) {
      const row = rows.get(p.time) ?? { time: p.time };
      row[key] = p.value;
      rows.set(p.time, row);
    }
  }
  return [...rows.values()].sort((a, b) => a.time - b.time);
}

/** 把各 pod 的同一指标曲线合并为同一时间轴上的多列（列名 = pod 名） */
export function podChartData(
  instances: Record<string, InstanceState>,
  metric: keyof InstanceState,
): Array<Record<string, number>> {
  const rows = new Map<number, Record<string, number>>();
  for (const [pod, inst] of Object.entries(instances)) {
    for (const p of inst[metric]) {
      const row = rows.get(p.time) ?? { time: p.time };
      row[pod] = p.value;
      rows.set(p.time, row);
    }
  }
  return [...rows.values()].sort((a, b) => a.time - b.time);
}

/**
 * 各 pod 最新值求和（头部卡片集群总和）。
 * 只统计「最近仍在产出数据」的活跃实例：后端实例注册表保留 24h，已下线 pod 的历史曲线
 * 若全部求和会把死实例计入集群总和。最新点落在当前桶起点（latestTime）往前一个桶内的实例视为活跃。
 */
export function podSumLatest(
  instances: Record<string, InstanceState>,
  metric: keyof InstanceState,
  latestTime: number,
  bucketSec: number,
): number {
  let sum = 0;
  for (const inst of Object.values(instances)) {
    const last = inst[metric].at(-1);
    if (last && (latestTime <= 0 || last.time >= latestTime - bucketSec)) {
      sum += last.value;
    }
  }
  return sum;
}

/** 各 pod 同一指标按时间点求和，得到集群总量曲线（用于 KPI 迷你趋势与峰值） */
export function sumPodSeries(
  instances: Record<string, InstanceState>,
  metric: keyof InstanceState,
): Pt[] {
  const totals = new Map<number, number>();
  for (const inst of Object.values(instances)) {
    for (const p of inst[metric]) totals.set(p.time, (totals.get(p.time) ?? 0) + p.value);
  }
  return [...totals.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([time, value]) => ({ time, value }));
}

/** 各 key 曲线按时间点求和（SSE 各 provider → 总连接数曲线） */
export function sumKeyedSeries(seriesMap: Record<string, Pt[]>): Pt[] {
  const totals = new Map<number, number>();
  for (const points of Object.values(seriesMap)) {
    for (const p of points) totals.set(p.time, (totals.get(p.time) ?? 0) + p.value);
  }
  return [...totals.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([time, value]) => ({ time, value }));
}

/* ---------- HTTP 状态码归类 ---------- */

export const STATUS_CLASSES = ["2xx", "3xx", "4xx", "5xx", "other"] as const;
export type StatusClass = (typeof STATUS_CLASSES)[number];

/** 200 → 2xx；1xx、网关内部约定码（-1 / 0）和其他非三位码归入 other */
export function statusClassOf(code: string): StatusClass {
  if (!/^[2-5]\d\d$/.test(code)) return "other";
  return `${code[0]}xx` as StatusClass;
}

/** 把逐状态码曲线折叠为按类别（2xx/3xx/4xx/5xx/other）求和的曲线；只保留窗口内出现过的类别 */
export function groupStatusClasses(
  statusCodes: Record<string, Pt[]>,
): Partial<Record<StatusClass, Pt[]>> {
  const buckets = new Map<StatusClass, Map<number, number>>();
  for (const [code, points] of Object.entries(statusCodes)) {
    const cls = statusClassOf(code);
    const byTime = buckets.get(cls) ?? new Map<number, number>();
    for (const p of points) byTime.set(p.time, (byTime.get(p.time) ?? 0) + p.value);
    buckets.set(cls, byTime);
  }
  const out: Partial<Record<StatusClass, Pt[]>> = {};
  for (const cls of STATUS_CLASSES) {
    const byTime = buckets.get(cls);
    if (!byTime) continue;
    const points = [...byTime.entries()]
      .sort((a, b) => a[0] - b[0])
      .map(([time, value]) => ({ time, value }));
    if (points.some((p) => p.value > 0)) out[cls] = points;
  }
  return out;
}

export interface StatusTotals {
  total: number;
  c4xx: number;
  c5xx: number;
}

/** 窗口内请求总数与 4xx / 5xx 次数 */
export function statusTotals(statusCodes: Record<string, Pt[]>): StatusTotals {
  const totals: StatusTotals = { total: 0, c4xx: 0, c5xx: 0 };
  for (const [code, points] of Object.entries(statusCodes)) {
    const cls = statusClassOf(code);
    for (const p of points) {
      totals.total += p.value;
      if (cls === "4xx") totals.c4xx += p.value;
      if (cls === "5xx") totals.c5xx += p.value;
    }
  }
  return totals;
}

/** 窗口 5xx 比率（0–1）；窗口内无请求时为 null，展示「—」而不是 0% */
export function serverErrorRatio(totals: StatusTotals): number | null {
  return totals.total > 0 ? totals.c5xx / totals.total : null;
}

/** 逐桶 5xx 比率曲线（0–1）：无请求的桶记 0（无请求即无错误），保证迷你趋势不断线 */
export function errorRatioSeries(statusCodes: Record<string, Pt[]>): Pt[] {
  const total = new Map<number, number>();
  const err = new Map<number, number>();
  for (const [code, points] of Object.entries(statusCodes)) {
    const is5xx = statusClassOf(code) === "5xx";
    for (const p of points) {
      total.set(p.time, (total.get(p.time) ?? 0) + p.value);
      if (is5xx) err.set(p.time, (err.get(p.time) ?? 0) + p.value);
    }
  }
  return [...total.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([time, sum]) => ({ time, value: sum > 0 ? (err.get(time) ?? 0) / sum : 0 }));
}

export type ErrorLevel = "ok" | "warn" | "bad";

/** 5xx 比率分档：<1% 正常，<5% 告警，其余异常；无请求视为正常 */
export function errorLevel(ratio: number | null): ErrorLevel {
  if (ratio == null || ratio < 0.01) return "ok";
  return ratio < 0.05 ? "warn" : "bad";
}

/* ---------- pod 名缩写 ---------- */

/**
 * 去掉所有 pod 名共有的前缀 token（`aris-proxy-api-7d9f8b6c5-x2k4j` → `x2k4j`），
 * 让图例与浮层里只剩区分实例的部分。某个 pod 名被前缀吃光时，整组不缩写。
 */
export function shortPodNames(pods: string[]): Record<string, string> {
  const identity = Object.fromEntries(pods.map((p) => [p, p]));
  if (pods.length < 2) return identity;
  const tokens = pods.map((p) => p.split("-"));
  const minLen = Math.min(...tokens.map((t) => t.length));
  let prefix = 0;
  while (prefix < minLen - 1 && tokens.every((t) => t[prefix] === tokens[0][prefix])) prefix++;
  if (prefix === 0) return identity;
  return Object.fromEntries(pods.map((p, i) => [p, tokens[i].slice(prefix).join("-")]));
}

/** 图表 X 轴 / 浮层的时间文案：短窗口带秒，24h 带日期 */
export function formatMonitorTick(unix: number, rangeKey: string): string {
  const d = new Date(unix * 1000);
  if (rangeKey === "15m") {
    return d.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  }
  if (rangeKey === "24h") {
    return d.toLocaleString([], {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  }
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
}
