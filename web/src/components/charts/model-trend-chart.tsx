"use client";

import { useCallback } from "react";
import { api } from "@/lib/api-client";
import type { ModelTrendItem } from "@/lib/types";
import { LineChartCard } from "@/components/charts/line-chart-card";

export function ModelTrendChart() {
  const toChart = useCallback((data: ModelTrendItem[], colors: readonly string[]) => {
    const models = [...new Set(data.map((d) => d.modelId))];
    const series = models.map((m, i) => ({
      key: m,
      label: m,
      color: colors[i % colors.length],
    }));
    const timeSet = new Set<string>();
    const pointMap = new Map<string, Record<string, number>>();
    for (const item of data) {
      for (const p of item.points) {
        timeSet.add(p.time);
        if (!pointMap.has(p.time)) pointMap.set(p.time, {});
        pointMap.get(p.time)![item.modelId] = p.count;
      }
    }
    const rows = Array.from(timeSet)
      .sort()
      .map((time) => ({ time, ...pointMap.get(time) }));
    return { rows, series };
  }, []);

  return (
    <LineChartCard<ModelTrendItem>
      titleKey="dashboard.model_trend"
      storageKey="dashboard.chart.modelTrend"
      defaultRange="7d"
      fetchData={(p) => api.fetchModelTrend(p)}
      toChart={toChart}
    />
  );
}
