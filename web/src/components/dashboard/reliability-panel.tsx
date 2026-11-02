"use client";

import { useCallback } from "react";
import { format } from "date-fns";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { cn, formatInteger } from "@/lib/utils";
import {
  failureCodeLabel,
  formatRate,
  successRate,
  topFailures,
  uptimeLevel,
  type UptimeLevel,
} from "@/lib/dashboard";
import type { DashboardFailure, DashboardUptimeBucket } from "@/lib/types";
import { TooltipContent, TooltipRoot, TooltipTrigger } from "@/components/ui/tooltip";
import { Panel } from "@/components/dashboard/panel";
import { useOverview } from "@/components/dashboard/overview-context";
import { useDashboardQuery } from "@/components/dashboard/use-dashboard-query";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

const DAY_MS = 86_400_000;

const LEVEL_FILL: Record<UptimeLevel, string> = {
  ok: "bg-positive",
  warn: "bg-warning",
  bad: "bg-negative",
  idle: "bg-muted",
};

/** 段时间文案：小时级段带时分，天级段只显示日期；跨日时终点补日期，单日段不重复写终点 */
function segmentLabel(seg: DashboardUptimeBucket, spanDays: number): string {
  const start = new Date(seg.start);
  const end = new Date(seg.end);
  if (end.getTime() - start.getTime() < DAY_MS) {
    const sameDay = start.toDateString() === end.toDateString();
    const startText = format(start, spanDays <= 1 ? "HH:mm" : "MM-dd HH:mm");
    return `${startText} – ${format(end, sameDay ? "HH:mm" : "MM-dd HH:mm")}`;
  }
  const from = format(start, "MM-dd");
  const to = format(new Date(end.getTime() - 1), "MM-dd");
  return from === to ? from : `${from} – ${to}`;
}

/** 失败按状态码拆分（后端已降序）：只展示前 k 个，其余合并为一行，各行合计恒等于失败数 */
function FailureCounts({ failures }: { failures: DashboardFailure[] }) {
  const t = useT();
  if (failures.length === 0) return null;
  const { top, rest } = topFailures(failures);
  return (
    <dl className="grid grid-cols-2 gap-x-4 gap-y-0.5">
      {top.map((f) => (
        <div key={f.statusCode} className="flex items-baseline justify-between gap-2">
          <dt className="text-muted-foreground tabular-nums">
            {failureCodeLabel(f.statusCode, t)}
          </dt>
          <dd className="font-mono tabular-nums">{formatInteger(f.count)}</dd>
        </div>
      ))}
      {rest && (
        <div className="flex items-baseline justify-between gap-2">
          <dt className="text-muted-foreground tabular-nums">
            {t("dashboard.failure.rest")} ({rest.codes})
          </dt>
          <dd className="font-mono tabular-nums">{formatInteger(rest.count)}</dd>
        </div>
      )}
    </dl>
  );
}

