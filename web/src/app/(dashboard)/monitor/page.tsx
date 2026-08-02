"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Activity, MemoryStick, Radio } from "lucide-react";

import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { usePersistentState } from "@/hooks/use-persistent-state";
import type { RuntimeInstanceSeries, RuntimePoint } from "@/lib/types";
import { cn } from "@/lib/utils";
import { RuntimeGaugeCard } from "@/components/charts/runtime-gauge-card";
import { RuntimeChart } from "@/components/charts/runtime-chart";
import { useChartSeriesColors } from "@/lib/theme";

const POLL_INTERVAL_MS = 5000;

const RANGE_KEYS = ["15m", "1h", "6h", "24h"] as const;
type RangeKey = (typeof RANGE_KEYS)[number];

const RANGE_WINDOW_SEC: Record<RangeKey, number> = {
  "15m": 900,
  "1h": 3600,
  "6h": 21600,
  "24h": 86400,
};

type Pt = RuntimePoint;

interface InstanceState {
  goroutines: Pt[];
  heapMB: Pt[];
  cpuPercent: Pt[];
}

interface SeriesState {
  qps: Pt[];
  p95Ms: Pt[];
  sseActive: Record<string, Pt[]>;
  tokenInput: Pt[];
  tokenOutput: Pt[];
  successRate: Pt[];
  instances: Record<string, InstanceState>;
}

const EMPTY_INSTANCE: InstanceState = { goroutines: [], heapMB: [], cpuPercent: [] };

const EMPTY_STATE: SeriesState = {
  qps: [],
  p95Ms: [],
  sseActive: {},
  tokenInput: [],
  tokenOutput: [],
  successRate: [],
  instances: {},
};

function mergePoints(prev: Pt[], incoming: Pt[], cutoff: number): Pt[] {
  const map = new Map<number, number>();
  for (const p of prev) map.set(p.time, p.value);
  for (const p of incoming) map.set(p.time, p.value);
  return [...map.entries()]
    .filter(([t]) => t >= cutoff)
    .sort((a, b) => a[0] - b[0])
    .map(([time, value]) => ({ time, value }));
}

function mergeSSE(
  prev: Record<string, Pt[]>,
  incoming: Record<string, Pt[]>,
  cutoff: number,
): Record<string, Pt[]> {
  const providers = new Set([...Object.keys(prev), ...Object.keys(incoming)]);
  const out: Record<string, Pt[]> = {};
  for (const prov of providers) {
    out[prov] = mergePoints(prev[prov] ?? [], incoming[prov] ?? [], cutoff);
  }
  return out;
}

// mergeInstances 按 pod 名逐实例增量合并各指标曲线。
function mergeInstances(
  prev: Record<string, InstanceState>,
  incoming: Record<string, RuntimeInstanceSeries>,
  cutoff: number,
): Record<string, InstanceState> {
  const pods = new Set([...Object.keys(prev), ...Object.keys(incoming)]);
  const out: Record<string, InstanceState> = {};
  for (const pod of pods) {
    const p = prev[pod] ?? EMPTY_INSTANCE;
    const inc = incoming[pod] ?? EMPTY_INSTANCE;
    out[pod] = {
      goroutines: mergePoints(p.goroutines, inc.goroutines ?? [], cutoff),
      heapMB: mergePoints(p.heapMB, inc.heapMB ?? [], cutoff),
      cpuPercent: mergePoints(p.cpuPercent, inc.cpuPercent ?? [], cutoff),
    };
  }
  return out;
}

