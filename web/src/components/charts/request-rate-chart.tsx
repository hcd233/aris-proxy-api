"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { RequestRateItem, Granularity } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
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

const granularityOptions: { value: Granularity; label: string }[] = [
  { value: "hour", label: "Hour" },
  { value: "day", label: "Day" },
  { value: "week", label: "Week" },
];

function toISODate(d: Date): string {
  return d.toISOString().replace(/\.\d+Z$/, "Z");
}

export function RequestRateChart() {
  const [granularity, setGranularity] = useState<Granularity>("hour");
  const [data, setData] = useState<RequestRateItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const end = new Date();
      const start = new Date();
      start.setHours(start.getHours() - 24);
      const rsp = await api.fetchRequestRate({
        startTime: toISODate(start),
        endTime: toISODate(end),
        granularity,
      });
      setData(rsp.data ?? []);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [granularity]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const models = [...new Set(data.map((d) => d.model))];
  const CHART_COLORS = ["#D97757", "#B8654A", "#E8A87C", "#8C7E72", "#F5E6E0"];
  const chartConfig = Object.fromEntries(
    models.map((m, i) => [
      m,
      { label: m, color: CHART_COLORS[i % CHART_COLORS.length] },
    ])
  );

  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      pointMap.get(p.time)![item.model] = p.successRate * 100;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Request Success Rate</CardTitle>
        <ToggleGroup
          value={[granularity]}
          onValueChange={(v) => v.length > 0 && setGranularity(v[0] as Granularity)}
          size="sm"
        >
          {granularityOptions.map((opt) => (
            <ToggleGroupItem key={opt.value} value={opt.value}>
              {opt.label}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
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
                tickFormatter={(v) => new Date(v).toLocaleDateString()}
                fontSize={12}
              />
              <YAxis
                fontSize={12}
                domain={[0, 100]}
                allowDataOverflow={false}
                tickFormatter={(v) => `${v}%`}
              />
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
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
