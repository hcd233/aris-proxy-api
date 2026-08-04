"use client";

import { useCallback, useMemo } from "react";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import type { TokenThroughputPoint } from "@/lib/types";
import { LineChartCard } from "@/components/charts/line-chart-card";
import { useTokenLayerColors } from "@/lib/theme";

function formatTokenCount(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

export function TokenVolumeChart() {
  const t = useT();
  const tokenColors = useTokenLayerColors();
  const layers = useMemo(
    () =>
      [
        { key: "cacheReadTokens", label: t("charts.cache_read"), color: tokenColors.cacheRead },
        { key: "inputTokens", label: t("charts.input"), color: tokenColors.input },
        { key: "cacheCreationTokens", label: t("charts.cache_write"), color: tokenColors.cacheCreated },
        { key: "outputTokens", label: t("charts.output"), color: tokenColors.output },
      ] as const,
    [t, tokenColors],
  );

  const toChart = useCallback(
    (data: TokenThroughputPoint[]) => {
      const rows = data.map((p) => {
        const freshInput = Math.max(p.inputTokens - p.cacheReadTokens, 0);
        const freshOutput = Math.max(p.outputTokens - p.cacheCreationTokens, 0);
        return {
          time: p.time,
          inputTokens: freshInput,
          outputTokens: freshOutput,
          cacheReadTokens: p.cacheReadTokens,
          cacheCreationTokens: p.cacheCreationTokens,
          // tooltip 展示原始 token 值（与图上 fresh 值区分）
          rawInputTokens: p.inputTokens,
          rawOutputTokens: p.outputTokens,
        };
      });
      const series = layers.map((l) => ({ key: l.key, label: l.label, color: l.color }));
      return { rows, series };
    },
    [layers],
  );

  return (
    <LineChartCard<TokenThroughputPoint>
      titleKey="dashboard.token_volume"
      storageKey="dashboard.chart.tokenVolume"
      defaultRange="7d"
      fetchData={(p) => api.fetchTokenThroughput(p)}
      toChart={toChart}
      chartType="area"
      stackId="1"
      yAxis={{ tickFormatter: formatTokenCount }}
      tooltipFormatter={(value, name, item) => {
        if (value == null) return null;
        const indicatorColor = item?.color ?? "#888";
        let displayValue = Number(value);
        const payload = item?.payload;
        if (payload) {
          if (name === "inputTokens") displayValue = Number(payload.rawInputTokens ?? displayValue);
          if (name === "outputTokens") displayValue = Number(payload.rawOutputTokens ?? displayValue);
        }
        const label = layers.find((l) => l.key === name)?.label ?? name;
        return (
          <>
            <div
              className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
              style={{ backgroundColor: indicatorColor }}
            />
            <div className="flex flex-1 items-center justify-between leading-none">
              <span className="text-muted-foreground">{label}</span>
              <span className="font-mono font-medium text-foreground tabular-nums">
                {formatTokenCount(displayValue)}
              </span>
            </div>
          </>
        );
      }}
    />
  );
}
