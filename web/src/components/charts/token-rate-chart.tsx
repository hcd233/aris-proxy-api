"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import type { TokenRateItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { Line, LineChart, XAxis, YAxis, CartesianGrid, ReferenceLine } from "recharts";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange, formatChartTime } from "@/lib/time-range";

const CHART_COLORS = ["#D97757", "#5B8DB8", "#7C6BA5", "#4A9E7D", "#C76B8A", "#8B7355", "#6B8BA4", "#A0522D"];

export function TokenRateChart() {
  const t = useT();
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>("dashboard.chart.tokenRate.timeRange", "7d");
  const [customStart, setCustomStart] = usePersistentState("dashboard.chart.tokenRate.customStart", "");
  const [customEnd, setCustomEnd] = usePersistentState("dashboard.chart.tokenRate.customEnd", "");
  const requestIdRef = useRef(0);
  const [data, setData] = useState<TokenRateItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();

  const fetchData = useCallback(async (range?: TimeRangeKey, cs?: string, ce?: string) => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(range ?? timeRange, cs ?? customStart, ce ?? customEnd);
      const rsp = await api.fetchTokenRate({ startTime, endTime, granularity });
      if (requestId !== requestIdRef.current) return;
      setData(rsp.data ?? []);
    } catch {
      if (requestId !== requestIdRef.current) return;
      setError(true);
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [timeRange, customStart, customEnd]);

  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const models = [...new Set(data.map((d) => d.model))];
  const chartConfig = Object.fromEntries(
    models.map((m, i) => [m, { label: m, color: CHART_COLORS[i % CHART_COLORS.length] }])
  );

  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number | null>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      pointMap.get(p.time)![item.model] = p.outputTokensPerSecond === 0 ? null : p.outputTokensPerSecond;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  // Calculate average output token rate per model
  const modelAverages = models.map((model) => {
    const values = data
      .find((d) => d.model === model)
      ?.points.filter((p) => p.outputTokensPerSecond > 0)
      .map((p) => p.outputTokensPerSecond) ?? [];
    if (values.length === 0) return { model, average: 0 };
    const sum = values.reduce((a, b) => a + b, 0);
    return { model, average: sum / values.length };
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">{t("dashboard.token_rate")}</CardTitle>
        <TimeRangePicker
          value={timeRange}
          customStart={customStart}
          customEnd={customEnd}
          onChange={(key, cs, ce) => {
            setTimeRange(key);
            setCustomStart(cs);
            setCustomEnd(ce);
            fetchData(key, cs, ce);
          }}
        />
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : error ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
            <p>Failed to load</p>
            <Button variant="outline" size="sm" onClick={() => fetchData()}>
              Retry
            </Button>
          </div>
        ) : flatData.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            <LineChart data={flatData}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tickFormatter={(v) => formatChartTime(v, timeRange, customStart, customEnd)}
                fontSize={12}
              />
              <YAxis fontSize={12} domain={[0, "auto"]} allowDataOverflow={false} />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(value, name, item) => {
                      if (value == null) return null;
                      const indicatorColor = item?.color ?? "#888";
                      return (
                        <>
                          <div
                            className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                            style={{ backgroundColor: indicatorColor }}
                          />
                          <div className="flex flex-1 items-center justify-between leading-none">
                            <span className="text-muted-foreground">{name}</span>
                            <span className="font-mono font-medium text-foreground tabular-nums">
                              {`${Number(value).toFixed(2)} tok/s`}
                            </span>
                          </div>
                        </>
                      );
                    }}
                  />
                }
              />
              <ChartLegend content={<ChartLegendContent activeLegend={activeLegend} onLegendHover={onLegendHover} />} />
              {models.map((m) => (
                <Line
                  key={m}
                  type="monotone"
                  dataKey={m}
                  stroke={chartConfig[m]?.color ?? "#888"}
                  strokeWidth={2}
                  strokeOpacity={getStrokeOpacity(m)}
                  dot={false}
                />
              ))}
              {modelAverages.map(({ model, average }) => (
                activeLegend === model && average > 0 && (
                  <ReferenceLine
                    key={`avg-${model}`}
                    y={average}
                    stroke={chartConfig[model]?.color ?? "#888"}
                    strokeDasharray="8 4"
                    strokeWidth={1.5}
                    label={({ viewBox }: { viewBox: { x?: number; y?: number; width?: number } }) => {
                      const color = chartConfig[model]?.color ?? "#888";
                      const formatted =
                        average >= 1000
                          ? average.toLocaleString(undefined, { maximumFractionDigits: 0 })
                          : average.toFixed(2);
                      const text = `avg ${formatted} tok/s`;
                      const labelWidth = 120;
                      const right = (viewBox.x ?? 0) + (viewBox.width ?? 0);
                      return (
                        <foreignObject
                          x={right - labelWidth - 4}
                          y={(viewBox.y ?? 0) - 22}
                          width={labelWidth}
                          height={20}
                        >
                          <div
                            style={{
                              display: "flex",
                              justifyContent: "flex-end",
                            }}
                          >
                            <span
                              style={{
                                display: "inline-flex",
                                alignItems: "center",
                                padding: "2px 8px",
                                borderRadius: 999,
                                background: `${color}1A`,
                                color,
                                fontSize: 11,
                                fontWeight: 600,
                                lineHeight: "16px",
                                whiteSpace: "nowrap",
                                fontVariantNumeric: "tabular-nums",
                              }}
                            >
                              {text}
                            </span>
                          </div>
                        </foreignObject>
                      );
                    }}
                  />
                )
              ))}
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
