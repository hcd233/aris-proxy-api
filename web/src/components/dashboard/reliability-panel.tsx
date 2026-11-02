"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { format } from "date-fns";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { cn, formatInteger } from "@/lib/utils";
import {
  failureCodeLabel,
  fitFailureTopK,
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

/** 紧凑状态码清单（段悬停浮层、「其余」浮层共用）：全部列出，不折叠 */
function FailureList({ failures }: { failures: DashboardFailure[] }) {
  const t = useT();
  if (failures.length === 0) return null;
  return (
    <dl className="flex flex-col gap-0.5">
      {failures.map((f) => (
        <div key={f.statusCode} className="flex items-baseline justify-between gap-4">
          <dt className="text-muted-foreground tabular-nums">
            {failureCodeLabel(f.statusCode, t)}
          </dt>
          <dd className="font-mono tabular-nums">{formatInteger(f.count)}</dd>
        </div>
      ))}
    </dl>
  );
}

/** 失败分布一行：状态码 · 占比条 · 次数 · 占比 */
function FailureRow({ label, count, total }: { label: string; count: number; total: number }) {
  const share = total > 0 ? count / total : 0;
  return (
    <>
      <span className="text-muted-foreground tabular-nums">{label}</span>
      <span className="h-1.5 overflow-hidden rounded-full bg-muted">
        <span
          className="block h-full rounded-full bg-negative/70"
          style={{ width: `${Math.max(share * 100, 1)}%` }}
        />
      </span>
      <span className="text-right font-mono tabular-nums">{formatInteger(count)}</span>
      <span className="w-9 text-right font-mono text-muted-foreground tabular-nums">
        {formatRate(share, 0)}
      </span>
    </>
  );
}

/** 失败分布单行高度与行距（px），与下方 grid 的 auto-rows-[20px] gap-y-1 保持一致 */
const FAILURE_ROW_PX = 20;
const FAILURE_ROW_GAP_PX = 4;

/** 量出容器当前能放下几行失败分布（ResizeObserver 跟随面板尺寸变化；首帧未量到前为 null） */
function useRowCapacity() {
  const ref = useRef<HTMLDivElement>(null);
  const [rows, setRows] = useState<number | null>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(([entry]) => {
      const h = entry.contentRect.height;
      setRows(Math.floor((h + FAILURE_ROW_GAP_PX) / (FAILURE_ROW_PX + FAILURE_ROW_GAP_PX)));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return { ref, rows };
}

/**
 * 失败分布（后端已降序）：按容器实际高度自适应 Top-K——放得下就全展示（首帧未量到高度时同样全展示），
 * 只有容器放不下时才前 k 个各占一行、其余合并为一行并以浮层展示明细；各行合计恒等于失败数。
 */
function FailureBreakdown({ failures }: { failures: DashboardFailure[] }) {
  const t = useT();
  const { ref, rows } = useRowCapacity();
  const total = failures.reduce((s, f) => s + f.count, 0);
  const k = rows == null ? failures.length : fitFailureTopK(failures.length, rows);
  const { top, rest } = topFailures(failures, k);
  return (
    <div ref={ref} className="min-h-0 flex-1 overflow-hidden">
      <div className="grid auto-rows-[20px] grid-cols-[auto_1fr_auto_auto] items-center gap-x-3 gap-y-1">
        {top.map((f) => (
          <FailureRow
            key={f.statusCode}
            label={failureCodeLabel(f.statusCode, t)}
            count={f.count}
            total={total}
          />
        ))}
        {rest && (
          <TooltipRoot>
            <TooltipTrigger
              render={
                <div className="col-span-4 grid cursor-default grid-cols-subgrid items-center rounded-sm transition-colors hover:bg-muted/60">
                  <FailureRow
                    label={`${t("dashboard.failure.rest")} (${rest.items.length})`}
                    count={rest.count}
                    total={total}
                  />
                </div>
              }
            />
            <TooltipContent className="min-w-36 text-xs">
              <FailureList failures={rest.items} />
            </TooltipContent>
          </TooltipRoot>
        )}
      </div>
    </div>
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
      <div className="flex h-full min-h-0 flex-col gap-2.5 overflow-hidden text-[11px]">
        <div className="flex shrink-0 items-baseline justify-end gap-1.5">
          <span className="font-display text-base text-foreground tabular-nums">
            {summary ? formatRate(successRate(summary), 2) : "\u2014"}
          </span>
          <span className="text-muted-foreground">{t("dashboard.kpi.success_rate")}</span>
        </div>

        {/* 状态条与失败分布按 1:2 分配剩余高度：状态条夹在 56~112px 之间（大屏不拉成细长柱、小屏不压扁），
            失败分布按实际可用行数自适应 Top-K，矮面板也不会溢出压到页脚 */}
        <div className="flex max-h-28 min-h-14 flex-[1_1_0] flex-col gap-1">
          <div className="flex min-h-0 flex-1 gap-[2px]">
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
                          <FailureList failures={seg.failures} />
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

        {summary && summary.failures.length > 0 && (
          <div className="flex min-h-0 flex-[2_1_0] flex-col gap-1.5 border-t border-border/60 pt-2">
            <span className="shrink-0 text-[10px] tracking-wide text-muted-foreground">
              {t("dashboard.failure.title")}
            </span>
            <FailureBreakdown failures={summary.failures} />
          </div>
        )}
      </div>
    </Panel>
  );
}
