"use client";

import { useCallback, useMemo } from "react";
import { Area, AreaChart, CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";

import {
  type ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { useChartMotion } from "@/lib/motion/chart-motion";
import { formatMonitorTick } from "@/lib/monitor";

export interface MonitorChartSeries {
  key: string;
  label: string;
  color: string;
}

/**
 * line  多条折线（按 pod 对比）
 * area  单条面积（总量趋势）
 * stack 堆叠面积（构成：状态码类别、SSE provider）
 */
export type MonitorChartKind = "line" | "area" | "stack";

interface MonitorChartProps {
  kind: MonitorChartKind;
  data: Array<Record<string, number>>;
  series: MonitorChartSeries[];
  rangeKey: string;
  /** 浮层数值格式化；必须是模块级函数或 useCallback（引用变化会让 recharts 重放动画） */
  format: (value: number) => string;
  /** Y 轴刻度格式化，缺省沿用 format */
  axisFormat?: (value: number) => string;
  /** 每个 Y 轴刻度的最小宽度 */
  yAxisWidth?: number;
}

/** 模块级常量：内联数组每次渲染都是新引用，会让 recharts 重算坐标轴并重放曲线动画 */
const Y_DOMAIN: [number, "auto"] = [0, "auto"];
const CHART_MARGIN = { top: 4, right: 4, bottom: 0, left: 0 };

/** 运维图表：与 Dashboard 面板同一套图表语言（虚线网格、无轴线、紧凑坐标），铺满 Panel 主体 */
export function MonitorChart({
  kind,
  data,
  series,
  rangeKey,
  format,
  axisFormat,
  yAxisWidth = 40,
}: MonitorChartProps) {
  const config = useMemo<ChartConfig>(
    () => Object.fromEntries(series.map((s) => [s.key, { label: s.label, color: s.color }])),
    [series],
  );
  const showLegend = series.length > 1;
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();
  // 刷新按 time 配对补间：窗口右移时曲线平移、新点滑入；data 引用不变时不触发
  const motion = useChartMotion();
  const xTickFormatter = useCallback(
    (v: unknown) => formatMonitorTick(Number(v), rangeKey),
    [rangeKey],
  );
  const tooltipLabel = useCallback(
    (_: unknown, payload: ReadonlyArray<{ payload?: { time?: number } }>) => {
      const time = payload?.[0]?.payload?.time;
      return time ? formatMonitorTick(time, rangeKey) : "";
    },
    [rangeKey],
  );

  const axes = (
    <>
      <CartesianGrid vertical={false} strokeDasharray="3 3" />
      <XAxis
        dataKey="time"
        tickLine={false}
        axisLine={false}
        minTickGap={32}
        tickMargin={6}
        tickFormatter={xTickFormatter}
      />
      <YAxis
        tickLine={false}
        axisLine={false}
        width={yAxisWidth}
        domain={Y_DOMAIN}
        tickFormatter={axisFormat ?? format}
      />
      <ChartTooltip
        content={
          <ChartTooltipContent
            labelFormatter={tooltipLabel}
            formatter={(value, name, item) => {
              if (value == null) return null;
              return (
                <>
                  <div
                    className="size-2.5 shrink-0 rounded-[2px]"
                    style={{ backgroundColor: item?.color }}
                  />
                  <div className="flex flex-1 items-center justify-between gap-3 leading-none">
                    <span className="text-muted-foreground">
                      {config[name as string]?.label ?? name}
                    </span>
                    <span className="font-mono font-medium text-foreground tabular-nums">
                      {format(Number(value))}
                    </span>
                  </div>
                </>
              );
            }}
          />
        }
      />
      {showLegend && (
        <ChartLegend
          content={
            <ChartLegendContent
              className="pt-1 text-[11px]"
              activeLegend={activeLegend}
              onLegendHover={onLegendHover}
            />
          }
        />
      )}
    </>
  );

  const className = "aspect-auto h-full min-h-32 w-full";

  if (kind === "line") {
    return (
      <ChartContainer config={config} className={className}>
        <LineChart data={data} margin={CHART_MARGIN}>
          {axes}
          {series.map((s) => (
            <Line
              key={s.key}
              type="monotone"
              dataKey={s.key}
              stroke={s.color}
              strokeWidth={1.5}
              strokeOpacity={getStrokeOpacity(s.key)}
              dot={false}
              {...motion}
            />
          ))}
        </LineChart>
      </ChartContainer>
    );
  }

  const stacked = kind === "stack";
  return (
    <ChartContainer config={config} className={className}>
      <AreaChart data={data} margin={CHART_MARGIN}>
        {axes}
        {series.map((s) => (
          <Area
            key={s.key}
            type="monotone"
            dataKey={s.key}
            stackId={stacked ? "stack" : undefined}
            stroke={s.color}
            fill={s.color}
            fillOpacity={stacked ? 0.35 : 0.18}
            strokeWidth={stacked ? 1.25 : 1.5}
            strokeOpacity={getStrokeOpacity(s.key)}
            dot={false}
            {...motion}
          />
        ))}
      </AreaChart>
    </ChartContainer>
  );
}
