"use client";

import { format } from "date-fns";
import { RotateCw } from "lucide-react";
import { useT } from "@/lib/i18n";
import { PermissionGuard } from "@/components/permission-guard";
import { Button } from "@/components/ui/button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ActivityPanel } from "@/components/dashboard/activity-panel";
import { EndpointHealthPanel } from "@/components/dashboard/endpoint-health-panel";
import { KpiRow } from "@/components/dashboard/kpi-row";
import { LatencyPanel } from "@/components/dashboard/latency-panel";
import { OverviewProvider } from "@/components/dashboard/overview-context";
import { ReliabilityPanel } from "@/components/dashboard/reliability-panel";
import { TopRankingPanel } from "@/components/dashboard/top-ranking-panel";
import { UsageTrendPanel } from "@/components/dashboard/usage-trend-panel";
import { useDashboardRange } from "@/components/dashboard/use-dashboard-range";

/**
 * 一屏指挥台（spec docs/superpowers/specs/2026-10-28-dashboard-redesign-design.md §3）：
 * 页头 → 6 KPI → 用量/可靠性/延迟（5:3:3）→ Endpoint/Top/活跃时段（4:4:3）。
 * xl 及以上锁定视口高度（layout 内边距 lg:p-10 = 5rem），面板内部自适应撑满；更窄时纵向堆叠滚动。
 */
export default function DashboardPage() {
  const t = useT();
  const range = useDashboardRange();
  const rangeFormat = range.spanDays <= 1 ? "MM-dd HH:mm" : "MM-dd";

  return (
    <PermissionGuard module="dashboard">
      <TooltipProvider>
        <OverviewProvider range={range}>
          <div
            data-page-wide
            className="flex flex-col gap-3 xl:h-[calc(100dvh-5rem)] xl:min-h-[680px]"
          >
            <header className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
              <div className="min-w-0">
                <h1 className="motion-title font-display text-2xl tracking-tight text-foreground">
                  {t("dashboard.title")}
                </h1>
                <p className="mt-0.5 text-xs text-muted-foreground tabular-nums">
                  {format(new Date(range.startTime), rangeFormat)} →{" "}
                  {format(new Date(range.endTime), rangeFormat)} · {t("dashboard.compare_previous")}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <TimeRangePicker
                  value={range.key}
                  customStart={range.customStart}
                  customEnd={range.customEnd}
                  onChange={range.setRange}
                />
                <Button
                  variant="outline"
                  size="icon-sm"
                  onClick={range.refresh}
                  aria-label={t("dashboard.refresh")}
                >
                  <RotateCw />
                </Button>
              </div>
            </header>

            <KpiRow range={range} />

            <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_minmax(0,1fr)] gap-3 max-xl:grid-rows-none">
              <div className="stagger-rise grid min-h-0 gap-3 max-xl:auto-rows-[18rem] lg:grid-cols-2 xl:grid-cols-[5fr_3fr_3fr]">
                <UsageTrendPanel range={range} />
                <ReliabilityPanel range={range} />
                <LatencyPanel range={range} />
              </div>
              <div className="stagger-rise grid min-h-0 gap-3 max-xl:auto-rows-[16rem] lg:grid-cols-2 xl:grid-cols-[4fr_4fr_3fr]">
                <EndpointHealthPanel range={range} />
                <TopRankingPanel range={range} />
                <ActivityPanel range={range} />
              </div>
            </div>
          </div>
        </OverviewProvider>
      </TooltipProvider>
    </PermissionGuard>
  );
}
