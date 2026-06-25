"use client";

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

export interface RuntimeChartSeries {
  key: string;
  label: string;
  color: string;
}

interface RuntimeChartProps {
  title: string;
  data: Array<Record<string, number>>;
  series: RuntimeChartSeries[];
  unit?: string;
  rangeKey: string;
  emptyLabel: string;
}

function formatTick(unix: number, rangeKey: string): string {
  const d = new Date(unix * 1000);
  if (rangeKey === "15m") {
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
  }
  if (rangeKey === "24h") {
    return d.toLocaleString([], { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });
  }
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", hour12: false });
}

export function RuntimeChart({ title, data, series, unit, rangeKey, emptyLabel }: RuntimeChartProps) {
  const config: ChartConfig = Object.fromEntries(
    series.map((s) => [s.key, { label: s.label, color: s.color }]),
  );
  const isEmpty = data.length === 0;
  const showLegend = series.length > 1;

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
                tickFormatter={(v) => formatTick(Number(v), rangeKey)}
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
                domain={[0, "auto"]}
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
                              {Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 })}
                              {unit ?? ""}
                            </span>
                          </div>
                        </>
                      );
                    }}
                  />
                }
              />
              {showLegend && <ChartLegend content={<ChartLegendContent />} />}
              {series.map((s) => (
                <Line
                  key={s.key}
                  type="monotone"
                  dataKey={s.key}
                  stroke={s.color}
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
              ))}
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
