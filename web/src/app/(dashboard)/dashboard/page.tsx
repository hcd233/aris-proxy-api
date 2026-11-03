"use client";

import { useT } from "@/lib/i18n";
import { resolveModelMetric, type ModelMetric } from "@/lib/dashboard";
import { DASHBOARD_REFRESH_INTERVALS } from "@/lib/refresh-interval";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { PermissionGuard } from "@/components/permission-guard";
import { AutoRefreshButton } from "@/components/shared/auto-refresh-button";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ActivityPanel } from "@/components/dashboard/activity-panel";
import { EndpointHealthPanel } from "@/components/dashboard/endpoint-health-panel";
import { KpiRow } from "@/components/dashboard/kpi-row";
import { LatencyPanel } from "@/components/dashboard/latency-panel";
import { OverviewProvider, useOverview } from "@/components/dashboard/overview-context";
import { ReliabilityPanel } from "@/components/dashboard/reliability-panel";
import { TopRankingPanel } from "@/components/dashboard/top-ranking-panel";
import { UsageTrendPanel } from "@/components/dashboard/usage-trend-panel";
import { useDashboardRange, type DashboardRange } from "@/components/dashboard/use-dashboard-range";

/** 刷新按钮：拉取中的状态取共享 overview（KPI 与多数面板都依赖它） */
function RefreshControl({ range }: { range: DashboardRange }) {
  const { loading } = useOverview();
  return (
    <AutoRefreshButton
      intervalMs={range.refreshIntervalMs}
      options={DASHBOARD_REFRESH_INTERVALS}
      onIntervalChange={range.setRefreshIntervalMs}
      cycle={range.refreshCycle}
      onRefresh={range.refresh}
      fetching={loading}
    />
  );
}

/**
 * 一屏指挥台（spec docs/superpowers/specs/2026-10-28-dashboard-redesign-design.md §3）：
 * 页头 → 6 KPI → 用量/可靠性/延迟（5:3:3）→ Endpoint/Top/活跃时段（4:4:3）。
 * xl 及以上锁定视口高度（layout 内边距 lg:p-10 = 5rem），面板内部自适应撑满；更窄时纵向堆叠滚动。
 */
export default function DashboardPage() {
  const t = useT();
  const range = useDashboardRange();
  // 模型用量与 Top 排行共用同一指标口径，持久化后刷新页面保持上次选择
  const [storedMetric, setStoredMetric] = usePersistentState<string>("dashboard.metric", "cost");
  const metric = resolveModelMetric(storedMetric);
  const setMetric = (next: ModelMetric) => setStoredMetric(next);

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
              </div>
              {/* 窄屏独占一行并撑满：范围条 + 刷新按钮不再把自己挤出视口 */}
              <div className="flex max-w-full min-w-0 items-center gap-2 max-sm:w-full max-sm:justify-between">
                <TimeRangePicker
                  value={range.key}
                  customStart={range.customStart}
                  customEnd={range.customEnd}
                  onChange={range.setRange}
                />
                <RefreshControl range={range} />
              </div>
            </header>

            <KpiRow range={range} />

            <div className="grid min-h-0 flex-1 grid-rows-[minmax(0,1fr)_minmax(0,1fr)] gap-3 max-xl:grid-rows-none">
              <div className="stagger-rise grid min-h-0 gap-3 max-xl:auto-rows-[18rem] lg:grid-cols-2 xl:grid-cols-[5fr_3fr_3fr]">
                <UsageTrendPanel range={range} metric={metric} onMetricChange={setMetric} />
                <ReliabilityPanel range={range} />
                <LatencyPanel range={range} />
              </div>
              <div className="stagger-rise grid min-h-0 gap-3 max-xl:auto-rows-[16rem] lg:grid-cols-2 xl:grid-cols-[4fr_4fr_3fr]">
                <EndpointHealthPanel range={range} />
                <TopRankingPanel range={range} metric={metric} />
                <ActivityPanel range={range} />
              </div>
            </div>
          </div>
        </OverviewProvider>
      </TooltipProvider>
    </PermissionGuard>
  );
}
