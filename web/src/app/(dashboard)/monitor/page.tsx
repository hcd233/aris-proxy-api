"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Activity, Cpu, MemoryStick, Zap } from "lucide-react";

import { api } from "@/lib/api-client";
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

    const interval = setInterval(poll, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, [pushPoint]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Monitor</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Real-time runtime and business metrics (5s interval)
        </p>
      </div>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <RuntimeGaugeCard
          label="Goroutines"
          value={currentValues.goroutines}
          icon={<Activity className="size-4" />}
        />
        <RuntimeGaugeCard
          label="Heap"
          value={currentValues.heapMB}
          unit="MB"
          icon={<MemoryStick className="size-4" />}
        />
        <RuntimeGaugeCard
          label="In-Progress"
          value={currentValues.inProgress}
          icon={<Zap className="size-4" />}
        />
        <RuntimeGaugeCard
          label="SSE Active"
          value={currentValues.sseActive}
          icon={<Cpu className="size-4" />}
        />
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <RuntimeLineChart
          data={state.cpuPercent}
          dataKey="cpuPercent"
          label="CPU Usage"
          unit="%"
        />
        <RuntimeLineChart
          data={state.heapMB}
          dataKey="heapMB"
          label="Heap Memory"
          unit=" MB"
        />
        <RuntimeLineChart
          data={state.qps}
          dataKey="qps"
          label="Request QPS"
          color="#5B8DB8"
        />
        <RuntimeLineChart
          data={state.p95Ms}
          dataKey="p95Ms"
          label="Latency P95"
          unit=" ms"
          color="#7C6BA5"
        />
        <RuntimeLineChart
          data={state.goroutines}
          dataKey="goroutines"
          label="Goroutines"
          color="#4A9E7D"
        />
        <RuntimeLineChart
          data={state.inProgress}
          dataKey="inProgress"
          label="In-Progress Requests"
          color="#C76B8A"
        />
      </div>
    </div>
  );
}
