"use client";

import { useMemo } from "react";

import { useI18n } from "@/lib/i18n";
import { cn, formatDecimal, formatDurationMs, formatInteger } from "@/lib/utils";
import { formatRate } from "@/lib/dashboard";
import {
  RANGE_BUCKET_SEC,
  avgValue,
  errorLevel,
  errorRatioSeries,
  lastValue,
  peakValue,
  podSumLatest,
  serverErrorRatio,
  statusTotals,
  sumKeyedSeries,
  sumPodSeries,
  type ErrorLevel,
  type Pt,
  type RangeKey,
  type SeriesState,
} from "@/lib/monitor";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { MetricNumber } from "@/components/motion/metric-number";
import { Sparkline } from "@/components/dashboard/sparkline";

const LEVEL_TEXT: Record<ErrorLevel, string> = {
  ok: "text-positive",
  warn: "text-warning",
  bad: "text-negative",
};

interface KpiSpec {
  id: string;
  label: string;
  value: string;
  unit?: string;
  badge?: { text: string; className: string };
  sub: string[];
  series: number[];
  lineClass: string;
}

function MonitorKpiCard({ spec, loading }: { spec: KpiSpec; loading: boolean }) {
  return (
    <Card size="sm" className="gap-1 px-3 py-2.5" data-interactive>
      <div className="flex items-center justify-between gap-2">
        <span className="text-[11px] font-medium tracking-wider whitespace-nowrap text-muted-foreground uppercase">
          {spec.label}
        </span>
        {!loading && spec.badge && (
          <span className={cn("text-[11px] font-semibold whitespace-nowrap", spec.badge.className)}>
            {spec.badge.text}
          </span>
        )}
      </div>
      {loading ? (
        <>
          <Skeleton className="h-7 w-24" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-6 w-full" />
        </>
      ) : (
        <>
          <div className="font-display text-2xl leading-tight font-normal whitespace-nowrap text-foreground tabular-nums">
            <MetricNumber value={spec.value} />
            {spec.unit && <span className="ml-1 text-xs text-muted-foreground">{spec.unit}</span>}
          </div>
          <div className="min-h-[2lh] text-[11px] leading-snug text-muted-foreground tabular-nums">
            {spec.sub.join(" · ")}
          </div>
          <Sparkline values={spec.series} className={spec.lineClass} />
        </>
      )}
    </Card>
  );
}

const values = (points: Pt[]) => points.map((p) => p.value);

/** 请求量级自适应小数位：个位数看 2 位，百位以上取整 */
function formatRateValue(v: number): string {
  return formatDecimal(v, v >= 100 ? 0 : v >= 10 ? 1 : 2);
}

/**
 * 6 张运维 KPI：请求速率 / 5xx 错误率 / P95 延迟 / SSE 连接 / CPU / 堆内存。
 * 全部是「系统是否健康」的信号；用量、费用、缓存命中等业务指标留在 Dashboard。
 */
export function MonitorKpiRow({
  state,
  range,
  loading,
}: {
  state: SeriesState;
  range: RangeKey;
  loading: boolean;
}) {
  const { t, locale } = useI18n();
  const isLoading = loading && state.qps.length === 0;

  const specs = useMemo<KpiSpec[]>(() => {
    const bucketSec = RANGE_BUCKET_SEC[range];
    const peak = t("monitor.kpi.peak");
    const avg = t("monitor.kpi.avg");

    const qpsNow = lastValue(state.qps);
    const totals = statusTotals(state.statusCodes);
    const ratio = serverErrorRatio(totals);
    const level = errorLevel(ratio);

    const p95Active = state.p95Ms.filter((p) => p.value > 0);

    const sseSeries = sumKeyedSeries(state.sseActive);
    const topProviders = Object.entries(state.sseActive)
      .map(([prov, pts]) => [prov, Math.round(lastValue(pts))] as const)
      .filter(([, n]) => n > 0)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 2)
      .map(([prov, n]) => `${prov} ${formatInteger(n)}`);

    const pods = Object.keys(state.instances).length;
    const cpuSeries = sumPodSeries(state.instances, "cpuPercent");
    const cpuNow = podSumLatest(state.instances, "cpuPercent", state.latestTime, bucketSec);
    const heapSeries = sumPodSeries(state.instances, "heapMB");
    const heapNow = podSumLatest(state.instances, "heapMB", state.latestTime, bucketSec);
    const goroutines = podSumLatest(state.instances, "goroutines", state.latestTime, bucketSec);
    const threads = podSumLatest(state.instances, "threads", state.latestTime, bucketSec);

    return [
      {
        id: "qps",
        label: t("monitor.request_qps"),
        value: formatRateValue(qpsNow),
        unit: "req/s",
        sub: [
          `${peak} ${formatRateValue(peakValue(state.qps))}`,
          `${avg} ${formatRateValue(avgValue(state.qps))}`,
        ],
        series: values(state.qps),
        lineClass: "text-primary",
      },
      {
        id: "errors",
        label: t("monitor.kpi.error_rate"),
        value: formatRate(ratio, 2),
        badge: { text: t(`monitor.level.${level}`), className: LEVEL_TEXT[level] },
        sub: [
          `5xx ${formatInteger(Math.round(totals.c5xx))}`,
          `4xx ${formatInteger(Math.round(totals.c4xx))}`,
        ],
        series: values(errorRatioSeries(state.statusCodes)),
        lineClass: LEVEL_TEXT[level],
      },
      {
        id: "p95",
        label: t("monitor.latency_p95"),
        value: formatDurationMs(lastValue(state.p95Ms)),
        sub: [
          `${peak} ${formatDurationMs(peakValue(state.p95Ms))}`,
          `${avg} ${formatDurationMs(avgValue(p95Active))}`,
        ],
        series: values(state.p95Ms),
        lineClass: "text-chart-3",
      },
      {
        id: "sse",
        label: t("monitor.sse_active"),
        value: formatInteger(Math.round(lastValue(sseSeries))),
        sub: [
          `${peak} ${formatInteger(Math.round(peakValue(sseSeries)))}`,
          topProviders.length > 0 ? topProviders.join(" · ") : "\u2014",
        ],
        series: values(sseSeries),
        lineClass: "text-chart-2",
      },
      {
        id: "cpu",
        label: t("monitor.cpu_usage"),
        value: formatDecimal(cpuNow, 1),
        unit: "%",
        sub: [
          `${peak} ${formatDecimal(peakValue(cpuSeries), 1)}%`,
          `${formatInteger(pods)} ${t("monitor.instances")}`,
        ],
        series: values(cpuSeries),
        lineClass: "text-primary",
      },
      {
        id: "heap",
        label: t("monitor.heap_memory"),
        value: formatDecimal(heapNow, 1),
        unit: "MB",
        sub: [
          `${t("monitor.goroutines")} ${formatInteger(Math.round(goroutines))}`,
          `${t("monitor.threads")} ${formatInteger(Math.round(threads))}`,
        ],
        series: values(heapSeries),
        lineClass: "text-chart-4",
      },
    ];
    // locale 驱动翻译刷新（t 引用稳定，见 lib/i18n.tsx）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state, range, locale]);

  return (
    <div className="stagger-rise grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6">
      {specs.map((spec) => (
        <MonitorKpiCard key={spec.id} spec={spec} loading={isLoading} />
      ))}
    </div>
  );
}
