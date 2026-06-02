"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { TokenThroughputItem } from "@/lib/types";
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
import { Bar, BarChart, XAxis, YAxis, CartesianGrid } from "recharts";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

const METRIC_LAYERS = [
  { key: "inputTokens", label: "Input", color: "#D97757" },
  { key: "outputTokens", label: "Output", color: "#5B8DB8" },
  { key: "cacheReadTokens", label: "Cache Read", color: "#7C6BA5" },
  { key: "cacheCreationTokens", label: "Cache Creation", color: "#4A9E7D" },
] as const;

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

export function ModelTokenBarChart() {
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [data, setData] = useState<TokenThroughputItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.fetchTokenThroughput({ startTime, endTime, granularity });
      setData(rsp.data ?? []);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [timeRange, customStart, customEnd]);

  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const chartConfig = Object.fromEntries(
    METRIC_LAYERS.map((l) => [l.key, { label: l.label, color: l.color }])
  );

  const modelData = data.map((item) => {
    const totals: Record<string, number | string> = { model: item.model };
    for (const metric of METRIC_LAYERS) {
      totals[metric.key] = item.points.reduce((sum, p) => sum + p[metric.key], 0);
    }
    return totals;
  });

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Model Token Usage</CardTitle>
        <TimeRangePicker
          value={timeRange}
          customStart={customStart}
          customEnd={customEnd}
          onChange={(key, cs, ce) => {
            setTimeRange(key);
            setCustomStart(cs);
            setCustomEnd(ce);
          }}
        />
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : error ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
            <p>Failed to load</p>
            <Button variant="outline" size="sm" onClick={fetchData}>
              Retry
            </Button>
          </div>
        ) : modelData.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            <BarChart data={modelData} barCategoryGap="20%" barGap={4}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="model"
                fontSize={12}
                tickLine={false}
              />
              <YAxis
                fontSize={12}
                tickFormatter={formatTokenCount}
                domain={[0, "auto"]}
                allowDataOverflow={false}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(value) => (
                      <span className="font-mono font-medium text-foreground tabular-nums">
                        {formatTokenCount(Number(value))}
                      </span>
                    )}
                  />
                }
              />
              <ChartLegend content={<ChartLegendContent activeLegend={activeLegend} onLegendHover={onLegendHover} />} />
              {METRIC_LAYERS.map((layer) => (
                <Bar
                  key={layer.key}
                  dataKey={layer.key}
                  fill={layer.color}
                  fillOpacity={getStrokeOpacity(layer.key)}
                  radius={[4, 4, 0, 0]}
                />
              ))}
            </BarChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
