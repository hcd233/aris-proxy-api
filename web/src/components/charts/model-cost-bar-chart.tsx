"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useI18n } from "@/lib/i18n";
import { formatCost } from "@/lib/money";
import type { ModelCostItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { useRatioSegmentColors } from "@/lib/theme";
import { RatioLegend, StackedRatioBar } from "@/components/charts/stacked-ratio-bar";

/**
 * ModelCostBarChart 模型成本：时间范围内各模型总成本 + 输入/输出/缓存读/缓存写四维成本占比条。
 * 行 = 模型 × 币种；模型名下方条形按同币种最大总成本归一化；总成本列可排序。
 */
export function ModelCostBarChart() {
  // t 引用已稳定化（见 lib/i18n.tsx），useMemo 改依赖 locale 以响应语言切换
  const { t, locale } = useI18n();
  const tokenColors = useRatioSegmentColors();
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

  // 进度条按币种分别归一化：不同币种金额不可比，混算会让小币种的条形失真
  const maxTotalByCurrency = useMemo(() => {
    const m = new Map<string, number>();
    for (const d of data) m.set(d.currency, Math.max(m.get(d.currency) ?? 0, d.totalCost));
    return m;
  }, [data]);

  const sorted = useMemo(() => {
    return [...data].sort((a, b) =>
      sortDir === "desc" ? b.totalCost - a.totalCost : a.totalCost - b.totalCost,
    );
  }, [data, sortDir]);

  const legend = useMemo(
    () => [
      { key: "input", label: t("cost.dim.input"), color: tokenColors.input },
      { key: "output", label: t("cost.dim.output"), color: tokenColors.output },
      { key: "cacheRead", label: t("cost.dim.cacheRead"), color: tokenColors.cacheRead },
      { key: "cacheWrite", label: t("cost.dim.cacheWrite"), color: tokenColors.cacheCreated },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, t, tokenColors],
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
                  <th
                    className="cursor-pointer py-2 pr-4 text-right font-medium hover:text-foreground"
                    onClick={() => setSortDir((d) => (d === "desc" ? "asc" : "desc"))}
                  >
                    {t("cost.dim.total")}
                    {sortDir === "desc" ? " ▼" : " ▲"}
                  </th>
                  <th className="w-[360px] py-2 pr-6 text-left text-xs font-medium">
                    <RatioLegend segments={legend} />
                  </th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((item, i) => {
                  const maxTotal = maxTotalByCurrency.get(item.currency) ?? 0;
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
                      <td className="w-[360px] py-3 pr-6">
                        <StackedRatioBar
                          formatValue={(v) => formatCost(v, item.currency)}
                          segments={[
                            { ...legend[0], value: item.inputCost },
                            { ...legend[1], value: item.outputCost },
                            { ...legend[2], value: item.cacheReadCost },
                            { ...legend[3], value: item.cacheCreationCost },
                          ]}
                        />
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
