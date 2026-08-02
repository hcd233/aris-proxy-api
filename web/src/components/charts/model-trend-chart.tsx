"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import type { ModelTrendItem, Granularity } from "@/lib/types";
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
import { Line, LineChart, XAxis, YAxis, CartesianGrid } from "recharts";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange, formatChartTime, generateEmptyTimeline } from "@/lib/time-range";
import { useChartSeriesColors } from "@/lib/theme";

export function ModelTrendChart() {
  const t = useT();
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>("dashboard.chart.modelTrend.timeRange", "7d");
  const [customStart, setCustomStart] = usePersistentState("dashboard.chart.modelTrend.customStart", "");
  const [customEnd, setCustomEnd] = usePersistentState("dashboard.chart.modelTrend.customEnd", "");
  const requestIdRef = useRef(0);
  const [data, setData] = useState<ModelTrendItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [rangeState, setRangeState] = useState<{
    startTime: string;
    endTime: string;
    granularity: Granularity;
  } | null>(null);
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();

  const fetchData = useCallback(async (range?: TimeRangeKey, cs?: string, ce?: string) => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(range ?? timeRange, cs ?? customStart, ce ?? customEnd);
      setRangeState({ startTime, endTime, granularity });
      const rsp = await api.fetchModelTrend({
        startTime,
        endTime,
        granularity,
      });
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

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const seriesColors = useChartSeriesColors();
  const models = [...new Set(data.map((d) => d.modelId))];
  const chartConfig = Object.fromEntries(
    models.map((m, i) => [
      m,
      { label: m, color: seriesColors[i % seriesColors.length] },
    ])
  );

  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      pointMap.get(p.time)![item.modelId] = p.count;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  // 后端无数据时仍渲染空坐标轴（X 轴时间刻度 + Y 轴网格），而非显示空态文案
  const isEmpty = flatData.length === 0;
  const chartData =
    isEmpty && rangeState
      ? generateEmptyTimeline(
          rangeState.startTime,
          rangeState.endTime,
          rangeState.granularity,
        ).map((time) => ({ time, __empty: 0 }))
      : flatData;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">{t("dashboard.model_trend")}</CardTitle>
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
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tickFormatter={(v) => formatChartTime(v, timeRange, customStart, customEnd)}
                fontSize={12}
              />
              <YAxis fontSize={12} domain={isEmpty ? [0, 1] : [0, "auto"]} tickCount={isEmpty ? 3 : undefined} allowDataOverflow={false} />
              <ChartTooltip content={<ChartTooltipContent />} />
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
              {isEmpty && <Line dataKey="__empty" stroke="transparent" dot={false} />}
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
