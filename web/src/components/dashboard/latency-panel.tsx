"use client";

import { useCallback, useMemo } from "react";
import { Area, CartesianGrid, ComposedChart, Line, XAxis, YAxis } from "recharts";
import { useI18n } from "@/lib/i18n";
import { useChartSeriesColors, useStatusColors } from "@/lib/theme";
import { useChartMotion } from "@/lib/motion/chart-motion";
import { formatChartTime } from "@/lib/time-range";
import { formatDecimal, formatDurationMs } from "@/lib/utils";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { Panel } from "@/components/dashboard/panel";
import { useOverview } from "@/components/dashboard/overview-context";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

/** 延迟：首字 P50 / P95 / P99 三线 + P50–P95 带；底部输出速度与平均流式时长 */
/**
 * 轴刻度格式化：整根轴共用一个量纲与一个精度。
 *
 * 逐点调用 `formatDurationMs` 会让同一根轴同时出现 `950ms`、`2.85s`、`3.8s`
 * 三种刻度——单位跳变、小数位也随数值抖动，读起来像三套标尺叠在一起。
 * 这里按轴域最大值定单位：≥1s 整轴走秒（<10s 保留 1 位小数，≥10s 取整），
 * 否则整轴走毫秒。单个数值的浮层仍用 `formatDurationMs` 精确展示。
 */
function formatAxisDuration(v: number, axisMax: number): string {
  if (axisMax >= 1000) return `${formatDecimal(v / 1000, axisMax >= 10_000 ? 0 : 1)}s`;
  return `${formatDecimal(v, 0)}ms`;
}

export function LatencyPanel({ range }: { range: DashboardRange }) {
  const { t, locale } = useI18n();
  const colors = useChartSeriesColors();
  const status = useStatusColors();
  const motion = useChartMotion();
  const { data, scope, loading, error, retry } = useOverview();

  const config = useMemo<ChartConfig>(
    () => ({
      p50: { label: "P50", color: status.positive },
      p95: { label: "P95", color: colors[0] },
      p99: { label: "P99", color: colors[2] },
      band: { label: "P50–P95", color: colors[0] },
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [colors, status, locale],
  );

  // 坐标轴回调保持引用稳定：内联函数每次渲染都会让 recharts 重算坐标并重放曲线动画
  const { key: rangeKey, customStart, customEnd } = range;
  const xTickFormatter = useCallback(
    (v: string) => formatChartTime(v, rangeKey, customStart, customEnd),
    [rangeKey, customStart, customEnd],
  );
  const series = data?.current.series;
  const chartData = useMemo(
    () =>
      (series ?? []).map((p) => ({
        time: p.time,
        p50: p.latency.p50,
        p95: p.latency.p95,
        p99: p.latency.p99,
        band:
          p.latency.p50 != null && p.latency.p95 != null
            ? [p.latency.p50, p.latency.p95]
            : undefined,
      })),
    [series],
  );

  const summary = data?.current.summary;

  // 轴域上限决定整根轴的单位与精度（见 formatAxisDuration）
  const axisMax = useMemo(() => {
    let max = 0;
    for (const p of chartData) {
      for (const v of [p.p50, p.p95, p.p99]) {
        if (v != null && Number.isFinite(v) && v > max) max = v;
      }
    }
    return max;
  }, [chartData]);
  // 引用必须稳定：内联 tickFormatter 会让 recharts 重算坐标并重放曲线动画
  const yTickFormatter = useCallback((v: number) => formatAxisDuration(v, axisMax), [axisMax]);

  return (
    <Panel
      title={t("dashboard.panel.latency")}
      loading={loading}
      error={error}
      empty={!data}
      onRetry={retry}
      footer={
        summary && (
          <span className="tabular-nums">
            {t("dashboard.kpi.speed")}{" "}
            {summary.outputTokensPerSecond != null
              ? formatDecimal(summary.outputTokensPerSecond, 1)
              : "\u2014"}{" "}
            tok/s · {t("dashboard.panel.avg_stream")}{" "}
            {formatDurationMs(summary.avgStreamDurationMs)}
          </span>
        )
      }
    >
      {/* 换视图时整图重新入场；同视图刷新只做点位补间 */}
      <ChartContainer key={scope} config={config} className="aspect-auto h-full min-h-32 w-full">
        <ComposedChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
          <CartesianGrid vertical={false} strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            tickLine={false}
            axisLine={false}
            minTickGap={28}
            tickFormatter={xTickFormatter}
          />
          {/* 轴宽不写死：按实际刻度测量，`160ms` 这类 5 字符刻度在 40px 轴上会被 SVG 左缘裁掉 */}
          <YAxis tickLine={false} axisLine={false} width="auto" tickFormatter={yTickFormatter} />
          <ChartTooltip
            content={
              <ChartTooltipContent
                labelFormatter={(_, payload) =>
                  formatChartTime(
                    String(payload?.[0]?.payload?.time ?? ""),
                    range.key,
                    range.customStart,
                    range.customEnd,
                  )
                }
                formatter={(value, name) =>
                  name === "band" ? null : (
                    <div className="flex flex-1 items-center justify-between gap-3">
                      <span className="text-muted-foreground">
                        {config[String(name)]?.label ?? name}
                      </span>
                      <span className="font-mono text-foreground tabular-nums">
                        {formatDurationMs(Number(value))}
                      </span>
                    </div>
                  )
                }
              />
            }
          />
          <Area
            dataKey="band"
            type="monotone"
            stroke="none"
            fill={config.band.color}
            fillOpacity={0.12}
            {...motion}
          />
          <Line
            dataKey="p99"
            type="monotone"
            stroke={config.p99.color}
            strokeOpacity={0.55}
            strokeWidth={1}
            dot={false}
            {...motion}
          />
          <Line
            dataKey="p95"
            type="monotone"
            stroke={config.p95.color}
            strokeWidth={1.5}
            strokeDasharray="4 2"
            dot={false}
            {...motion}
          />
          <Line
            dataKey="p50"
            type="monotone"
            stroke={config.p50.color}
            strokeWidth={1.75}
            dot={false}
            {...motion}
          />
        </ComposedChart>
      </ChartContainer>
    </Panel>
  );
}
