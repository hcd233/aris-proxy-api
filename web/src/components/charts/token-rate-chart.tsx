"use client";

import { useCallback } from "react";
import { api } from "@/lib/api-client";
import type { TokenRateItem } from "@/lib/types";
import { LineChartCard, formatTooltipRow } from "@/components/charts/line-chart-card";
import { ReferenceLine } from "recharts";
import { useI18n } from "@/lib/i18n";

/** 色板只有 8 色；超过 8 个模型时把尾部模型并成「其他」，避免颜色复用后不可区分。 */
const MAX_SERIES = 7;
const OTHER_KEY = "__other__";

export function TokenRateChart() {
  const { t, locale } = useI18n();
  const toChart = useCallback(
    (data: TokenRateItem[], colors: readonly string[]) => {
      const models = [...new Set(data.map((d) => d.modelId))];
      const totals = new Map<string, number>();
      for (const item of data) {
        for (const p of item.points) {
          totals.set(item.modelId, (totals.get(item.modelId) ?? 0) + p.outputTokensPerSecond);
        }
      }
      const ranked = [...models].sort((a, b) => (totals.get(b) ?? 0) - (totals.get(a) ?? 0));
      const fold = ranked.length > MAX_SERIES + 1;
      const visible = fold ? ranked.slice(0, MAX_SERIES) : ranked;
      const otherModels = fold ? new Set(ranked.slice(MAX_SERIES)) : null;

      const series = visible.map((m, i) => ({
        key: m,
        label: m,
        color: colors[i % colors.length],
      }));
      if (otherModels) {
        series.push({
          key: OTHER_KEY,
          label: t("charts.other"),
          color: "#8C8C8C",
        });
      }

      const timeSet = new Set<string>();
      const pointMap = new Map<string, Record<string, number | null>>();
      for (const item of data) {
        const key = otherModels?.has(item.modelId) ? OTHER_KEY : item.modelId;
        for (const p of item.points) {
          timeSet.add(p.time);
          if (!pointMap.has(p.time)) pointMap.set(p.time, {});
          const row = pointMap.get(p.time)!;
          if (key === OTHER_KEY) {
            row[key] = (row[key] ?? 0) + p.outputTokensPerSecond;
          } else {
            row[key] = p.outputTokensPerSecond === 0 ? null : p.outputTokensPerSecond;
          }
        }
      }
      const rows = Array.from(timeSet)
        .sort()
        .map((time) => {
          const row: Record<string, number | string | null> = { time, ...pointMap.get(time) };
          if (otherModels && row[OTHER_KEY] === 0) row[OTHER_KEY] = null;
          return row;
        });
      return { rows, series };
    },
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），「其他」标签刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, t],
  );

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
