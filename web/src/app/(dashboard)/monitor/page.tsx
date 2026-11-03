"use client";

import { useMemo } from "react";

import { useI18n } from "@/lib/i18n";
import { useChartSeriesColors, useStatusColors } from "@/lib/theme";
import { MONITOR_REFRESH_INTERVALS } from "@/lib/refresh-interval";
import {
  RANGE_BUCKET_SEC,
  RANGE_KEYS,
  RANGE_LABEL_KEYS,
  STATUS_CLASSES,
  groupStatusClasses,
  lastValue,
  podChartData,
  podSumLatest,
  seriesMapChartData,
  shortPodNames,
  statusTotals,
  toChartData,
  type RangeKey,
  type StatusClass,
} from "@/lib/monitor";
import { cn, formatDecimal, formatDurationMs, formatInteger, formatTokenCount } from "@/lib/utils";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { PermissionGuard } from "@/components/permission-guard";
import { AutoRefreshButton } from "@/components/shared/auto-refresh-button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import {
  TooltipContent,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Panel } from "@/components/dashboard/panel";
import { MonitorChart, type MonitorChartSeries } from "@/components/monitor/monitor-chart";
import { MonitorKpiRow } from "@/components/monitor/monitor-kpi-row";
import { useRuntimeMetrics } from "@/components/monitor/use-runtime-metrics";

/* 图表数值格式化：模块级函数，引用稳定（内联函数会让 recharts 重算坐标并重放曲线动画） */
const fmtScalar = (v: number) => formatDecimal(v, v >= 100 ? 0 : v >= 10 ? 1 : 2);
const fmtQps = (v: number) => `${fmtScalar(v)} req/s`;
const fmtCount = (v: number) => formatInteger(Math.round(v));
const fmtPercent = (v: number) => `${formatDecimal(v, 1)}%`;
const fmtMemory = (v: number) => `${formatDecimal(v, 1)} MB`;
const fmtMemoryAxis = (v: number) => formatDecimal(v, 0);
const fmtLatencyAxis = (v: number) =>
  v >= 1000 ? `${formatDecimal(v / 1000, 1)}s` : `${formatDecimal(v, 0)}ms`;

const RUNTIME_METRICS = ["goroutines", "threads"] as const;
type RuntimeMetric = (typeof RUNTIME_METRICS)[number];

/** 面板右上角的「当前值」：等宽小字，和 Dashboard 的 Reliability 面板同一写法 */
function NowValue({ children }: { children: React.ReactNode }) {
  return (
    <span className="font-mono text-[11px] text-muted-foreground tabular-nums">{children}</span>
  );
}

/** 标题旁的实例状态：一眼看出采集是否正常、几个 pod 在线；悬停列出 pod 名 */
function InstanceStatus({
  pods,
  loading,
  failed,
}: {
  pods: string[];
  loading: boolean;
  failed: boolean;
}) {
  const { t } = useI18n();
  if (loading) return null;
  const online = pods.length > 0 && !failed;
  const label = failed
    ? t("monitor.fetch_failed")
    : pods.length === 0
      ? t("monitor.no_instances")
      : t("monitor.instances_online").replace("{count}", String(pods.length));
  const dot = failed ? "bg-negative" : online ? "bg-positive" : "bg-muted-foreground/40";
  const pill = (
    <span className="inline-flex cursor-default items-center gap-1.5 rounded-full border border-border/70 px-2 py-0.5 text-[11px] text-muted-foreground">
      <span aria-hidden className={cn("size-1.5 rounded-full", dot)} />
      {label}
    </span>
  );
  if (pods.length === 0) return pill;
  return (
    <TooltipRoot>
      <TooltipTrigger render={pill} />
      <TooltipContent className="max-w-xs text-xs break-all">
        <ul className="flex flex-col gap-0.5 font-mono">
          {pods.map((pod) => (
            <li key={pod}>{pod}</li>
          ))}
        </ul>
      </TooltipContent>
    </TooltipRoot>
  );
}

/**
 * 运维监控（与 Dashboard 的业务指标刻意分工）：页头 → 6 张健康 KPI →
 * 流量/状态码/延迟（5:3:3）→ 资源与连接（CPU / 内存 / 协程线程 / SSE，等分四栏）。
 * 版式与 Dashboard 同构：xl 以上锁定视口高度，面板内部自适应撑满；更窄时纵向堆叠滚动。
 */
