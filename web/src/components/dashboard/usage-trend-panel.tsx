"use client";

import { useCallback, useMemo, useState } from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { api } from "@/lib/api-client";
import { useI18n } from "@/lib/i18n";
import { useChartSeriesColors } from "@/lib/theme";
import { useChartMotion } from "@/lib/motion/chart-motion";
import { formatChartTime } from "@/lib/time-range";
import { formatInteger, formatTokenCount } from "@/lib/utils";
import { foldTopModels, formatCostShort, OTHER_KEY, type ModelMetric } from "@/lib/dashboard";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { SegmentedChoice } from "@/components/dashboard/segmented-choice";
import { Panel } from "@/components/dashboard/panel";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

const METRICS: ModelMetric[] = ["cost", "requests", "tokens"];

/** 模型用量：Top 5 模型堆叠 + 「其他」；切换指标不重新请求 */
export function UsageTrendPanel({ range }: { range: DashboardRange }) {
  const { t, locale } = useI18n();
  const colors = useChartSeriesColors();
  const motion = useChartMotion();
  const [metric, setMetric] = useState<ModelMetric>("cost");
  const { data: overview } = useOverview();
  const { startTime, endTime, granularity, refreshToken } = range;

  const fetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardModelSeries({ startTime, endTime, granularity });
    return rsp.data ?? [];
  }, [startTime, endTime, granularity]);
  const {
    data: rows,
    loading,
    error,
    retry,
  } = useDashboardQuery(`${startTime}|${endTime}|${granularity}|${refreshToken}`, fetcher);

  const currency = overview?.currency;
  const format = useCallback(
    (v: number) =>
      metric === "cost"
        ? formatCostShort(v, currency)
        : metric === "tokens"
          ? formatTokenCount(v)
          : formatInteger(v),
    [metric, currency],
  );

  const { keys, chartData, config } = useMemo(() => {
    const folded = foldTopModels(rows ?? [], metric, 5);
    const byTime = new Map(folded.data.map((d) => [new Date(d.time).getTime(), d]));
    // 以 overview 已补齐空桶的时间轴为准，模型序列缺失的桶补 0
    const timeline = overview?.current.series ?? [];
    const merged = timeline.map((p) => {
      const row: Record<string, number | string> = { time: p.time };
      const hit = byTime.get(new Date(p.time).getTime());
      for (const k of folded.keys) row[k] = hit?.[k] ?? 0;
      return row;
    });
    const cfg: ChartConfig = Object.fromEntries(
      folded.keys.map((k, i) => [
        k,
        { label: k === OTHER_KEY ? t("dashboard.other") : k, color: colors[i % colors.length] },
      ]),
    );
    return { keys: folded.keys, chartData: merged, config: cfg };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, overview, metric, colors, locale]);

  return (
    <Panel
      title={t("dashboard.panel.usage")}
      loading={loading}
      error={error}
      empty={!rows}
      onRetry={retry}
      action={
        <SegmentedChoice
          value={metric}
          options={METRICS.map((m) => ({ value: m, label: t(`dashboard.metric.${m}`) }))}
          onChange={setMetric}
          aria-label={t("dashboard.panel.usage")}
        />
      }
    >
      <ChartContainer config={config} className="aspect-auto h-full min-h-40 w-full">
        <AreaChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
          <CartesianGrid vertical={false} strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            tickLine={false}
            axisLine={false}
            minTickGap={32}
            tickFormatter={(v: string) =>
              formatChartTime(v, range.key, range.customStart, range.customEnd)
            }
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={44}
            tickFormatter={(v: number) => format(v)}
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
                      {format(Number(value))}
                    </span>
                  </div>
                )}
              />
            }
          />
          {keys.map((k) => (
            <Area
              key={k}
              dataKey={k}
              type="monotone"
              stackId="usage"
              stroke={config[k].color}
              fill={config[k].color}
              fillOpacity={0.35}
              strokeWidth={1.25}
              {...motion}
            />
          ))}
        </AreaChart>
      </ChartContainer>
    </Panel>
  );
}
