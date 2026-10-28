"use client";

import { useCallback, useMemo } from "react";
import { Bar, CartesianGrid, ComposedChart, Line, XAxis, YAxis } from "recharts";
import { api } from "@/lib/api-client";
import { useI18n } from "@/lib/i18n";
import { useStatusColors } from "@/lib/theme";
import { useChartMotion } from "@/lib/motion/chart-motion";
import { formatChartTime } from "@/lib/time-range";
import { formatInteger } from "@/lib/utils";
import { formatRate, successRate } from "@/lib/dashboard";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { TooltipContent, TooltipRoot, TooltipTrigger } from "@/components/ui/tooltip";
import { Panel } from "@/components/dashboard/panel";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

const FAILURE_KEYS = ["rateLimit", "server", "connection", "other"] as const;
const FAILURE_LABEL: Record<(typeof FAILURE_KEYS)[number], string> = {
  rateLimit: "dashboard.failure.rate_limit",
  server: "dashboard.failure.server",
  connection: "dashboard.failure.connection",
  other: "dashboard.failure.other",
};

/** 可靠性：失败请求按原因堆叠柱 + 成功率线（右轴），底部给出现最多的错误 */
export function ReliabilityPanel({ range }: { range: DashboardRange }) {
  const { t, locale } = useI18n();
  const status = useStatusColors();
  const motion = useChartMotion();
  const { data, loading, error, retry } = useOverview();
  const { startTime, endTime, refreshToken } = range;

  const errFetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardErrors({ startTime, endTime, limit: 1 });
    return rsp.data ?? [];
  }, [startTime, endTime]);
  const { data: topErrors } = useDashboardQuery(
    `${startTime}|${endTime}|${refreshToken}`,
    errFetcher,
  );

  const config = useMemo<ChartConfig>(
    () => ({
      rateLimit: { label: t(FAILURE_LABEL.rateLimit), color: status.warning },
      server: { label: t(FAILURE_LABEL.server), color: status.negative },
      connection: { label: t(FAILURE_LABEL.connection), color: status.muted },
      other: { label: t(FAILURE_LABEL.other), color: status.muted },
      rate: { label: t("dashboard.success_rate_line"), color: status.positive },
    }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [status, locale],
  );

  const series = data?.current.series;
  const chartData = useMemo(
    () =>
      (series ?? []).map((p) => ({
        time: p.time,
        ...p.failures,
        rate: successRate(p) ?? undefined,
      })),
    [series],
  );

  const summary = data?.current.summary;
  const top = topErrors?.[0];

  return (
    <Panel
      title={t("dashboard.panel.reliability")}
      loading={loading}
      error={error}
      empty={!data}
      onRetry={retry}
      action={
        summary && (
          <span className="font-mono text-[11px] text-muted-foreground tabular-nums">
            {formatInteger(summary.requests - summary.success)} / {formatInteger(summary.requests)}
          </span>
        )
      }
      footer={
        top ? (
          <div className="flex min-w-0 items-center gap-1.5">
            <span className="shrink-0">{t("dashboard.panel.top_error")}</span>
            <TooltipRoot>
              <TooltipTrigger
                render={
                  <code className="min-w-0 truncate rounded bg-muted px-1 font-mono text-[10px] text-foreground">
                    {top.statusCode} {top.message || "\u2014"}
                  </code>
                }
              />
              <TooltipContent className="max-w-xs break-all">
                {top.statusCode} {top.message}
              </TooltipContent>
            </TooltipRoot>
            <span className="shrink-0 font-mono tabular-nums">×{formatInteger(top.count)}</span>
          </div>
        ) : (
          <span>{t("dashboard.panel.no_errors")}</span>
        )
      }
    >
      <ChartContainer config={config} className="aspect-auto h-full min-h-32 w-full">
        <ComposedChart data={chartData} margin={{ top: 4, right: 0, bottom: 0, left: 0 }}>
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
            yAxisId="count"
            tickLine={false}
            axisLine={false}
            width={32}
            allowDecimals={false}
          />
          <YAxis
            yAxisId="rate"
            orientation="right"
            tickLine={false}
            axisLine={false}
            width={36}
            domain={[(min: number) => Math.max(0, Math.floor(min * 100) / 100 - 0.01), 1]}
            tickFormatter={(v: number) => formatRate(v, 0)}
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
                formatter={(value, name) => (
                  <div className="flex flex-1 items-center justify-between gap-3">
                    <span className="text-muted-foreground">
                      {config[String(name)]?.label ?? name}
                    </span>
                    <span className="font-mono text-foreground tabular-nums">
                      {name === "rate"
                        ? formatRate(Number(value), 2)
                        : formatInteger(Number(value))}
                    </span>
                  </div>
                )}
              />
            }
          />
          {FAILURE_KEYS.map((k, i) => (
            <Bar
              key={k}
              yAxisId="count"
              dataKey={k}
              stackId="failures"
              fill={config[k].color}
              fillOpacity={k === "other" ? 0.45 : 0.85}
              radius={i === FAILURE_KEYS.length - 1 ? [2, 2, 0, 0] : 0}
              {...motion}
            />
          ))}
          <Line
            yAxisId="rate"
            dataKey="rate"
            type="monotone"
            stroke={config.rate.color}
            strokeWidth={1.5}
            dot={false}
            connectNulls
            {...motion}
          />
        </ComposedChart>
      </ChartContainer>
    </Panel>
  );
}
