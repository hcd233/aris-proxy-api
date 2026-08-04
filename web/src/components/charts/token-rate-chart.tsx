"use client";

import { useCallback } from "react";
import { api } from "@/lib/api-client";
import type { TokenRateItem } from "@/lib/types";
import { LineChartCard, formatTooltipRow } from "@/components/charts/line-chart-card";
import { ReferenceLine } from "recharts";

export function TokenRateChart() {
  const toChart = useCallback((data: TokenRateItem[], colors: readonly string[]) => {
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
        pointMap.get(p.time)![item.modelId] =
          p.outputTokensPerSecond === 0 ? null : p.outputTokensPerSecond;
      }
    }
    const rows = Array.from(timeSet)
      .sort()
      .map((time) => ({ time, ...pointMap.get(time) }));
    return { rows, series };
  }, []);

  return (
    <LineChartCard<TokenRateItem>
      titleKey="dashboard.token_rate"
      storageKey="dashboard.chart.tokenRate"
      defaultRange="7d"
      fetchData={(p) => api.fetchTokenRate(p)}
      toChart={toChart}
      tooltipFormatter={(value, name, item) =>
        formatTooltipRow(value, name, item, (v) => `${v.toFixed(2)} tok/s`)
      }
      extraElements={({ data, series, chartConfig, activeLegend }) => {
        // Calculate average output token rate per model
        const modelAverages = series.map((s) => {
          const values =
            data
              .find((d) => d.modelId === s.key)
              ?.points.filter((p) => p.outputTokensPerSecond > 0)
              .map((p) => p.outputTokensPerSecond) ?? [];
          if (values.length === 0) return { model: s.key, average: 0 };
          const sum = values.reduce((a, b) => a + b, 0);
          return { model: s.key, average: sum / values.length };
        });
        return modelAverages.map(({ model, average }) =>
          activeLegend === model && average > 0 ? (
            <ReferenceLine
              key={`avg-${model}`}
              y={average}
              stroke={chartConfig[model]?.color ?? "#888"}
              strokeDasharray="8 4"
              strokeWidth={1.5}
              label={({ viewBox }: { viewBox: { x?: number; y?: number; width?: number } }) => {
                const color = chartConfig[model]?.color ?? "#888";
                const formatted =
                  average >= 1000
                    ? average.toLocaleString(undefined, { maximumFractionDigits: 0 })
                    : average.toFixed(2);
                const text = `avg ${formatted} tok/s`;
                const labelWidth = 120;
                const right = (viewBox.x ?? 0) + (viewBox.width ?? 0);
                return (
                  <foreignObject
                    x={right - labelWidth - 4}
                    y={(viewBox.y ?? 0) - 22}
                    width={labelWidth}
                    height={20}
                  >
                    <div
                      style={{
                        display: "flex",
                        justifyContent: "flex-end",
                      }}
                    >
                      <span
                        style={{
                          display: "inline-flex",
                          alignItems: "center",
                          padding: "2px 8px",
                          borderRadius: 999,
                          background: `${color}1A`,
                          color,
                          fontSize: 11,
                          fontWeight: 600,
                          lineHeight: "16px",
                          whiteSpace: "nowrap",
                          fontVariantNumeric: "tabular-nums",
                        }}
                      >
                        {text}
                      </span>
                    </div>
                  </foreignObject>
                );
              }}
            />
          ) : null,
        );
      }}
    />
  );
}
