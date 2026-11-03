"use client";

import { useMemo } from "react";

import { useI18n } from "@/lib/i18n";
import { formatBytes, formatDecimal, formatInteger, splitBytes } from "@/lib/utils";
import {
  RANGE_BUCKET_SEC,
  avgValue,
  lastValue,
  peakValue,
  podSumLatest,
  sumPodSeries,
  type Pt,
  type RangeKey,
  type SeriesState,
} from "@/lib/monitor";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { MetricNumber } from "@/components/motion/metric-number";
import { Sparkline } from "@/components/dashboard/sparkline";

interface KpiSpec {
  id: string;
  label: string;
  value: string;
  unit?: string;
  sub: string[];
  series: number[];
  lineClass: string;
}

function MonitorKpiCard({ spec, loading }: { spec: KpiSpec; loading: boolean }) {
  return (
    <Card size="sm" className="gap-1 px-3 py-2.5" data-interactive>
      <span className="text-[11px] font-medium tracking-wider whitespace-nowrap text-muted-foreground uppercase">
        {spec.label}
      </span>
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

const formatByteRate = (v: number) => `${formatBytes(v)}/s`;

/**
 * 6 张运维 KPI：请求速率 / CPU / 堆内存 / 协程 / 网络下行 / 网络上行。
 * 只看「进程是否健康」的资源与流量信号；用量、费用、成功率等业务指标留在 Dashboard。
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
    const pods = Object.keys(state.instances).length;
    const cpuSeries = sumPodSeries(state.instances, "cpuPercent");
    const cpuNow = podSumLatest(state.instances, "cpuPercent", state.latestTime, bucketSec);
    const heapSeries = sumPodSeries(state.instances, "heapMB");
    const heapNow = podSumLatest(state.instances, "heapMB", state.latestTime, bucketSec);
    const goroutineSeries = sumPodSeries(state.instances, "goroutines");
    const goroutinesNow = podSumLatest(state.instances, "goroutines", state.latestTime, bucketSec);

    const network = (id: "net_rx" | "net_tx", points: Pt[], lineClass: string): KpiSpec => {
      const now = splitBytes(lastValue(points));
      return {
        id,
        label: t(`monitor.${id}`),
        value: now.value,
        unit: now.unit ? `${now.unit}/s` : undefined,
        sub: [
          `${peak} ${formatByteRate(peakValue(points))}`,
          `${avg} ${formatByteRate(avgValue(points))}`,
        ],
        series: values(points),
        lineClass,
      };
    };

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
        id: "cpu",
        label: t("monitor.cpu_usage"),
        value: formatDecimal(cpuNow, 1),
        unit: "%",
        sub: [
          `${peak} ${formatDecimal(peakValue(cpuSeries), 1)}%`,
          `${formatInteger(pods)} ${t("monitor.instances")}`,
        ],
        series: values(cpuSeries),
        lineClass: "text-chart-2",
      },
      {
        id: "heap",
        label: t("monitor.heap_memory"),
        value: formatDecimal(heapNow, 1),
        unit: "MB",
        sub: [
          `${peak} ${formatDecimal(peakValue(heapSeries), 1)} MB`,
          `${avg} ${formatDecimal(avgValue(heapSeries), 1)} MB`,
        ],
        series: values(heapSeries),
        lineClass: "text-chart-4",
      },
      {
        id: "goroutines",
        label: t("monitor.goroutines"),
        value: formatInteger(Math.round(goroutinesNow)),
        sub: [
          `${peak} ${formatInteger(Math.round(peakValue(goroutineSeries)))}`,
          `${avg} ${formatInteger(Math.round(avgValue(goroutineSeries)))}`,
        ],
        series: values(goroutineSeries),
        lineClass: "text-chart-3",
      },
      network("net_rx", state.netRx, "text-primary"),
      network("net_tx", state.netTx, "text-chart-2"),
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
