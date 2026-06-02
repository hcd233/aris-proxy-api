"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type { TokenUsageItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

type SortField = "total" | "inputTokens" | "outputTokens" | "cacheReadTokens" | "cacheCreationTokens";

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

function tokenTotal(item: TokenUsageItem): number {
  return item.inputTokens + item.outputTokens + item.cacheReadTokens + item.cacheCreationTokens;
}

export function ModelTokenBarChart() {
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [data, setData] = useState<TokenUsageItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [sortField, setSortField] = useState<SortField>("total");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.fetchTokenUsage({ startTime, endTime, granularity });
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

  const sorted = [...data].sort((a, b) => {
    let va: number, vb: number;
    if (sortField === "total") {
      va = tokenTotal(a);
      vb = tokenTotal(b);
    } else {
      va = a[sortField];
      vb = b[sortField];
    }
    return sortDir === "desc" ? vb - va : va - vb;
  });

  function handleSort(field: SortField) {
    if (sortField === field) {
      setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    } else {
      setSortField(field);
      setSortDir("desc");
    }
  }

  function sortIndicator(field: SortField) {
    if (sortField !== field) return "";
    return sortDir === "desc" ? " ▼" : " ▲";
  }

  function renderBar(
    leftLabel: string,
    leftValue: number,
    leftColor: string,
    rightLabel: string,
    rightValue: number,
    rightColor: string,
    total: number,
  ) {
    const leftPct = total > 0 ? (leftValue / total) * 100 : 0;
    const rightPct = total > 0 ? (rightValue / total) * 100 : 0;
    return (
      <div>
        <div
          className="flex h-3 overflow-hidden rounded-md bg-muted"
          title={`${leftLabel}: ${formatTokenCount(leftValue)} / ${rightLabel}: ${formatTokenCount(rightValue)}`}
        >
          {leftPct > 0 && (
            <div
              style={{ width: `${leftPct}%`, backgroundColor: leftColor }}
              className="transition-all duration-200"
            />
          )}
          {rightPct > 0 && (
            <div
              style={{ width: `${rightPct}%`, backgroundColor: rightColor }}
              className="transition-all duration-200"
            />
          )}
        </div>
        <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
          <span style={{ color: leftColor }}>{leftLabel} {formatTokenCount(leftValue)}</span>
          <span style={{ color: rightColor }}>{rightLabel} {formatTokenCount(rightValue)}</span>
        </div>
      </div>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">Model Usage</CardTitle>
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
        ) : sorted.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm tabular-nums">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="w-8 py-2 text-left font-medium">#</th>
                  <th className="py-2 text-left font-medium">Model</th>
                  <th
                    className="cursor-pointer py-2 text-right font-medium hover:text-foreground"
                    onClick={() => handleSort("total")}
                  >
                    Total{sortIndicator("total")}
                  </th>
                  <th className="w-[220px] py-2 text-left font-medium">Input</th>
                  <th className="w-[220px] py-2 text-left font-medium">Output</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((item, i) => {
                  const total = tokenTotal(item);
                  const inputTotal = item.inputTokens + item.cacheReadTokens;
                  const outputTotal = item.outputTokens + item.cacheCreationTokens;
                  return (
                    <tr
                      key={item.model}
                      className="border-b border-border transition-colors hover:bg-muted/50"
                    >
                      <td className="py-3 pr-2 text-muted-foreground">{i + 1}</td>
                      <td className="py-3 pr-4 font-medium">{item.model}</td>
                      <td className="py-3 pr-4 text-right font-semibold">
                        {formatTokenCount(total)}
                      </td>
                      <td className="py-3 pr-4">
                        {renderBar(
                          "Cache Read",
                          item.cacheReadTokens,
                          "#7C6BA5",
                          "Input",
                          item.inputTokens,
                          "#D97757",
                          inputTotal,
                        )}
                      </td>
                      <td className="py-3">
                        {renderBar(
                          "Cache Created",
                          item.cacheCreationTokens,
                          "#4A9E7D",
                          "Output",
                          item.outputTokens,
                          "#5B8DB8",
                          outputTotal,
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
