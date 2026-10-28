"use client";

import { createContext, useCallback, useContext, type ReactNode } from "react";
import { api } from "@/lib/api-client";
import type { DashboardOverview } from "@/lib/types";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";
import { useDashboardQuery, type DashboardQuery } from "@/components/dashboard/use-dashboard-query";

const OverviewContext = createContext<DashboardQuery<DashboardOverview> | null>(null);

/** 拉取一次 overview，供 KPI 行、可靠性、延迟、用量趋势（上一周期虚线）共享 */
export function OverviewProvider({
  range,
  children,
}: {
  range: DashboardRange;
  children: ReactNode;
}) {
  const { startTime, endTime, granularity, refreshToken } = range;
  const fetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardOverview({ startTime, endTime, granularity });
    if (!rsp.data) throw new Error("empty overview");
    return rsp.data;
  }, [startTime, endTime, granularity]);
  const query = useDashboardQuery(
    `${startTime}|${endTime}|${granularity}|${refreshToken}`,
    fetcher,
  );
  return <OverviewContext.Provider value={query}>{children}</OverviewContext.Provider>;
}

export function useOverview(): DashboardQuery<DashboardOverview> {
  const ctx = useContext(OverviewContext);
  if (!ctx) throw new Error("useOverview must be used within OverviewProvider");
  return ctx;
}
