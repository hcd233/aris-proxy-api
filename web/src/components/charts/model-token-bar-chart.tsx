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
import { ChevronLeft, ChevronRight } from "lucide-react";

type SortField = "total" | "inputTokens" | "outputTokens" | "cacheReadTokens" | "cacheCreationTokens";

const PAGE_SIZE = 8;

// warm-toned palette (claude.ai inspired)
const CACHE_READ_COLOR = "#F2D0B8";
const INPUT_COLOR = "#E6733F";
const CACHE_CREATED_COLOR = "#F2D5BE";
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
  const [page, setPage] = useState(0);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const { startTime, endTime, granularity } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.fetchModelUsage({ startTime, endTime, granularity });
      setData(rsp.data ?? []);
      setPage(0);
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

  const totalPages = Math.ceil(sorted.length / PAGE_SIZE);
  const pageItems = sorted.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  function handleSort(field: SortField) {
    if (sortField === field) {
      setSortDir((d) => (d === "desc" ? "asc" : "desc"));
    } else {
      setSortField(field);
      setSortDir("desc");
    }
    setPage(0);
  }

  function sortIndicator(field: SortField) {
    if (sortField !== field) return "";
    return sortDir === "desc" ? " ▼" : " ▲";
  }

  function renderBarPair(
    cacheLabel: string,
    cacheValue: number,
    cacheColor: string,
    mainLabel: string,
    mainValue: number,
    mainColor: string,
  ) {
    const maxVal = Math.max(cacheValue, mainValue);
    return (
      <div className="space-y-1.5">
        <div className="flex items-center gap-2">
          <span className="w-20 shrink-0 text-[11px] text-muted-foreground">{cacheLabel}</span>
          <div className="h-2.5 flex-1 overflow-hidden rounded-sm bg-muted" title={`${cacheLabel}: ${formatTokenCount(cacheValue)}`}>
            {maxVal > 0 && (
              <div
                style={{ width: `${(cacheValue / maxVal) * 100}%`, backgroundColor: cacheColor }}
                className="h-full rounded-sm transition-all duration-200"
              />
            )}
          </div>
          <span className="w-12 shrink-0 text-right text-[11px] tabular-nums" style={{ color: cacheColor }}>{formatTokenCount(cacheValue)}</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="w-20 shrink-0 text-[11px] text-muted-foreground">{mainLabel}</span>
          <div className="h-2.5 flex-1 overflow-hidden rounded-sm bg-muted" title={`${mainLabel}: ${formatTokenCount(mainValue)}`}>
            {maxVal > 0 && (
              <div
                style={{ width: `${(mainValue / maxVal) * 100}%`, backgroundColor: mainColor }}
                className="h-full rounded-sm transition-all duration-200"
              />
            )}
          </div>
          <span className="w-12 shrink-0 text-right text-[11px] tabular-nums" style={{ color: mainColor }}>{formatTokenCount(mainValue)}</span>
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
          <div>
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
                    <th className="w-[240px] py-2 text-left font-medium">Input</th>
                    <th className="w-[240px] py-2 text-left font-medium">Output</th>
                  </tr>
                </thead>
              </table>
            </div>
            <div className="max-h-[400px] overflow-y-auto">
              <table className="w-full text-sm tabular-nums">
                <tbody>
                  {pageItems.map((item, i) => {
                    const total = tokenTotal(item);
                    const realIdx = page * PAGE_SIZE + i;
                    return (
                      <tr
                        key={item.model}
                        className="border-b border-border transition-colors hover:bg-muted/50"
                      >
                        <td className="w-8 py-3 pr-2 text-muted-foreground">{realIdx + 1}</td>
                        <td className="py-3 pr-4 font-medium">{item.model}</td>
                        <td className="py-3 pr-4 text-right font-semibold">
                          {formatTokenCount(total)}
                        </td>
                        <td className="w-[240px] py-3 pr-4">
                          {renderBarPair(
                            "Read",
                            item.cacheReadTokens,
                            CACHE_READ_COLOR,
                            "Input",
                            item.inputTokens,
                            INPUT_COLOR,
                          )}
                        </td>
                        <td className="w-[240px] py-3">
                          {renderBarPair(
                            "Created",
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
            {totalPages > 1 && (
              <div className="flex items-center justify-between border-t border-border pt-3">
                <span className="text-xs text-muted-foreground">
                  {page * PAGE_SIZE + 1}&ndash;{Math.min((page + 1) * PAGE_SIZE, sorted.length)} of {sorted.length} models
                </span>
                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    disabled={page === 0}
                    onClick={() => setPage((p) => p - 1)}
                  >
                    <ChevronLeft className="size-4" />
                  </Button>
                  <span className="px-2 text-xs text-muted-foreground">{page + 1} / {totalPages}</span>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    disabled={page >= totalPages - 1}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
