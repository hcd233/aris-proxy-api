"use client";

import { useCallback } from "react";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { cn, formatDecimal, formatDurationMs, formatInteger } from "@/lib/utils";
import { formatCostShort, formatRate } from "@/lib/dashboard";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Panel } from "@/components/dashboard/panel";
import { DENSE_CELL, DENSE_HEAD, TruncatedName } from "@/components/dashboard/dense-table";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

/** 错误率健康分档：< 1% 正常，1–5% 警示，≥ 5% 异常（spec §3） */
function healthDot(errorRate: number): string {
  if (errorRate >= 0.05) return "bg-negative";
  if (errorRate >= 0.01) return "bg-warning";
  return "bg-positive";
}

/** Endpoint 健康表：请求 · 错误率 · P95 · 输出速度 · 费用（按请求数降序前 5） */
export function EndpointHealthPanel({ range }: { range: DashboardRange }) {
  const t = useT();
  const { data: overview } = useOverview();
  const { startTime, endTime, refreshToken } = range;
  const fetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardEndpoints({ startTime, endTime, limit: 5 });
    return rsp.data ?? [];
  }, [startTime, endTime]);
  const { data, loading, error, retry } = useDashboardQuery(
    `${startTime}|${endTime}|${refreshToken}`,
    fetcher,
  );

  return (
    <Panel
      title={t("dashboard.panel.endpoints")}
      loading={loading}
      error={error}
      empty={!data || data.length === 0}
      onRetry={retry}
      bodyClassName="px-1.5"
    >
      <Table fit>
        <TableHeader className="bg-transparent">
          <TableRow className="hover:bg-transparent">
            <TableHead className={DENSE_HEAD}>{t("dashboard.col.name")}</TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-14 text-right")}>
              {t("dashboard.col.requests")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-14 text-right")}>
              {t("dashboard.col.error_rate")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-14 text-right")}>
              {t("dashboard.col.p95")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-12 text-right")}>
              {t("dashboard.col.speed")}
            </TableHead>
            <TableHead className={cn(DENSE_HEAD, "w-16 text-right")}>
              {t("dashboard.col.cost")}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data ?? []).map((row, i) => (
            // demo 下 endpoint 名已脱敏，短名可能脱敏成同一占位符，key 加序号防冲突
            <TableRow key={`${i}:${row.endpoint}`}>
              <TableCell className={DENSE_CELL}>
                <div className="flex min-w-0 items-center gap-1.5">
                  <span
                    className={cn("size-1.5 shrink-0 rounded-full", healthDot(row.errorRate))}
                  />
                  <TruncatedName text={row.endpoint} className="font-medium" />
                </div>
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {formatInteger(row.requests)}
              </TableCell>
              <TableCell
                className={cn(
                  DENSE_CELL,
                  "text-right",
                  row.errorRate >= 0.05
                    ? "text-negative"
                    : row.errorRate >= 0.01
                      ? "text-warning"
                      : "",
                )}
              >
                {formatRate(row.errorRate)}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {formatDurationMs(row.p95LatencyMs)}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {row.outputTokensPerSecond != null
                  ? formatDecimal(row.outputTokensPerSecond, 0)
                  : "\u2014"}
              </TableCell>
              <TableCell className={cn(DENSE_CELL, "text-right")}>
                {formatCostShort(row.cost, overview?.currency)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Panel>
  );
}
