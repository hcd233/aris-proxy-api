"use client";

import { useCallback } from "react";
import { Line, LineChart, XAxis, YAxis, CartesianGrid } from "recharts";

import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { useChartMotion } from "@/lib/motion/chart-motion";
import { formatDecimal } from "@/lib/utils";

interface RuntimeChartSeries {
  key: string;
  label: string;
  color: string;
  dashed?: boolean;
}

interface RuntimeChartProps {
  title: string;
  data: Array<Record<string, number>>;
  series: RuntimeChartSeries[];
  unit?: string;
  rangeKey: string;
  emptyLabel: string;
}

/** 模块级常量：内联数组每次渲染都是新引用，会让 recharts 重算坐标轴并重放曲线动画 */
const Y_DOMAIN: [number, "auto"] = [0, "auto"];

function formatTick(unix: number, rangeKey: string): string {
  const d = new Date(unix * 1000);
  if (rangeKey === "15m") {
    return d.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    });
  }
  if (rangeKey === "24h") {
    return d.toLocaleString([], {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  }
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
}

export function RuntimeChart({
  title,
  data,
  series,
  unit,
  rangeKey,
  emptyLabel,
}: RuntimeChartProps) {
  const config: ChartConfig = Object.fromEntries(
    series.map((s) => [s.key, { label: s.label, color: s.color }]),
  );
  const isEmpty = data.length === 0;
  const showLegend = series.length > 1;
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();
  // 刷新按 time 配对补间：窗口右移时曲线平移、新点滑入；data 引用不变时不触发
  const motion = useChartMotion();
  const xTickFormatter = useCallback((v: unknown) => formatTick(Number(v), rangeKey), [rangeKey]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="font-display">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        {isEmpty ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            {emptyLabel}
          </div>
        ) : (
          <ChartContainer config={config} className="h-64 w-full">
            <LineChart data={data} margin={{ left: 8, right: 12, top: 8, bottom: 4 }}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tickFormatter={xTickFormatter}
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={32}
                fontSize={12}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                width={44}
                domain={Y_DOMAIN}
                fontSize={12}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    labelFormatter={(_, payload) => {
                      const t = payload?.[0]?.payload?.time as number | undefined;
                      return t ? formatTick(t, rangeKey) : "";
                    }}
                    formatter={(value, name, item) => {
                      if (value == null) return null;
                      const indicatorColor = item?.color ?? "#888";
                      const seriesLabel = config[name as string]?.label ?? name;
                      return (
                        <>
                          <div
                            className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                            style={{ backgroundColor: indicatorColor }}
                          />
                          <div className="flex flex-1 items-center justify-between leading-none gap-3">
                            <span className="text-muted-foreground">{seriesLabel as string}</span>
                            <span className="font-mono font-medium text-foreground tabular-nums">
                              {formatDecimal(Number(value), 2)}
                              {unit ?? ""}
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
                    <ChartLegendContent activeLegend={activeLegend} onLegendHover={onLegendHover} />
                  }
                />
              )}
              {series.map((s) => (
                <Line
                  key={s.key}
                  type="monotone"
                  dataKey={s.key}
                  stroke={s.color}
                  strokeWidth={2}
                  strokeOpacity={getStrokeOpacity(s.key)}
                  strokeDasharray={s.dashed ? "6 4" : undefined}
                  dot={false}
                  {...motion}
                />
              ))}
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
