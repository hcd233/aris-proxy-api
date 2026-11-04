"use client";

import { useCallback } from "react";
import { ArrowDown, ArrowUpDown } from "lucide-react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { cn, formatInteger, formatTokenCount } from "@/lib/utils";
import { formatCostShort, formatRate } from "@/lib/dashboard";
import type { DashboardDimension, DashboardTopSort } from "@/lib/types";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { TooltipContent, TooltipRoot, TooltipTrigger } from "@/components/ui/tooltip";
import { Panel } from "@/components/dashboard/panel";
import {
  DENSE_CELL,
  DENSE_HEAD,
  SHOW_FROM_360,
  SHOW_FROM_440,
  SHOW_FROM_520,
} from "@/components/dashboard/dense-table";
import { TruncatedText } from "@/components/truncated-text";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

/** 指标表头的排序按钮：当前指标显示降序箭头，另一列显示可切换的提示箭头 */
function SortHeader({
  label,
  active,
  hint,
  onClick,
}: {
  label: string;
  active: boolean;
  hint: string;
  onClick: () => void;
}) {
  return (
    <TooltipRoot>
      <TooltipTrigger
        render={
          <button
            type="button"
            aria-pressed={active}
            onClick={onClick}
            className={cn(
              "inline-flex w-full items-center justify-end gap-0.5 rounded-sm hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
              active && "text-foreground",
            )}
          >
            {label}
            {active ? (
              <ArrowDown className="size-2.5" aria-hidden />
            ) : (
              <ArrowUpDown className="size-2.5 opacity-40" aria-hidden />
            )}
          </button>
        }
      />
      <TooltipContent>{hint}</TooltipContent>
    </TooltipRoot>
  );
}

/**
 * Top 排行：维度、排序指标、占比口径都可切换并持久化；普通用户没有「用户」维度（后端同样拒绝）。
 *
 * 排序决定榜单取哪前 N 名（后端按所选指标重排后再截断），占比只决定条形与百分比按费用还是请求数算。
 * 窄面板（容器 < 360px）本来就不展示请求列，此时也只能按费用排序。
 */
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
  const [storedSortBy, setStoredSortBy] = usePersistentState<string>("dashboard.topSortBy", "cost");
  const sortBy: DashboardTopSort = storedSortBy === "requests" ? "requests" : "cost";
  const [storedShareBy, setStoredShareBy] = usePersistentState<string>(
    "dashboard.topShareBy",
    "cost",
  );
  const shareBy: DashboardTopSort = storedShareBy === "requests" ? "requests" : "cost";
  const { data: overview } = useOverview();
  const { startTime, endTime, refreshToken } = range;

  const fetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardTop({ startTime, endTime, dimension, sortBy, limit: 5 });
    if (!rsp.data) throw new Error("empty top");
    return rsp.data;
  }, [startTime, endTime, dimension, sortBy]);
  const { data, loading, error, retry } = useDashboardQuery(
    `${startTime}|${endTime}|${dimension}|${sortBy}|${refreshToken}`,
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
            <TableHead
              className={cn(DENSE_HEAD, "w-16 text-right", SHOW_FROM_360)}
              aria-sort={sortBy === "requests" ? "descending" : "none"}
            >
              <SortHeader
                label={t("dashboard.col.requests")}
                active={sortBy === "requests"}
                hint={t("dashboard.top.sort_requests")}
                onClick={() => setStoredSortBy("requests")}
              />
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-[3.75rem] text-right", SHOW_FROM_440)}>
              {t("dashboard.col.tokens")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-20 text-right", SHOW_FROM_520)}>
              {t("dashboard.col.cache")}
            </TableHead>
            <TableHead
              className={cn(DENSE_HEAD, "w-16 text-right")}
              aria-sort={sortBy === "cost" ? "descending" : "none"}
            >
              <SortHeader
                label={t("dashboard.col.cost")}
                active={sortBy === "cost"}
                hint={t("dashboard.top.sort_cost")}
                onClick={() => setStoredSortBy("cost")}
              />
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-20")}>
              <TooltipRoot>
                <TooltipTrigger
                  render={
                    <button
                      type="button"
                      onClick={() => setStoredShareBy(shareBy === "cost" ? "requests" : "cost")}
                      className="inline-flex h-5 items-center rounded-full bg-muted px-2 text-[11px] leading-none text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                    >
                      {t(
                        shareBy === "cost"
                          ? "dashboard.col.share_cost"
                          : "dashboard.col.share_requests",
                      )}
                    </button>
                  }
                />
                <TooltipContent>{t("dashboard.top.share_hint")}</TooltipContent>
              </TooltipRoot>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data?.items ?? []).map((item) => {
            const share = shareBy === "cost" ? item.costShare : item.requestShare;
            return (
              <TableRow key={item.key}>
                <TableCell className={DENSE_CELL}>
                  <TruncatedText className="block min-w-0 font-medium">
                    {item.name || item.key}
                  </TruncatedText>
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
                        style={{ width: `${Math.min(100, share * 100)}%` }}
                      />
                    </div>
                    <span className="w-8 text-right text-[11px] text-muted-foreground">
                      {formatRate(share, 0)}
                    </span>
                  </div>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </Panel>
  );
}
