"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import type { TokenThroughputPoint } from "@/lib/types";
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
  const [data, setData] = useState<TokenThroughputPoint[]>([]);
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

  const rawMap = useMemo(() => {
    const m = new Map<string, { input: number; output: number }>();
    for (const p of data) {
      m.set(p.time, { input: p.inputTokens, output: p.outputTokens });
    }
    return m;
  }, [data]);

  const flatData = data.map((p) => {
    const freshInput = Math.max(p.inputTokens - p.cacheReadTokens, 0);
    const freshOutput = Math.max(p.outputTokens - p.cacheCreationTokens, 0);
    return {
      time: p.time,
      inputTokens: freshInput,
      outputTokens: freshOutput,
      cacheReadTokens: p.cacheReadTokens,
      cacheCreationTokens: p.cacheCreationTokens,
    };
  });

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
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(value, name, item) => {
                      if (value == null) return null;
                      const indicatorColor = item?.color ?? "#888";
                      let displayValue = Number(value);
                      if (item?.payload?.time) {
                        const raw = rawMap.get(item.payload.time as string);
                        if (raw) {
                          if (name === "inputTokens") displayValue = raw.input;
                          if (name === "outputTokens") displayValue = raw.output;
                        }
                      }
                      const label = name ? (chartConfig[name]?.label ?? name) : "";
                      return (
                        <>
                          <div
                            className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                            style={{ backgroundColor: indicatorColor }}
                          />
                          <div className="flex flex-1 items-center justify-between leading-none">
                            <span className="text-muted-foreground">{label}</span>
                            <span className="font-mono font-medium text-foreground tabular-nums">
                              {formatTokenCount(displayValue)}
                            </span>
                          </div>
                        </>
                      );
                    }}
                  />
                }
              />
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
