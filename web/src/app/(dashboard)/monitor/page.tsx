"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Activity, MemoryStick, Radio } from "lucide-react";

import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { usePersistentState } from "@/hooks/use-persistent-state";
import type { RuntimePoint } from "@/lib/types";
import { cn } from "@/lib/utils";
import { RuntimeGaugeCard } from "@/components/charts/runtime-gauge-card";
import { RuntimeChart } from "@/components/charts/runtime-chart";

const POLL_INTERVAL_MS = 5000;

const RANGE_KEYS = ["15m", "1h", "6h", "24h"] as const;
type RangeKey = (typeof RANGE_KEYS)[number];

const RANGE_WINDOW_SEC: Record<RangeKey, number> = {
  "15m": 900,
  "1h": 3600,
  "6h": 21600,
  "24h": 86400,
};

const SSE_COLORS = ["#D97757", "#5B8DB8", "#7C6BA5", "#4A9E7D", "#C76B8A", "#8B7355"];

type Pt = RuntimePoint;

interface SeriesState {
  goroutines: Pt[];
  heapMB: Pt[];
  qps: Pt[];
  cpuPercent: Pt[];
  p95Ms: Pt[];
  sseActive: Record<string, Pt[]>;
}

const EMPTY_STATE: SeriesState = {
  goroutines: [],
  heapMB: [],
  qps: [],
  cpuPercent: [],
  p95Ms: [],
  sseActive: {},
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
          goroutines: mergePoints(prev.goroutines, s.goroutines ?? [], cutoff),
          heapMB: mergePoints(prev.heapMB, s.heapMB ?? [], cutoff),
          qps: mergePoints(prev.qps, s.qps ?? [], cutoff),
          cpuPercent: mergePoints(prev.cpuPercent, s.cpuPercent ?? [], cutoff),
          p95Ms: mergePoints(prev.p95Ms, s.p95Ms ?? [], cutoff),
          sseActive: mergeSSE(prev.sseActive, s.sseActive ?? {}, cutoff),
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
  const sseSeries = sseProviders.map((prov, i) => ({
    key: prov,
    label: prov,
    color: SSE_COLORS[i % SSE_COLORS.length],
  }));
  const sseTotal = sseProviders.reduce((sum, prov) => sum + lastValue(state.sseActive[prov]), 0);

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
        <RuntimeGaugeCard label={t("monitor.goroutines")} value={lastValue(state.goroutines)} icon={<Activity className="size-4" />} tone="primary" loading={loading} />
        <RuntimeGaugeCard label={t("monitor.heap")} value={lastValue(state.heapMB)} unit="MB" icon={<MemoryStick className="size-4" />} tone="blue" loading={loading} />
        <RuntimeGaugeCard label={t("monitor.sse_active")} value={sseTotal} icon={<Radio className="size-4" />} tone="violet" loading={loading} />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <RuntimeChart title={t("monitor.cpu_usage")} data={toChartData(state.cpuPercent)} series={[{ key: "value", label: t("monitor.cpu_usage"), color: "#D97757" }]} unit="%" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.heap_memory")} data={toChartData(state.heapMB)} series={[{ key: "value", label: t("monitor.heap_memory"), color: "#5B8DB8" }]} unit=" MB" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.request_qps")} data={toChartData(state.qps)} series={[{ key: "value", label: t("monitor.request_qps"), color: "#4A9E7D" }]} rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.latency_p95")} data={toChartData(state.p95Ms)} series={[{ key: "value", label: t("monitor.latency_p95"), color: "#7C6BA5" }]} unit=" ms" rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.goroutines_chart")} data={toChartData(state.goroutines)} series={[{ key: "value", label: t("monitor.goroutines_chart"), color: "#4A9E7D" }]} rangeKey={range} emptyLabel={t("monitor.collecting")} />
        <RuntimeChart title={t("monitor.sse_active")} data={sseChartData(state.sseActive)} series={sseSeries} rangeKey={range} emptyLabel={t("monitor.collecting")} />
      </div>
    </div>
  );
}
