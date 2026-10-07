"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { formatCost } from "@/lib/money";
import type { ModelCostItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

type SortField = "totalCost" | "inputCost" | "outputCost" | "cacheReadCost" | "cacheCreationCost";

/**
 * ModelCostBarChart 模型成本排行：时间范围内各模型成本（输入/输出/缓存读/缓存写四维 + 总成本）。
 * 行 = 模型 × 币种；条形宽度按总成本占比；列头可排序。
 */
export function ModelCostBarChart() {
  const t = useT();
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>(
    "dashboard.chart.modelCostBar.timeRange",
    "7d",
  );
  const [customStart, setCustomStart] = usePersistentState(
    "dashboard.chart.modelCostBar.customStart",
    "",
  );
  const [customEnd, setCustomEnd] = usePersistentState(
    "dashboard.chart.modelCostBar.customEnd",
    "",
  );
  const requestIdRef = useRef(0);
  const [data, setData] = useState<ModelCostItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [sortField, setSortField] = useState<SortField>("totalCost");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const fetchData = useCallback(
    async (range?: TimeRangeKey, cs?: string, ce?: string) => {
      const requestId = ++requestIdRef.current;
      setLoading(true);
      setError(false);
      try {
        const { startTime, endTime } = computeRange(
          range ?? timeRange,
          cs ?? customStart,
          ce ?? customEnd,
        );
        const rsp = await api.fetchModelCost({ startTime, endTime });
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
    },
    [timeRange, customStart, customEnd],
  );

  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const maxTotal = useMemo(() => data.reduce((m, d) => Math.max(m, d.totalCost), 0), [data]);

  const sorted = useMemo(() => {
    return [...data].sort((a, b) =>
      sortDir === "desc" ? b[sortField] - a[sortField] : a[sortField] - b[sortField],
    );
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

  const headCell = (field: SortField, label: string, right = true) => (
    <th
      className={`cursor-pointer py-2 pr-4 font-medium hover:text-foreground ${right ? "text-right" : "text-left"}`}
      onClick={() => handleSort(field)}
    >
      {label}
      {sortIndicator(field)}
    </th>
  );

  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
        <CardTitle className="font-display">{t("dashboard.model_cost")}</CardTitle>
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
      <CardContent className="p-0">
        {loading ? (
          <div className="px-6 pb-6">
            <Skeleton className="h-64 w-full" />
          </div>
        ) : error ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 px-6 pb-6 text-sm text-muted-foreground">
            <p>{t("charts.failed_to_load")}</p>
            <Button variant="outline" size="sm" onClick={() => fetchData()}>
              {t("charts.retry")}
            </Button>
          </div>
        ) : sorted.length === 0 ? (
          <div className="flex h-64 items-center justify-center px-6 pb-6 text-sm text-muted-foreground">
            {t("audit.cost.unpriced")}
          </div>
        ) : (
          <div className="h-64 overflow-y-auto">
            <table className="w-full text-sm tabular-nums">
              <thead>
                <tr className="whitespace-nowrap border-b border-border text-muted-foreground">
                  <th className="w-8 py-2 pl-6 text-left font-medium">{t("model_chart.rank")}</th>
                  <th className="py-2 text-left font-medium">{t("model_chart.model")}</th>
                  {headCell("totalCost", t("cost.dim.total"))}
                  {headCell("inputCost", t("cost.dim.input"))}
                  {headCell("outputCost", t("cost.dim.output"))}
                  {headCell("cacheReadCost", t("cost.dim.cacheRead"))}
                  <th className="py-2 pr-6 text-right font-medium">{t("cost.dim.cacheWrite")}</th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((item, i) => {
                  const widthPct =
                    maxTotal > 0 ? Math.max((item.totalCost / maxTotal) * 100, 2) : 2;
                  return (
                    <tr
                      key={`${item.modelId}|${item.currency}`}
                      className="border-b border-border transition-colors hover:bg-muted/50"
                    >
                      <td className="py-3 pl-6 pr-2 text-muted-foreground">{i + 1}</td>
                      <td className="py-3 pr-4">
                        <div className="font-medium">{item.modelId}</div>
                        <div className="mt-1 h-1.5 overflow-hidden rounded-md bg-muted">
                          <div
                            className="h-full rounded-md bg-primary/60 transition-all duration-200"
                            style={{ width: `${widthPct}%` }}
                          />
                        </div>
                      </td>
                      <td className="py-3 pr-4 text-right font-semibold">
                        {formatCost(item.totalCost, item.currency)}
                      </td>
                      <td className="py-3 pr-4 text-right">
                        {formatCost(item.inputCost, item.currency)}
                      </td>
                      <td className="py-3 pr-4 text-right">
                        {formatCost(item.outputCost, item.currency)}
                      </td>
                      <td className="py-3 pr-4 text-right">
                        {formatCost(item.cacheReadCost, item.currency)}
                      </td>
                      <td className="py-3 pr-6 text-right">
                        {formatCost(item.cacheCreationCost, item.currency)}
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
