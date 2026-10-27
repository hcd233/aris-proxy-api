"use client";

import { useCallback, useMemo } from "react";
import { api } from "@/lib/api-client";
import { useI18n } from "@/lib/i18n";
import type { TokenThroughputPoint } from "@/lib/types";
import { LineChartCard } from "@/components/charts/line-chart-card";
import { useTokenLayerColors } from "@/lib/theme";
import { toTokenVolumeRows, colorLuminance } from "@/lib/token-volume";
import { formatTokenCount } from "@/lib/utils";

export function TokenVolumeChart() {
  // t 引用已稳定化（见 lib/i18n.tsx），useMemo 改依赖 locale 以响应语言切换
  const { t, locale } = useI18n();
  const tokenColors = useTokenLayerColors();
  const layers = useMemo(
    () =>
      [
        { key: "cacheReadTokens", label: t("charts.cache_read"), color: tokenColors.cacheRead },
        { key: "inputTokens", label: t("charts.input"), color: tokenColors.input },
        {
          key: "cacheCreationTokens",
          label: t("charts.cache_write"),
          color: tokenColors.cacheCreated,
        },
        { key: "outputTokens", label: t("charts.output"), color: tokenColors.output },
      ] as const,
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, t, tokenColors],
  );

  // 深色在下、浅色在上：按亮度升序决定堆叠与图例顺序
  const stackLayers = useMemo(
    () => [...layers].sort((a, b) => colorLuminance(a.color) - colorLuminance(b.color)),
    [layers],
  );

  const toChart = useCallback(
    (data: TokenThroughputPoint[]) => {
      const rows = toTokenVolumeRows(data);
      const series = stackLayers.map((l) => ({ key: l.key, label: l.label, color: l.color }));
      return { rows, series };
    },
    [stackLayers],
  );

  return (
    <LineChartCard<TokenThroughputPoint>
      titleKey="dashboard.token_volume"
      storageKey="dashboard.chart.tokenVolume"
      defaultRange="7d"
      fetchData={(p) => api.fetchTokenThroughput(p)}
      toChart={toChart}
      chartType="bar"
      stackId="1"
      yAxis={{ tickFormatter: formatTokenCount }}
      tooltipFormatter={(value, name, item) => {
        if (value == null) return null;
        const indicatorColor = item?.color ?? "#888";
        const label = stackLayers.find((l) => l.key === name)?.label ?? name;
        return (
          <>
            <div
              className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
              style={{ backgroundColor: indicatorColor }}
            />
            <div className="flex flex-1 items-center justify-between leading-none">
              <span className="text-muted-foreground">{label}</span>
              <span className="font-mono font-medium text-foreground tabular-nums">
                {formatTokenCount(Number(value))}
              </span>
            </div>
          </>
        );
      }}
    />
  );
}