// podChartData 把各 pod 的同一指标曲线合并为同一时间轴上的多列（列名 = pod 名）。
function podChartData(instances: Record<string, InstanceState>, metric: keyof InstanceState): Array<Record<string, number>> {
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

// podSumLatest 各 pod 最新值求和（用于头部卡片集群总和）。
function podSumLatest(instances: Record<string, InstanceState>, metric: keyof InstanceState): number {
  let sum = 0;
  for (const inst of Object.values(instances)) {
    sum += inst[metric].at(-1)?.value ?? 0;
  }
  return sum;
}

function lastValue(points: Pt[]): number {
  return points.at(-1)?.value ?? 0;
}

function toChartData(points: Pt[]): Array<Record<string, number>> {
  return points.map((p) => ({ time: p.time, value: p.value }));
}

function sseChartData(sse: Record<string, Pt[]>): Array<Record<string, number>> {
  const rows = new Map<number, Record<string, number>>();
  for (const [prov, points] of Object.entries(sse)) {
    for (const p of points) {
      const row = rows.get(p.time) ?? { time: p.time };
      row[prov] = p.value;
      rows.set(p.time, row);
    }
  }
  return [...rows.values()].sort((a, b) => a.time - b.time);
}

// tpsChartData 把输入/输出两条 token 速率时序合并为同一时间轴上的两列（input/output）。
function tpsChartData(input: Pt[], output: Pt[]): Array<Record<string, number>> {
  const rows = new Map<number, Record<string, number>>();
  for (const p of input) {
    const row = rows.get(p.time) ?? { time: p.time };
    row.input = p.value;
    rows.set(p.time, row);
  }
  for (const p of output) {
    const row = rows.get(p.time) ?? { time: p.time };
    row.output = p.value;
    rows.set(p.time, row);
  }
  return [...rows.values()].sort((a, b) => a.time - b.time);
}

export default function MonitorPage() {
  const t = useT();
  const [range, setRange] = usePersistentState<RangeKey>("monitor.range", "1h");
  const [state, setState] = useState<SeriesState>(EMPTY_STATE);
  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<string>("--:--:--");
  const sinceRef = useRef(0);

  const poll = useCallback(
    async (rangeKey: RangeKey) => {
      try {
        const rsp = await api.getRuntimeMetrics({ range: rangeKey, since: sinceRef.current });
        const s = rsp.series ?? {};
        const now = Math.floor(Date.now() / 1000);
        const cutoff = now - RANGE_WINDOW_SEC[rangeKey];

        setState((prev) => ({
          qps: mergePoints(prev.qps, s.qps ?? [], cutoff),
          p95Ms: mergePoints(prev.p95Ms, s.p95Ms ?? [], cutoff),
          sseActive: mergeSSE(prev.sseActive, s.sseActive ?? {}, cutoff),
          tokenInput: mergePoints(prev.tokenInput, s.tokenInput ?? [], cutoff),
          tokenOutput: mergePoints(prev.tokenOutput, s.tokenOutput ?? [], cutoff),
          successRate: mergePoints(prev.successRate, s.successRate ?? [], cutoff),
          instances: mergeInstances(prev.instances, s.instances ?? {}, cutoff),
        }));

        if (rsp.latestTime > 0) sinceRef.current = rsp.latestTime;
        setLastUpdated(new Date().toLocaleTimeString([], { hour12: false }));
      } catch {
        // silently ignore polling errors
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  /* eslint-disable react-hooks/set-state-in-effect -- range 切换需重置时序状态并立即触发首次拉取 */
  useEffect(() => {
    sinceRef.current = 0;
    setState(EMPTY_STATE);
    setLoading(true);
    poll(range);
    const interval = setInterval(() => poll(range), POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [range, poll]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const sseProviders = Object.keys(state.sseActive).sort();
  const seriesColors = useChartSeriesColors();
  const sseSeries = sseProviders.map((prov, i) => ({
    key: prov,
    label: prov,
    color: seriesColors[i % seriesColors.length],
  }));
  const sseTotal = Math.round(sseProviders.reduce((sum, prov) => sum + lastValue(state.sseActive[prov]), 0));
  const pods = Object.keys(state.instances).sort();
  const podSeries = pods.map((pod, i) => ({
    key: pod,
    label: pod,
    color: seriesColors[i % seriesColors.length],
  }));
  const goroutinesTotal = Math.round(podSumLatest(state.instances, "goroutines"));
  const heapTotal = podSumLatest(state.instances, "heapMB");
  const tpsData = tpsChartData(state.tokenInput, state.tokenOutput);
  const tpsSeries = [
    { key: "input", label: t("monitor.request_tps_input"), color: seriesColors[0] },
    { key: "output", label: t("monitor.request_tps_output"), color: seriesColors[1] },
  ];

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
            {t("monitor.title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{t("monitor.subtitle")}</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="relative flex size-2">
              <span className="absolute inline-flex size-full animate-ping rounded-full opacity-60 bg-[#4A9E7D]" />
              <span className="relative inline-flex size-2 rounded-full bg-[#4A9E7D]" />
            </span>
            <span className="font-mono tabular-nums">5s · {lastUpdated}</span>
          </div>
          <div className="flex items-center gap-0.5 rounded-lg bg-muted p-0.5">
            {RANGE_KEYS.map((key) => (
              <button
                key={key}
                type="button"
                onClick={() => setRange(key)}
                className={cn(
                  "inline-flex h-8 items-center justify-center rounded-md px-3 text-xs font-medium transition-colors",
                  range === key
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {key}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <RuntimeGaugeCard label={t("monitor.goroutines")} value={goroutinesTotal} icon={<Activity className="size-4" />} tone="primary" loading={loading} />
        <RuntimeGaugeCard label={t("monitor.heap")} value={heapTotal} unit="MB" icon={<MemoryStick className="size-4" />} tone="blue" loading={loading} />
        <RuntimeGaugeCard label={t("monitor.sse_active")} value={sseTotal} icon={<Radio className="size-4" />} tone="violet" loading={loading} />
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <RuntimeChart title={t("monitor.request_qps")} data={toChartData(state.qps)} series={[{ key: "value", label: t("monitor.request_qps"), color: seriesColors[3] }]} unit=" req/s" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.request_tps")} data={tpsData} series={tpsSeries} unit=" tok/s" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.sse_active")} data={sseChartData(state.sseActive)} series={sseSeries} rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.cpu_usage")} data={podChartData(state.instances, "cpuPercent")} series={podSeries} unit="%" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.heap_memory")} data={podChartData(state.instances, "heapMB")} series={podSeries} unit=" MB" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.goroutines_chart")} data={podChartData(state.instances, "goroutines")} series={podSeries} rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.success_rate")} data={toChartData(state.successRate)} series={[{ key: "value", label: t("monitor.success_rate"), color: seriesColors[2] }]} unit="%" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.latency_p95")} data={toChartData(state.p95Ms)} series={[{ key: "value", label: t("monitor.latency_p95"), color: seriesColors[2] }]} unit=" ms" rangeKey={range} emptyLabel={t("monitor.collecting")} />
      </div>
    </div>
  );
}
