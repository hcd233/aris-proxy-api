"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api-client";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { cn, formatInteger, formatTokenCount } from "@/lib/utils";
import { formatCostShort, formatRate } from "@/lib/dashboard";
import type { DashboardDimension } from "@/lib/types";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SegmentedChoice } from "@/components/dashboard/segmented-choice";
import { Panel } from "@/components/dashboard/panel";
import { DENSE_CELL, DENSE_HEAD, TruncatedName } from "@/components/dashboard/dense-table";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

/** Top 排行：维度可切换；普通用户没有「用户」维度（后端同样拒绝） */
export function TopRankingPanel({ range }: { range: DashboardRange }) {
  const t = useT();
  const { isAdmin, isDemo } = useAuth();
  const dims: DashboardDimension[] =
    isAdmin() || isDemo() ? ["model", "user", "apikey", "harness"] : ["model", "apikey", "harness"];
  const [dimension, setDimension] = useState<DashboardDimension>("model");
  const { data: overview } = useOverview();
  const { startTime, endTime, refreshToken } = range;

  const fetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardTop({ startTime, endTime, dimension, limit: 5 });
    if (!rsp.data) throw new Error("empty top");
    return rsp.data;
  }, [startTime, endTime, dimension]);
  const { data, loading, error, retry } = useDashboardQuery(
    `${startTime}|${endTime}|${dimension}|${refreshToken}`,
    fetcher,
  );
  const currency = overview?.currency;

  return (
    <Panel
      title={t("dashboard.panel.top")}
      loading={loading}
      error={error}
      empty={!data || data.items.length === 0}
      onRetry={retry}
      bodyClassName="px-1.5"
      action={
        <SegmentedChoice
          value={dimension}
          options={dims.map((d) => ({ value: d, label: t(`dashboard.dim.${d}`) }))}
          onChange={setDimension}
          aria-label={t("dashboard.panel.top")}
        />
      }
    >
      <Table fit>
        <TableHeader className="bg-transparent">
          <TableRow className="hover:bg-transparent">
            <TableHead className={DENSE_HEAD}>{t(`dashboard.dim.${dimension}`)}</TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-14 text-right")}>
              {t("dashboard.col.requests")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-14 text-right")}>
              {t("dashboard.col.tokens")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-12 text-right")}>
              {t("dashboard.col.cache")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-16 text-right")}>
              {t("dashboard.col.cost")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-20")}>{t("dashboard.col.share")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data?.items ?? []).map((item) => (
            <TableRow key={item.key}>
              <TableCell className={DENSE_CELL}>
                <TruncatedName text={item.name || item.key} className="font-medium" />
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {formatInteger(item.requests)}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {formatTokenCount(item.tokens)}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {formatRate(item.cacheHitRate, 0)}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {formatCostShort(item.cost, currency)}
              </TableCell>
              <TableCell className={DENSE_CELL}>
                <div className="flex items-center gap-1.5">
                  <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                    <div
                      className="h-full rounded-full bg-primary"
                      style={{ width: `${Math.min(100, item.costShare * 100)}%` }}
                    />
                  </div>
                  <span className="w-8 text-right text-[10px] text-muted-foreground">
                    {formatRate(item.costShare, 0)}
                  </span>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Panel>
  );
}
