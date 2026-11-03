"use client";

import { useCallback, useMemo } from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useI18n } from "@/lib/i18n";
import { useChartSeriesColors } from "@/lib/theme";
import { useChartMotion } from "@/lib/motion/chart-motion";
import { formatChartTime } from "@/lib/time-range";
import { formatInteger, formatTokenCount } from "@/lib/utils";
import {
  foldTopModels,
  formatCostShort,
  MODEL_METRICS,
  OTHER_KEY,
  rankModelsPerPoint,
  resolveModelMetric,
} from "@/lib/dashboard";
import { ChartContainer, ChartTooltip, type ChartConfig } from "@/components/ui/chart";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Panel } from "@/components/dashboard/panel";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

/** 堆叠图与图例（tooltip）每个时间点展示的模型数上限，其余归入「其他」 */
const TOP_K = 5;

/** 模型用量：全局 Top K 模型堆叠 + 「其他」；图例按每个时间点独立取 Top K；指标切换持久化 */
export function UsageTrendPanel({ range }: { range: DashboardRange }) {
  const { t, locale } = useI18n();
  const [storedMetric, setStoredMetric] = usePersistentState<string>(
    "dashboard.usageMetric",
    "cost",
  );
  const metric = resolveModelMetric(storedMetric);
  const colors = useChartSeriesColors();
  const motion = useChartMotion();
  const { data: overview, scope: overviewScope } = useOverview();
  const { startTime, endTime, granularity, refreshToken, scope } = range;

  const fetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardModelSeries({ startTime, endTime, granularity });
    return rsp.data ?? [];
  }, [startTime, endTime, granularity]);
  const {
    data: rows,
    loading,
    error,
    retry,
  } = useDashboardQuery(`${startTime}|${endTime}|${granularity}|${refreshToken}`, fetcher, scope);

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

  // 坐标轴回调保持引用稳定：内联函数每次渲染都会让 recharts 重算坐标并重放曲线动画
  const { key: rangeKey, customStart, customEnd } = range;
  const xTickFormatter = useCallback(
    (v: string) => formatChartTime(v, rangeKey, customStart, customEnd),
    [rangeKey, customStart, customEnd],
  );
  const timeline = overview?.current.series;
  const { keys, chartData, config } = useMemo(() => {
    const folded = foldTopModels(rows ?? [], metric, TOP_K);
    const byTime = new Map(folded.data.map((d) => [new Date(d.time).getTime(), d]));
    // 以 overview 已补齐空桶的时间轴为准，模型序列缺失的桶补 0
    const merged = (timeline ?? []).map((p) => {
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
  }, [rows, timeline, metric, colors, locale]);

  // 图例：每个时间点独立取 Top K，其余归「其他」并置尾，不参与排名（与堆叠图的全局 Top K 解耦）
  const rankedByTime = useMemo(() => rankModelsPerPoint(rows ?? [], metric, TOP_K), [rows, metric]);
  const renderTooltip = useCallback(
    ({ active, payload }: { active?: boolean; payload?: ReadonlyArray<{ payload?: unknown }> }) => {
      if (!active || !payload?.length) return null;
      const time = String((payload[0]?.payload as { time?: unknown } | undefined)?.time ?? "");
      const entries = rankedByTime.get(new Date(time).getTime()) ?? [];
      if (entries.length === 0) return null;
      const colorOf = (key: string) => config[key]?.color ?? config[OTHER_KEY]?.color;
      return (
        <div className="grid min-w-32 gap-1.5 rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl">
          <div className="font-medium">
            {formatChartTime(time, rangeKey, customStart, customEnd)}
          </div>
          <div className="grid gap-1.5">
            {entries.map((e) => (
              <div key={e.key} className="flex items-center justify-between gap-3">
                <span className="flex min-w-0 items-center gap-2">
                  <span
                    className="size-2.5 shrink-0 rounded-[2px]"
                    style={{ backgroundColor: colorOf(e.key) }}
                  />
                  <span className="break-all text-muted-foreground">
                    {e.key === OTHER_KEY ? t("dashboard.other") : e.key}
                  </span>
                </span>
                <span className="font-mono font-medium text-foreground tabular-nums">
                  {format(e.value)}
                </span>
              </div>
            ))}
          </div>
        </div>
      );
    },
    [rankedByTime, config, format, rangeKey, customStart, customEnd, t],
  );

  return (
    <Panel
      title={t("dashboard.panel.usage")}
      loading={loading}
      error={error}
      empty={!rows}
      onRetry={retry}
      action={
        <ToggleGroup
          variant="segmented"
          size="sm"
          spacing={0.5}
          value={[metric]}
          onValueChange={(next) => {
            const picked = MODEL_METRICS.find((m) => m === next[0]);
            if (picked) setStoredMetric(picked);
          }}
        >
          {MODEL_METRICS.map((m) => (
            <ToggleGroupItem key={m} value={m} className="h-6 px-2 text-[11px]">
              {t(`dashboard.metric.${m}`)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      }
    >
      {/* 时间轴（overview）换视图时整图重新入场；同视图刷新只做点位补间 */}
      <ChartContainer
        key={overviewScope}
        config={config}
        className="aspect-auto h-full min-h-40 w-full"
      >
        <AreaChart data={chartData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
          <CartesianGrid vertical={false} strokeDasharray="3 3" />
          <XAxis
            dataKey="time"
            tickLine={false}
            axisLine={false}
            minTickGap={32}
            tickFormatter={xTickFormatter}
          />
          <YAxis tickLine={false} axisLine={false} width={44} tickFormatter={format} />
          <ChartTooltip content={renderTooltip} />
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
