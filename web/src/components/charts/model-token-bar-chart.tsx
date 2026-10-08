"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useI18n } from "@/lib/i18n";
import type { ModelUsageItem } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { useTokenLayerColors } from "@/lib/theme";
import { RatioLegend, StackedRatioBar } from "@/components/charts/stacked-ratio-bar";

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

function formatExactTokenCount(v: number): string {
  return v.toLocaleString();
}

/** tokenTotal 总计 = 输入 + 输出 + 缓存读 + 缓存写。 */
function tokenTotal(item: ModelUsageItem): number {
  return item.inputTokens + item.outputTokens + item.cacheReadTokens + item.cacheCreationTokens;
}

export function ModelTokenBarChart() {
  // t 引用已稳定化（见 lib/i18n.tsx），useMemo 改依赖 locale 以响应语言切换
  const { t, locale } = useI18n();
  const tokenColors = useTokenLayerColors();
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>(
    "dashboard.chart.modelTokenBar.timeRange",
    "7d",
  );
  const [customStart, setCustomStart] = usePersistentState(
    "dashboard.chart.modelTokenBar.customStart",
    "",
  );
  const [customEnd, setCustomEnd] = usePersistentState(
    "dashboard.chart.modelTokenBar.customEnd",
    "",
  );
  const requestIdRef = useRef(0);
  const [data, setData] = useState<ModelUsageItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  const fetchData = useCallback(
    async (range?: TimeRangeKey, cs?: string, ce?: string) => {
      const requestId = ++requestIdRef.current;
      setLoading(true);
      setError(false);
      try {
        const { startTime, endTime, granularity } = computeRange(
          range ?? timeRange,
          cs ?? customStart,
          ce ?? customEnd,
        );
        const rsp = await api.fetchModelUsage({ startTime, endTime, granularity });
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

  const sorted = useMemo(() => {
    return [...data].sort((a, b) =>
      sortDir === "desc" ? tokenTotal(b) - tokenTotal(a) : tokenTotal(a) - tokenTotal(b),
    );
  }, [data, sortDir]);

  const legend = useMemo(
    () => [
      { key: "input", label: t("charts.input"), color: tokenColors.input },
      { key: "output", label: t("charts.output"), color: tokenColors.output },
      { key: "cacheRead", label: t("charts.cache_read"), color: tokenColors.cacheRead },
      { key: "cacheWrite", label: t("charts.cache_write"), color: tokenColors.cacheCreated },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, t, tokenColors],
  );

  return (
    <Card>
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
        <CardTitle className="font-display">{t("dashboard.model_usage")}</CardTitle>
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
                    {t("charts.total")}
                    {sortDir === "desc" ? " ▼" : " ▲"}
                  </th>
                  <th className="w-[360px] py-2 pr-6 text-left text-xs font-medium">
                    <RatioLegend segments={legend} />
                  </th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((item, i) => (
                  <tr
                    key={item.modelId}
                    className="border-b border-border transition-colors hover:bg-muted/50"
                  >
                    <td className="py-3 pl-6 pr-2 text-muted-foreground">{i + 1}</td>
                    <td className="py-3 pr-4 font-medium">{item.modelId}</td>
                    <td className="py-3 pr-4 text-right font-semibold">
                      {formatTokenCount(tokenTotal(item))}
                    </td>
                    <td className="w-[360px] py-3 pr-6">
                      <StackedRatioBar
                        formatValue={formatExactTokenCount}
                        segments={[
                          { ...legend[0], value: item.inputTokens },
                          { ...legend[1], value: item.outputTokens },
                          { ...legend[2], value: item.cacheReadTokens },
                          { ...legend[3], value: item.cacheCreationTokens },
                        ]}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