/** 可用性：固定 32 段的状态条（按失败率分档上色），悬停看每段明细；底部为失败原因合计与 Top 错误 */
export function ReliabilityPanel({ range }: { range: DashboardRange }) {
  const t = useT();
  const { data, loading, error, retry } = useOverview();
  const { startTime, endTime, refreshToken, spanDays } = range;

  const errFetcher = useCallback(async () => {
    const rsp = await api.fetchDashboardErrors({ startTime, endTime, limit: 1 });
    return rsp.data ?? [];
  }, [startTime, endTime]);
  const { data: topErrors } = useDashboardQuery(
    `${startTime}|${endTime}|${refreshToken}`,
    errFetcher,
  );

  // 桶由后端按 overview 的 uptimeBuckets 参数定长切好（空桶为零值），前端不再合并
  const segments = data?.uptime ?? [];

  const summary = data?.current.summary;
  const top = topErrors?.[0];
  const axisFormat = spanDays <= 1 ? "HH:mm" : "MM-dd";

  return (
    <Panel
      title={t("dashboard.panel.reliability")}
      loading={loading}
      error={error}
      empty={!data}
      onRetry={retry}
      action={
        summary && (
          <span className="font-mono text-[11px] text-muted-foreground tabular-nums">
            {formatInteger(summary.requests - summary.success)} / {formatInteger(summary.requests)}
          </span>
        )
      }
      footer={
        top ? (
          <div className="flex min-w-0 items-center gap-1.5">
            <span className="shrink-0">{t("dashboard.panel.top_error")}</span>
            <TooltipRoot>
              <TooltipTrigger
                render={
                  <code className="min-w-0 truncate rounded bg-muted px-1 font-mono text-[10px] text-foreground">
                    {top.statusCode} {top.message || "\u2014"}
                  </code>
                }
              />
              <TooltipContent className="max-w-xs break-all">
                {top.statusCode} {top.message}
              </TooltipContent>
            </TooltipRoot>
            <span className="shrink-0 font-mono tabular-nums">×{formatInteger(top.count)}</span>
          </div>
        ) : (
          <span>{t("dashboard.panel.no_errors")}</span>
        )
      }
    >
      <div className="flex h-full flex-col gap-2 text-[11px]">
        <div className="flex items-baseline justify-end gap-1.5">
          <span className="font-display text-base text-foreground tabular-nums">
            {summary ? formatRate(successRate(summary), 2) : "\u2014"}
          </span>
          <span className="text-muted-foreground">{t("dashboard.kpi.success_rate")}</span>
        </div>

        {/* 状态条固定高度，面板被网格拉高时多余空间均分到上下，避免竖条被拉成细长柱 */}
        <div className="flex min-h-0 flex-1 flex-col justify-center gap-1">
          <div className="flex h-10 gap-[2px]">
            {segments.map((seg, i) => {
              const level = uptimeLevel(seg);
              const label = segmentLabel(seg, spanDays);
              // 段数固定，按位置复用节点：刷新时时间窗右移，各段颜色原地过渡而非整条重建
              return (
                <TooltipRoot key={i}>
                  <TooltipTrigger
                    render={
                      <div
                        role="img"
                        aria-label={`${label} ${formatRate(successRate(seg), 2)}`}
                        className={cn(
                          "min-w-0 flex-1 rounded-[2px] transition-[opacity,background-color] duration-500 hover:opacity-70",
                          LEVEL_FILL[level],
                        )}
                      />
                    }
                  />
                  <TooltipContent className="min-w-44">
                    <div className="flex flex-col gap-1 text-xs">
                      <span className="font-medium tabular-nums">{label}</span>
                      {level === "idle" ? (
                        <span className="text-muted-foreground">
                          {t("dashboard.uptime.no_requests")}
                        </span>
                      ) : (
                        <>
                          <div className="flex items-baseline justify-between gap-4">
                            <span className="text-muted-foreground">
                              {t("dashboard.col.requests")}
                            </span>
                            <span className="font-mono tabular-nums">
                              {formatInteger(seg.requests)}
                            </span>
                          </div>
                          <div className="flex items-baseline justify-between gap-4">
                            <span className="text-muted-foreground">
                              {t("dashboard.kpi.success_rate")}
                            </span>
                            <span className="font-mono tabular-nums">
                              {formatRate(successRate(seg), 2)}
                            </span>
                          </div>
                          <FailureCounts failures={seg.failures} />
                        </>
                      )}
                    </div>
                  </TooltipContent>
                </TooltipRoot>
              );
            })}
          </div>
          {segments.length > 0 && (
            <div className="flex justify-between text-[10px] text-muted-foreground tabular-nums">
              <span>{format(new Date(segments[0].start), axisFormat)}</span>
              <span>{format(new Date(segments[segments.length - 1].end), axisFormat)}</span>
            </div>
          )}
        </div>

        {summary && <FailureCounts failures={summary.failures} />}
      </div>
    </Panel>
  );
}
