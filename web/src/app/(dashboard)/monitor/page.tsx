"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Activity, MemoryStick, Radio, Signal, Zap } from "lucide-react";

import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import type { MetricFamilyItem } from "@/lib/types";
import { RuntimeGaugeCard } from "@/components/charts/runtime-gauge-card";
import { RuntimeLineChart } from "@/components/charts/runtime-line-chart";

const POLL_INTERVAL_MS = 5000;
const MAX_DATA_POINTS = 60;

interface TimeSeries {
  time: string;
  value: number;
}

interface MonitorState {
  goroutines: TimeSeries[];
  heapMB: TimeSeries[];
  inProgress: TimeSeries[];
  sseActive: TimeSeries[];
  cpuPercent: TimeSeries[];
  qps: TimeSeries[];
  p95Ms: TimeSeries[];
}

function nowLabel(): string {
  return new Date().toLocaleTimeString("en-US", {
    hour12: false,
    minute: "2-digit",
    second: "2-digit",
  });
}

function findMetric(
  families: MetricFamilyItem[],
  name: string,
): MetricFamilyItem | undefined {
  return families.find((f) => f.name === name);
}

function getGaugeValue(
  families: MetricFamilyItem[],
  name: string,
): number {
  const m = findMetric(families, name);
  return m?.samples?.[0]?.value ?? 0;
}

function getCounterValue(
  families: MetricFamilyItem[],
  name: string,
): number {
  const m = findMetric(families, name);
  if (!m?.samples) return 0;
  return m.samples.reduce((sum, s) => sum + s.value, 0);
}

function getHistogramP95(
  families: MetricFamilyItem[],
  name: string,
): number {
  const m = findMetric(families, name);
  if (!m?.samples) return 0;
  const buckets = m.samples
    .filter((s) => s.labels?.le !== undefined && s.labels?.le !== "+Inf")
    .map((s) => ({ le: parseFloat(s.labels!.le), count: s.value }))
    .sort((a, b) => a.le - b.le);
  const total = m.samples.find((s) => s.labels?.le === "+Inf")?.value ?? 0;
  if (total === 0 || buckets.length === 0) return 0;
  const target = total * 0.95;
  for (let i = buckets.length - 1; i >= 0; i--) {
    if (buckets[i].count >= target) {
      return buckets[i].le * 1000;
    }
  }
  return 0;
}

