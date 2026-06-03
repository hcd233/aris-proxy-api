"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import type { ModelUsageItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

type SortField = "total" | "inputTokens" | "outputTokens" | "cacheReadTokens" | "cacheCreationTokens";

const CACHE_READ_COLOR = "#E8B896";
const INPUT_COLOR = "#E6733F";
const CACHE_CREATED_COLOR = "#E8C0A0";
const OUTPUT_COLOR = "#D46A3E";

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

function tokenTotal(item: ModelUsageItem): number {
  return item.inputTokens + item.outputTokens + item.cacheReadTokens + item.cacheCreationTokens;
}

export function ModelTokenBarChart() {
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("7d");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [data, setData] = useState<ModelUsageItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [sortField, setSortField] = useState<SortField>("total");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.fetchModelUsage({ startTime, endTime, granularity });
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

  const sorted = useMemo(() => {
    const arr = [...data].sort((a, b) => {
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
    return arr;
  }, [data, sortField, sortDir]);

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

  function renderStackedBar(
    cacheLabel: string,
    cacheValue: number,
    cacheColor: string,
    mainLabel: string,
    mainValue: number,
    mainColor: string,
  ) {
    const total = cacheValue + mainValue;
    const denom = total > 0 ? total : 1;
    const cachePct = Math.min((cacheValue / denom) * 100, 100);
    const mainPct = Math.min((mainValue / denom) * 100, 100);
    return (
      <div>
        <div
          className="flex h-3 overflow-hidden rounded-md bg-muted"
          title={`${cacheLabel}: ${formatTokenCount(cacheValue)} (${cachePct.toFixed(1)}%) / ${mainLabel}: ${formatTokenCount(mainValue)} (${mainPct.toFixed(1)}%)`}
        >
          {cachePct > 0 && (
            <div
              style={{ width: `${cachePct}%`, backgroundColor: cacheColor }}
              className="transition-all duration-200"
            />
          )}
          {mainPct > 0 && (
            <div
              style={{ width: `${mainPct}%`, backgroundColor: mainColor }}
              className="transition-all duration-200"
            />
          )}
        </div>
        <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
          <span style={{ color: cacheColor }}>{cacheLabel}: {formatTokenCount(cacheValue)} ({cachePct.toFixed(1)}%)</span>
          <span style={{ color: mainColor }}>{mainLabel}: {formatTokenCount(mainValue)} ({mainPct.toFixed(1)}%)</span>
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
      <CardContent className="p-0">
        {loading ? (
          <div className="px-6 pb-6">
            <Skeleton className="h-64 w-full" />
          </div>
        ) : error ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 px-6 pb-6 text-sm text-muted-foreground">
            <p>Failed to load</p>
            <Button variant="outline" size="sm" onClick={fetchData}>
              Retry
            </Button>
          </div>
        ) : sorted.length === 0 ? (
          <div className="flex h-64 items-center justify-center px-6 pb-6 text-sm text-muted-foreground">
            No data for this period
          </div>
        ) : (
          <div className="h-64 overflow-y-auto">
            <table className="w-full text-sm tabular-nums">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="w-8 py-2 pl-6 text-left font-medium">#</th>
                  <th className="py-2 text-left font-medium">Model</th>
                  <th
                    className="cursor-pointer py-2 text-right font-medium hover:text-foreground"
                    onClick={() => handleSort("total")}
                  >
                    Total{sortIndicator("total")}
                  </th>
                  <th className="w-[220px] py-2 text-left font-medium">Input</th>
                  <th className="w-[220px] py-2 pr-6 text-left font-medium">Output</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((item, i) => {
                  const total = tokenTotal(item);
                  return (
                    <tr
                      key={item.model}
                      className="border-b border-border transition-colors hover:bg-muted/50"
                    >
                      <td className="py-3 pl-6 pr-2 text-muted-foreground">{i + 1}</td>
                      <td className="py-3 pr-4 font-medium">{item.model}</td>
                      <td className="py-3 pr-4 text-right font-semibold">
                        {formatTokenCount(total)}
                      </td>
                      <td className="w-[220px] py-3 pr-4">
                        {renderStackedBar(
                          "Cache Read",
                          item.cacheReadTokens,
                          CACHE_READ_COLOR,
                          "Input",
                          item.inputTokens,
                          INPUT_COLOR,
                        )}
                      </td>
                      <td className="w-[220px] py-3 pr-6">
                        {renderStackedBar(
                          "Cache Write",
                          item.cacheCreationTokens,
                          CACHE_CREATED_COLOR,
                          "Output",
                          item.outputTokens,
                          OUTPUT_COLOR,
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
