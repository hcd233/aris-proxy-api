"use client";

import { useMemo } from "react";
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
export function LatencyPanel({ range }: { range: DashboardRange }) {
  const { t, locale } = useI18n();
  const colors = useChartSeriesColors();
  const status = useStatusColors();
  const motion = useChartMotion();
  const { data, loading, error, retry } = useOverview();

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
      <ChartContainer config={config} className="aspect-auto h-full min-h-32 w-full">
        <ComposedChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
          <CartesianGrid vertical={false} strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            tickLine={false}
            axisLine={false}
            minTickGap={28}
            tickFormatter={(v: string) =>
              formatChartTime(v, range.key, range.customStart, range.customEnd)
            }
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={40}
            tickFormatter={(v: number) => formatDurationMs(v)}
          />
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