export default function MonitorPage() {
  const { t, locale } = useI18n();
  const {
    range,
    setRange,
    refreshMs,
    setRefreshMs,
    state,
    loading,
    fetching,
    failed,
    cycle,
    refreshNow,
  } = useRuntimeMetrics();
  const [storedRuntime, setStoredRuntime] = usePersistentState<string>(
    "monitor.runtimeMetric",
    "goroutines",
  );
  const runtimeMetric: RuntimeMetric = storedRuntime === "threads" ? "threads" : "goroutines";

  const colors = useChartSeriesColors();
  const status = useStatusColors();
  const bucketSec = RANGE_BUCKET_SEC[range];

  // 每张图的数据只随它自己的曲线变化而重建：引用不变，recharts 就不会重放动画
  const qpsData = useMemo(() => toChartData(state.qps), [state.qps]);
  const p95Data = useMemo(() => toChartData(state.p95Ms), [state.p95Ms]);
  const sseData = useMemo(() => seriesMapChartData(state.sseActive), [state.sseActive]);
  const cpuData = useMemo(() => podChartData(state.instances, "cpuPercent"), [state.instances]);
  const heapData = useMemo(() => podChartData(state.instances, "heapMB"), [state.instances]);
  const runtimeData = useMemo(
    () => podChartData(state.instances, runtimeMetric),
    [state.instances, runtimeMetric],
  );
  const grouped = useMemo(() => groupStatusClasses(state.statusCodes), [state.statusCodes]);
  const statusData = useMemo(() => seriesMapChartData(grouped), [grouped]);

  const pods = useMemo(() => Object.keys(state.instances).sort(), [state.instances]);
  const podSeries = useMemo<MonitorChartSeries[]>(() => {
    const short = shortPodNames(pods);
    return pods.map((pod, i) => ({
      key: pod,
      label: short[pod],
      color: colors[i % colors.length],
    }));
  }, [pods, colors]);

  const sseSeries = useMemo<MonitorChartSeries[]>(
    () =>
      Object.keys(state.sseActive)
        .sort()
        .map((prov, i) => ({ key: prov, label: prov, color: colors[i % colors.length] })),
    [state.sseActive, colors],
  );

  const statusSeries = useMemo<MonitorChartSeries[]>(() => {
    const color: Record<StatusClass, string> = {
      "2xx": status.positive,
      "3xx": status.muted,
      "4xx": status.warning,
      "5xx": status.negative,
      other: colors[4 % colors.length],
    };
    return STATUS_CLASSES.filter((cls) => grouped[cls]).map((cls) => ({
      key: cls,
      label: cls === "other" ? t("monitor.status_other") : cls,
      color: color[cls],
    }));
    // locale 驱动「其他」文案刷新（t 引用稳定，见 lib/i18n.tsx）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [grouped, status, colors, locale]);

  const qpsSeries = useMemo<MonitorChartSeries[]>(
    () => [{ key: "value", label: t("monitor.request_qps"), color: colors[0] }],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [colors, locale],
  );
  const p95Series = useMemo<MonitorChartSeries[]>(
    () => [{ key: "value", label: t("monitor.latency_p95"), color: colors[2 % colors.length] }],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [colors, locale],
  );

  const totals = statusTotals(state.statusCodes);
  const sseNow = Math.round(
    Object.values(state.sseActive).reduce((sum, pts) => sum + lastValue(pts), 0),
  );
  const cpuNow = podSumLatest(state.instances, "cpuPercent", state.latestTime, bucketSec);
  const heapNow = podSumLatest(state.instances, "heapMB", state.latestTime, bucketSec);
  const runtimeNow = Math.round(
    podSumLatest(state.instances, runtimeMetric, state.latestTime, bucketSec),
  );

  const panelState = {
    emptyLabel: t("monitor.collecting"),
    loading,
    error: failed,
    onRetry: refreshNow,
  };

  return (
    <PermissionGuard module="monitor">
      <TooltipProvider>
        <div
          data-page-wide
          className="flex flex-col gap-3 xl:h-[calc(100dvh-5rem)] xl:min-h-[680px]"
        >
          <header className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
            <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
              <h1 className="motion-title font-display text-2xl tracking-tight text-foreground">
                {t("monitor.title")}
              </h1>
              <InstanceStatus pods={pods} loading={loading} failed={failed} />
            </div>
            {/* 窄屏独占一行并撑满：范围条 + 刷新按钮不再把自己挤出视口 */}
            <div className="flex max-w-full min-w-0 items-center gap-2 max-sm:w-full max-sm:justify-between">
              <ToggleGroup
                variant="segmented"
                spacing={0.5}
                className="no-scrollbar max-w-full overflow-x-auto"
                value={[range]}
                onValueChange={(next) => {
                  if (next[0]) setRange(next[0] as RangeKey);
                }}
              >
                {RANGE_KEYS.map((key) => (
                  <ToggleGroupItem key={key} value={key}>
                    {t(RANGE_LABEL_KEYS[key])}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
              <AutoRefreshButton
                intervalMs={refreshMs}
                options={MONITOR_REFRESH_INTERVALS}
                onIntervalChange={setRefreshMs}
                cycle={cycle}
                onRefresh={refreshNow}
                fetching={fetching}
              />
            </div>
          </header>

          <MonitorKpiRow state={state} range={range} loading={loading} />

          <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_minmax(0,1fr)] gap-3 max-xl:grid-rows-none">
            {/* 流量面：请求速率、状态码构成、尾延迟 */}
            <div className="stagger-rise grid min-h-0 gap-3 max-xl:auto-rows-[18rem] lg:grid-cols-2 xl:grid-cols-[5fr_3fr_3fr]">
              <Panel
                title={t("monitor.request_qps")}
                empty={qpsData.length === 0}
                action={<NowValue>{fmtQps(lastValue(state.qps))}</NowValue>}
                {...panelState}
              >
                <MonitorChart
                  kind="area"
                  data={qpsData}
                  series={qpsSeries}
                  rangeKey={range}
                  format={fmtQps}
                  axisFormat={fmtScalar}
                />
              </Panel>
              <Panel
                title={t("monitor.status_code")}
                empty={statusData.length === 0}
                action={
                  <NowValue>
                    {formatInteger(Math.round(totals.total))} {t("dashboard.col.requests")}
                  </NowValue>
                }
                {...panelState}
              >
                <MonitorChart
                  kind="stack"
                  data={statusData}
                  series={statusSeries}
                  rangeKey={range}
                  format={fmtCount}
                  axisFormat={formatTokenCount}
                />
              </Panel>
              <Panel
                title={t("monitor.latency_p95")}
                empty={p95Data.length === 0}
                action={<NowValue>{formatDurationMs(lastValue(state.p95Ms))}</NowValue>}
                {...panelState}
              >
                <MonitorChart
                  kind="line"
                  data={p95Data}
                  series={p95Series}
                  rangeKey={range}
                  format={formatDurationMs}
                  axisFormat={fmtLatencyAxis}
                  yAxisWidth={44}
                />
              </Panel>
            </div>

            {/* 资源面：逐 pod 对比 CPU / 内存 / 运行时，外加 SSE 长连接 */}
            <div className="stagger-rise grid min-h-0 gap-3 max-xl:auto-rows-[16rem] lg:grid-cols-2 xl:grid-cols-4">
              <Panel
                title={t("monitor.cpu_usage")}
                empty={cpuData.length === 0}
                action={<NowValue>{fmtPercent(cpuNow)}</NowValue>}
                {...panelState}
              >
                <MonitorChart
                  kind="line"
                  data={cpuData}
                  series={podSeries}
                  rangeKey={range}
                  format={fmtPercent}
                  axisFormat={fmtScalar}
                />
              </Panel>
              <Panel
                title={t("monitor.heap_memory")}
                empty={heapData.length === 0}
                action={<NowValue>{fmtMemory(heapNow)}</NowValue>}
                {...panelState}
              >
                <MonitorChart
                  kind="line"
                  data={heapData}
                  series={podSeries}
                  rangeKey={range}
                  format={fmtMemory}
                  axisFormat={fmtMemoryAxis}
                />
              </Panel>
              <Panel
                title={t("monitor.goroutines_chart")}
                empty={runtimeData.length === 0}
                action={
                  <ToggleGroup
                    variant="segmented"
                    size="sm"
                    spacing={0.5}
                    value={[runtimeMetric]}
                    onValueChange={(next) => {
                      const picked = RUNTIME_METRICS.find((m) => m === next[0]);
                      if (picked) setStoredRuntime(picked);
                    }}
                  >
                    {RUNTIME_METRICS.map((m) => (
                      <ToggleGroupItem key={m} value={m} className="h-6 px-2 text-[11px]">
                        {t(`monitor.${m}`)}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                }
                footer={
                  <NowValue>
                    {t(`monitor.${runtimeMetric}`)} {formatInteger(runtimeNow)}
                  </NowValue>
                }
                {...panelState}
              >
                <MonitorChart
                  key={runtimeMetric}
                  kind="line"
                  data={runtimeData}
                  series={podSeries}
                  rangeKey={range}
                  format={fmtCount}
                  axisFormat={formatTokenCount}
                />
              </Panel>
              <Panel
                title={t("monitor.sse_active")}
                empty={sseData.length === 0}
                action={<NowValue>{formatInteger(sseNow)}</NowValue>}
                {...panelState}
              >
                <MonitorChart
                  kind="stack"
                  data={sseData}
                  series={sseSeries}
                  rangeKey={range}
                  format={fmtCount}
                  axisFormat={formatTokenCount}
                />
              </Panel>
            </div>
          </div>
        </div>
      </TooltipProvider>
    </PermissionGuard>
  );
}