export default function MonitorPage() {
  const t = useT();
  const [state, setState] = useState<MonitorState>({
    goroutines: [],
    heapMB: [],
    inProgress: [],
    sseActive: [],
    cpuPercent: [],
    qps: [],
    p95Ms: [],
  });
  const [currentValues, setCurrentValues] = useState({
    goroutines: 0,
    heapMB: 0,
    inProgress: 0,
    sseActive: 0,
  });
  const prevCpuRef = useRef<number | null>(null);
  const prevRequestCountRef = useRef<number | null>(null);
  const prevTimeRef = useRef<number | null>(null);

  const pushPoint = useCallback(
    (key: keyof MonitorState, time: string, value: number) => {
      setState((prev) => {
        const arr = [...prev[key], { time, value }];
        if (arr.length > MAX_DATA_POINTS) arr.shift();
        return { ...prev, [key]: arr };
      });
    },
    [],
  );

  useEffect(() => {
    const poll = async () => {
      try {
        const rsp = await api.getMetricsJSON();
        const families = rsp.metrics ?? [];
        const time = nowLabel();
        const now = Date.now() / 1000;

        const goroutines = getGaugeValue(families, "go_goroutines");
        const heapBytes = getGaugeValue(families, "go_memstats_alloc_bytes");
        const heapMB = heapBytes / (1024 * 1024);
        const inProgress = getGaugeValue(families, "http_requests_in_progress");
        const sseActive = getGaugeValue(families, "sse_active_connections");
        const cpuTotal = getGaugeValue(families, "process_cpu_seconds_total");

        setCurrentValues({
          goroutines,
          heapMB: Math.round(heapMB * 100) / 100,
          inProgress,
          sseActive,
        });

        pushPoint("goroutines", time, goroutines);
        pushPoint("heapMB", time, Math.round(heapMB * 100) / 100);
        pushPoint("inProgress", time, inProgress);
        pushPoint("sseActive", time, sseActive);

        if (prevCpuRef.current !== null && prevTimeRef.current !== null) {
          const cpuDelta = cpuTotal - prevCpuRef.current;
          const timeDelta = now - prevTimeRef.current;
          if (timeDelta > 0) {
            const cpuPercent = (cpuDelta / timeDelta) * 100;
            pushPoint("cpuPercent", time, Math.round(cpuPercent * 100) / 100);
          }
        }
        prevCpuRef.current = cpuTotal;

        const requestTotal = getCounterValue(families, "http_requests_total");
        if (prevRequestCountRef.current !== null && prevTimeRef.current !== null) {
          const reqDelta = requestTotal - prevRequestCountRef.current;
          const timeDelta = now - prevTimeRef.current;
          if (timeDelta > 0) {
            const qps = reqDelta / timeDelta;
            pushPoint("qps", time, Math.round(qps * 100) / 100);
          }
        }
        prevRequestCountRef.current = requestTotal;
        prevTimeRef.current = now;

        const p95 = getHistogramP95(families, "http_request_duration_seconds");
        pushPoint("p95Ms", time, Math.round(p95));
      } catch {
        // silently ignore polling errors
      }
    };

    poll();
    const interval = setInterval(poll, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [pushPoint]);

  const lastUpdated = state.goroutines.at(-1)?.time ?? "--:--:--";

  return (
    <div className="space-y-8">
      <div className="relative overflow-hidden rounded-3xl border border-border bg-card p-5 shadow-[0_24px_70px_rgba(92,62,29,0.10)] dark:shadow-[0_24px_70px_rgba(0,0,0,0.22)] sm:p-6">
        <div className="pointer-events-none absolute -right-20 -top-24 h-64 w-64 rounded-full bg-primary/15 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-28 left-1/3 h-56 w-56 rounded-full bg-[#5B8DB8]/12 blur-3xl" />
        <div className="relative flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-2xl">
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/70 px-3 py-1 text-xs font-medium text-muted-foreground supports-[backdrop-filter]:bg-background/55">
              <Signal className="size-3.5 text-primary" />
              <span>{t("monitor.telemetry")}</span>
            </div>
            <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
              {t("monitor.title")}
            </h1>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              {t("monitor.subtitle")}
            </p>
          </div>
          <div className="grid grid-cols-2 gap-2 text-sm sm:min-w-72">
            <div className="rounded-2xl border border-border/70 bg-background/70 p-3 supports-[backdrop-filter]:bg-background/55">
              <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground">{t("monitor.refresh")}</div>
              <div className="mt-1 font-display text-2xl font-semibold">5s</div>
            </div>
            <div className="rounded-2xl border border-border/70 bg-background/70 p-3 supports-[backdrop-filter]:bg-background/55">
              <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground">{t("monitor.updated")}</div>
              <div className="mt-1 font-mono text-lg font-semibold tabular-nums">{lastUpdated}</div>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <RuntimeGaugeCard
          label={t("monitor.goroutines")}
          value={currentValues.goroutines}
          icon={<Activity className="size-4" />}
          tone="primary"
        />
        <RuntimeGaugeCard
          label={t("monitor.heap")}
          value={currentValues.heapMB}
          unit="MB"
          icon={<MemoryStick className="size-4" />}
          tone="blue"
        />
        <RuntimeGaugeCard
          label={t("monitor.in_progress")}
          value={currentValues.inProgress}
          icon={<Zap className="size-4" />}
          tone="green"
        />
        <RuntimeGaugeCard
          label={t("monitor.sse_active")}
          value={currentValues.sseActive}
          icon={<Radio className="size-4" />}
          tone="violet"
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
        <RuntimeLineChart
          data={state.cpuPercent}
          dataKey="cpuPercent"
          label={t("monitor.cpu_usage")}
          unit="%"
          sampleLabel={t("monitor.samples")}
          className="xl:row-span-2"
          heightClassName="h-[320px]"
          accent="primary"
        />
        <RuntimeLineChart
          data={state.heapMB}
          dataKey="heapMB"
          label={t("monitor.heap_memory")}
          unit=" MB"
          sampleLabel={t("monitor.samples")}
          color="#5B8DB8"
          accent="blue"
        />
        <RuntimeLineChart
          data={state.qps}
          dataKey="qps"
          label={t("monitor.request_qps")}
          color="#5B8DB8"
          sampleLabel={t("monitor.samples")}
          accent="green"
        />
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <RuntimeLineChart
          data={state.p95Ms}
          dataKey="p95Ms"
          label={t("monitor.latency_p95")}
          unit=" ms"
          color="#7C6BA5"
          sampleLabel={t("monitor.samples")}
          accent="violet"
        />
        <RuntimeLineChart
          data={state.goroutines}
          dataKey="goroutines"
          label={t("monitor.goroutines_chart")}
          color="#4A9E7D"
          sampleLabel={t("monitor.samples")}
          accent="green"
        />
        <RuntimeLineChart
          data={state.inProgress}
          dataKey="inProgress"
          label={t("monitor.in_progress_requests")}
          color="#C76B8A"
          sampleLabel={t("monitor.samples")}
          accent="rose"
        />
      </div>
    </div>
  );
}
