"use client";

import { useCallback } from "react";
import { api } from "@/lib/api-client";
import type { RequestRateItem } from "@/lib/types";
import { LineChartCard, formatTooltipRow } from "@/components/charts/line-chart-card";

export function RequestRateChart() {
  const toChart = useCallback((data: RequestRateItem[], colors: readonly string[]) => {
    const models = [...new Set(data.map((d) => d.modelId))];
    const series = models.map((m, i) => ({
      key: m,
      label: m,
      color: colors[i % colors.length],
    }));
    const timeSet = new Set<string>();
    const pointMap = new Map<string, Record<string, number | null>>();
    for (const item of data) {
      for (const p of item.points) {
        timeSet.add(p.time);
        if (!pointMap.has(p.time)) pointMap.set(p.time, {});
        pointMap.get(p.time)![item.modelId] = p.total === 0 ? null : p.successRate * 100;
      }
    }
    const rows = Array.from(timeSet)
      .sort()
      .map((time) => ({ time, ...pointMap.get(time) }));
    return { rows, series };
  }, []);

  return (
    <LineChartCard<RequestRateItem>
      titleKey="dashboard.request_rate"
      storageKey="dashboard.chart.requestRate"
      defaultRange="24h"
      fetchData={(p) => api.fetchRequestRate(p)}
      toChart={toChart}
      yAxis={{
        domain: [0, 100],
        emptyDomain: [0, 100],
        tickFormatter: (v) => `${v}%`,
      }}
      tooltipFormatter={(value, name, item) =>
        formatTooltipRow(value, name, item, (v) => `${v.toFixed(1)}%`)
      }
    />
  );
}
