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
import { Area, AreaChart, XAxis, YAxis, CartesianGrid } from "recharts";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange, formatChartTime } from "@/lib/time-range";

const TOKEN_LAYERS = [
  { key: "cacheReadTokens", label: "Cache Read", color: "#F2D0B8" },
  { key: "inputTokens", label: "Input", color: "#E6733F" },
  { key: "cacheCreationTokens", label: "Cache Write", color: "#F2D5BE" },
  { key: "outputTokens", label: "Output", color: "#D46A3E" },
] as const;

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

export function TokenVolumeChart() {
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
    TOKEN_LAYERS.map((l) => [l.key, { label: l.label, color: l.color }])
  );

  const timeSet = new Set<string>();
  const pointMap = new Map<string, Record<string, number>>();
  for (const item of data) {
    for (const p of item.points) {
      timeSet.add(p.time);
      if (!pointMap.has(p.time)) pointMap.set(p.time, {});
      const entry = pointMap.get(p.time)!;
      const freshInput = Math.max(p.inputTokens - p.cacheReadTokens, 0);
      const freshOutput = Math.max(p.outputTokens - p.cacheCreationTokens, 0);
      entry.inputTokens = (entry.inputTokens ?? 0) + freshInput;
      entry.outputTokens = (entry.outputTokens ?? 0) + freshOutput;
      entry.cacheReadTokens = (entry.cacheReadTokens ?? 0) + p.cacheReadTokens;
      entry.cacheCreationTokens = (entry.cacheCreationTokens ?? 0) + p.cacheCreationTokens;
    }
  }
  const flatData = Array.from(timeSet).sort().map((time) => ({
    time,
    ...pointMap.get(time),
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Token Throughput</CardTitle>
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
        ) : flatData.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            <AreaChart data={flatData}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="time"
                tickFormatter={(v) => formatChartTime(v, timeRange, customStart, customEnd)}
                fontSize={12}
              />
              <YAxis fontSize={12} tickFormatter={formatTokenCount} domain={[0, "auto"]} allowDataOverflow={false} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <ChartLegend content={<ChartLegendContent activeLegend={activeLegend} onLegendHover={onLegendHover} />} />
              {TOKEN_LAYERS.map((layer) => (
                <Area
                  key={layer.key}
                  type="monotone"
                  dataKey={layer.key}
                  stackId="1"
                  stroke={layer.color}
                  fill={layer.color}
                  strokeOpacity={getStrokeOpacity(layer.key)}
                  fillOpacity={0.6}
                />
              ))}
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
