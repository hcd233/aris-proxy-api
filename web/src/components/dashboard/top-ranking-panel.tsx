"use client";

import { useCallback } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
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
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Panel } from "@/components/dashboard/panel";
import {
  DENSE_CELL,
  DENSE_HEAD,
  SHOW_FROM_360,
  SHOW_FROM_440,
  SHOW_FROM_520,
  TruncatedName,
} from "@/components/dashboard/dense-table";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

/** Top 排行：维度可切换并持久化；普通用户没有「用户」维度（后端同样拒绝） */
export function TopRankingPanel({ range }: { range: DashboardRange }) {
  const t = useT();
  const { isAdmin, isDemo } = useAuth();
  const dims: DashboardDimension[] =
    isAdmin() || isDemo() ? ["model", "user", "apikey", "harness"] : ["model", "apikey", "harness"];
  const [storedDimension, setStoredDimension] = usePersistentState<string>(
    "dashboard.topDimension",
    "model",
  );
  // 同一浏览器可能换号登录：存的维度不在当前身份可选范围内（如普通用户读到 user）时回落到模型
  const dimension = dims.find((d) => d === storedDimension) ?? "model";
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
      bodyClassName="@container px-1.5"
      action={
        <ToggleGroup
          variant="segmented"
          size="sm"
          spacing={0.5}
          value={[dimension]}
          onValueChange={(next) => {
            if (next[0]) setStoredDimension(String(next[0]));
          }}
        >
          {dims.map((d) => (
            <ToggleGroupItem key={d} value={d} className="h-6 px-2 text-[11px]">
              {t(`dashboard.dim.${d}`)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      }
    >
      <Table fit>
        <TableHeader className="bg-transparent">
          <TableRow className="hover:bg-transparent">
            <TableHead className={DENSE_HEAD}>{t(`dashboard.dim.${dimension}`)}</TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-16 text-right", SHOW_FROM_360)}>
              {t("dashboard.col.requests")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-[3.75rem] text-right", SHOW_FROM_440)}>
              {t("dashboard.col.tokens")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-20 text-right", SHOW_FROM_520)}>
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
              <TableCell className={cn(DENSE_CELL, "text-right", SHOW_FROM_360)}>
                {formatInteger(item.requests)}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right", SHOW_FROM_440)}>
                {formatTokenCount(item.tokens)}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right", SHOW_FROM_520)}>
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
                  <span className="w-8 text-right text-[11px] text-muted-foreground">
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
