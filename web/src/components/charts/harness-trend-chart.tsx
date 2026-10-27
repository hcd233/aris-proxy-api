"use client";

import { useCallback } from "react";
import { api } from "@/lib/api-client";
import type { HarnessTrendItem } from "@/lib/types";
import { LineChartCard } from "@/components/charts/line-chart-card";

/** 未识别流量的固定展示名，始终排在最后并固定为中性灰 */
const OTHER_HARNESS = "Other";
const OTHER_COLOR = "#94a3b8";

export function HarnessTrendChart() {
  const toChart = useCallback((data: HarnessTrendItem[], colors: readonly string[]) => {
    const names = [...new Set(data.map((d) => d.harness))].sort((a, b) => {
      if (a === OTHER_HARNESS) return 1;
      if (b === OTHER_HARNESS) return -1;
      return a.localeCompare(b);
    });
    const series = names.map((name, i) => ({
      key: name,
      label: name,
      color: name === OTHER_HARNESS ? OTHER_COLOR : colors[i % colors.length],
    }));

    const timeSet = new Set<string>();
    const pointMap = new Map<string, Record<string, number>>();
    for (const item of data) {
      for (const p of item.points) {
        timeSet.add(p.time);
        if (!pointMap.has(p.time)) pointMap.set(p.time, {});
        pointMap.get(p.time)![item.harness] = p.count;
      }
    }
    const rows = Array.from(timeSet)
      .sort()
      .map((time) => ({ time, ...pointMap.get(time) }));
    return { rows, series };
  }, []);

  return (
    <LineChartCard<HarnessTrendItem>
      titleKey="dashboard.harness_trend"
      storageKey="dashboard.chart.harnessTrend"
      defaultRange="7d"
      chartType="area"
      stackId="harness"
      fetchData={(p) => api.fetchHarnessTrend(p)}
      toChart={toChart}
    />
  );
}
