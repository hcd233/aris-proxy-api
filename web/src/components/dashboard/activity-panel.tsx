"use client";

import { useCallback, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import { useI18n } from "@/lib/i18n";
import { useChartSeriesColors } from "@/lib/theme";
import { cn, formatInteger } from "@/lib/utils";
import { formatRate, heatmapMatrix } from "@/lib/dashboard";
import { Panel } from "@/components/dashboard/panel";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

const HOURS = Array.from({ length: 24 }, (_, h) => h);

/** 浏览器本地 IANA 时区（热力图按本地时间分桶） */
function localTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

/** 活跃时段热力图（星期 × 小时，本地时区）+ 客户端分布条（Top 3 + 其他） */
export function ActivityPanel({ range }: { range: DashboardRange }) {
  const { t, locale } = useI18n();
  const colors = useChartSeriesColors();
  const { startTime, endTime, refreshToken } = range;

  const heatFetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardHeatmap({ startTime, endTime, tz: localTimeZone() });
    return rsp.data ?? [];
  }, [startTime, endTime]);
  const heat = useDashboardQuery(`${startTime}|${endTime}|${refreshToken}`, heatFetcher);

  const clientFetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardTop({ startTime, endTime, dimension: "harness", limit: 3 });
    if (!rsp.data) throw new Error("empty top");
    return rsp.data;
  }, [startTime, endTime]);
  const clients = useDashboardQuery(`${startTime}|${endTime}|${refreshToken}`, clientFetcher);

  const [hover, setHover] = useState<{ d: number; h: number; count: number } | null>(null);
  const heatCells = heat.data;
  const matrix = useMemo(() => heatmapMatrix(heatCells ?? []), [heatCells]);
  const max = Math.max(1, ...matrix.flat());
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const weekdays = useMemo(() => t("dashboard.weekday.short").split(","), [locale]);

  const clientData = clients.data;
  const segments = useMemo(() => {
    if (!clientData) return [];
    const total = clientData.items.reduce((s, i) => s + i.requests, 0) + clientData.otherRequests;
    if (total <= 0) return [];
    const list = clientData.items.map((i, idx) => ({
      key: i.key,
      label: i.name || i.key,
      share: i.requests / total,
      color: colors[idx % colors.length],
    }));
    if (clientData.otherRequests > 0) {
      list.push({
        key: "__other__",
        label: t("dashboard.other"),
        share: clientData.otherRequests / total,
        color: colors[colors.length - 1],
      });
    }
    return list;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clientData, colors, locale]);

  return (
    <Panel
      title={t("dashboard.panel.activity")}
      loading={heat.loading}
      error={heat.error}
      empty={!heatCells}
      onRetry={heat.retry}
      bodyClassName="flex flex-col gap-2"
      action={
        hover && (
          <span className="font-mono text-[11px] text-muted-foreground tabular-nums">
            {weekdays[hover.d]} {String(hover.h).padStart(2, "0")}:00 · {formatInteger(hover.count)}
          </span>
        )
      }
    >
      <div
        onMouseLeave={() => setHover(null)}
        className="grid min-h-0 flex-1 grid-cols-[14px_repeat(24,minmax(0,1fr))] grid-rows-[repeat(7,minmax(0,1fr))_auto] gap-[2px]"
      >
        {matrix.map((row, d) => (
          <div key={d} className="contents">
            <span className="flex items-center text-[9px] leading-none text-muted-foreground">
              {weekdays[d]}
            </span>
            {row.map((count, h) => (
              <span
                key={h}
                onMouseEnter={() => setHover({ d, h, count })}
                className={cn("min-h-1.5 rounded-[2px]", count > 0 ? "bg-primary" : "bg-muted")}
                style={count > 0 ? { opacity: 0.15 + (count / max) * 0.85 } : undefined}
              />
            ))}
          </div>
        ))}
        <span />
        {HOURS.map((h) => (
          <span key={h} className="text-center text-[8px] leading-none text-muted-foreground">
            {h % 6 === 0 ? h : ""}
          </span>
        ))}
      </div>

      <div className="space-y-1">
        <div className="text-[10px] tracking-wide text-muted-foreground uppercase">
          {t("dashboard.panel.clients")}
        </div>
        <div className="flex h-1.5 overflow-hidden rounded-full bg-muted">
          {segments.map((s) => (
            <div
              key={s.key}
              className="h-full"
              style={{ width: `${s.share * 100}%`, backgroundColor: s.color }}
            />
          ))}
        </div>
        <div className="flex flex-wrap gap-x-2.5 gap-y-0.5 text-[10px] text-muted-foreground">
          {segments.map((s) => (
            <span key={s.key} className="inline-flex items-center gap-1">
              <span className="size-1.5 rounded-[2px]" style={{ backgroundColor: s.color }} />
              {s.label} {formatRate(s.share, 0)}
            </span>
          ))}
        </div>
      </div>
    </Panel>
  );
}
