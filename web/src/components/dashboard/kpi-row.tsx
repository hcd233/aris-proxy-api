"use client";

import { useMemo, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";
import { cn, formatDurationMs, formatInteger, formatTokenCount, formatDecimal } from "@/lib/utils";
import {
  cacheHitRate,
  deltaDiff,
  deltaPct,
  deltaTone,
  formatCostShort,
  formatDelta,
  formatRate,
  KPI_SPECS,
  peakBucket,
  promptTokens,
  rangeDays,
  successRate,
  totalTokens,
  type DeltaTone,
  type KpiKind,
} from "@/lib/dashboard";
import type { DashboardStats, DashboardStatsPoint } from "@/lib/types";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/dashboard/sparkline";
import { useOverview } from "@/components/dashboard/overview-context";
import type { DashboardRange } from "@/components/dashboard/use-dashboard-range";

const TONE_CLASS: Record<DeltaTone, string> = {
  good: "text-positive",
  bad: "text-negative",
  neutral: "text-muted-foreground",
};

interface KpiSpec {
  kind: KpiKind;
  label: string;
  value: ReactNode;
  unit?: string;
  delta: number | null;
  sub: string[];
  series: number[];
  previous: number[];
  lineClass: string;
}

function KpiCard({ spec, loading }: { spec: KpiSpec; loading: boolean }) {
  const tone = deltaTone(spec.kind, spec.delta);
  return (
    <Card size="sm" className="gap-1 px-3 py-2.5" data-interactive>
      <div className="flex items-center justify-between gap-2">
        <span className="text-[11px] font-medium tracking-wider whitespace-nowrap text-muted-foreground uppercase">
          {spec.label}
        </span>
        {!loading && (
          <span
            className={cn(
              "font-mono text-[10px] font-semibold whitespace-nowrap tabular-nums",
              TONE_CLASS[tone],
            )}
          >
            {formatDelta(spec.kind, spec.delta)}
          </span>
        )}
      </div>
      {loading ? (
        <>
          <Skeleton className="h-7 w-24" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-6 w-full" />
        </>
      ) : (
        <>
          <div className="font-display text-2xl leading-tight font-normal whitespace-nowrap text-foreground tabular-nums">
            {spec.value}
            {spec.unit && <span className="ml-1 text-xs text-muted-foreground">{spec.unit}</span>}
          </div>
          <div className="min-h-[2lh] text-[11px] leading-snug text-muted-foreground tabular-nums">
            {spec.sub.join(" · ")}
          </div>
          <Sparkline values={spec.series} previous={spec.previous} className={spec.lineClass} />
        </>
      )}
    </Card>
  );
}

function pick(
  series: DashboardStatsPoint[] | undefined,
  f: (s: DashboardStats) => number | null | undefined,
) {
  return (series ?? []).map((p) => f(p) ?? Number.NaN);
}

/** 6 张 KPI：主值 + 环比 + 2-3 个辅助值 + 本期/上一周期迷你趋势 */
export function KpiRow({ range }: { range: DashboardRange }) {
  const { t, locale } = useI18n();
  const { data, loading } = useOverview();
  const isLoading = loading && !data;

  const specs = useMemo<KpiSpec[]>(() => {
    const cur = data?.current.summary;
    const prev = data?.previous.summary;
    const cs = data?.current.series;
    const ps = data?.previous.series;
    const currency = data?.currency;
    const days = rangeDays(range.startTime, range.endTime);
    if (!cur || !prev) {
      // 加载失败或尚无数据：保留标题，数值显示「—」（重试入口在共享 overview 的面板上）
      return KPI_SPECS.map(({ kind, labelKey }) => ({
        kind,
        label: t(labelKey),
        value: "\u2014",
        delta: null,
        sub: [],
        series: [],
        previous: [],
        lineClass: "",
      }));
    }
    const curTokens = totalTokens(cur.tokens);
    const curRate = successRate(cur);
    const curHit = cacheHitRate(cur.tokens);
    const failed = cur.requests - cur.success;
    return [
      {
        kind: "requests",
        label: t("dashboard.kpi.requests"),
        value: formatInteger(cur.requests),
        delta: deltaPct(cur.requests, prev.requests),
        sub: [
          `${t("dashboard.kpi.daily_avg")} ${formatInteger(cur.requests / days)}`,
          `${t("dashboard.kpi.peak")} ${formatInteger(peakBucket(cs ?? []))}`,
        ],
        series: pick(cs, (s) => s.requests),
        previous: pick(ps, (s) => s.requests),
        lineClass: "text-primary",
      },
      {
        kind: "tokens",
        label: t("dashboard.kpi.tokens"),
        value: formatTokenCount(curTokens),
        delta: deltaPct(curTokens, totalTokens(prev.tokens)),
        sub: [
          `${t("dashboard.kpi.input")} ${formatTokenCount(promptTokens(cur.tokens))} · ${t("dashboard.kpi.output")} ${formatTokenCount(cur.tokens.output)}`,
          `${formatTokenCount(cur.success > 0 ? curTokens / cur.success : 0)} ${t("dashboard.kpi.per_request")}`,
        ],
        series: pick(cs, (s) => totalTokens(s.tokens)),
        previous: pick(ps, (s) => totalTokens(s.tokens)),
        lineClass: "text-primary",
      },
      {
        kind: "cost",
        label: t("dashboard.kpi.cost"),
        value: formatCostShort(cur.cost, currency),
        delta: deltaPct(cur.cost, prev.cost),
        sub: [
          `${t("dashboard.kpi.daily_avg")} ${formatCostShort(cur.cost / days, currency)} · ${formatCostShort(cur.pricedSuccess > 0 ? (cur.cost / cur.pricedSuccess) * 1000 : 0, currency)}${t("dashboard.kpi.per_1k")}`,
          `${t("dashboard.kpi.priced")} ${formatRate(cur.success > 0 ? cur.pricedSuccess / cur.success : null)}`,
        ],
        series: pick(cs, (s) => s.cost),
        previous: pick(ps, (s) => s.cost),
        lineClass: "text-primary",
      },
      {
        kind: "successRate",
        label: t("dashboard.kpi.success_rate"),
        value: formatRate(curRate, 2),
        delta: deltaDiff(curRate, successRate(prev)),
        sub: [
          `${t("dashboard.kpi.failures")} ${formatInteger(failed)}`,
          `429 ${formatRate(failed > 0 ? cur.failures.rateLimit / failed : null, 0)} · 5xx ${formatRate(failed > 0 ? cur.failures.server / failed : null, 0)}`,
        ],
        series: pick(cs, successRate),
        previous: pick(ps, successRate),
        lineClass: "text-positive",
      },
      {
        kind: "cacheHitRate",
        label: t("dashboard.kpi.cache_hit"),
        value: formatRate(curHit),
        delta: deltaDiff(curHit, cacheHitRate(prev.tokens)),
        sub: [
          `${t("dashboard.kpi.cache_read")} ${formatTokenCount(cur.tokens.cacheRead)} · ${t("dashboard.kpi.cache_write")} ${formatTokenCount(cur.tokens.cacheWrite)}`,
          `${t("dashboard.kpi.cache_miss")} ${formatTokenCount(cur.tokens.input)}`,
        ],
        series: pick(cs, (s) => cacheHitRate(s.tokens)),
        previous: pick(ps, (s) => cacheHitRate(s.tokens)),
        lineClass: "text-positive",
      },
      {
        kind: "latency",
        label: t("dashboard.kpi.latency"),
        value: formatDurationMs(cur.latency.p95),
        delta: deltaDiff(cur.latency.p95, prev.latency.p95),
        sub: [
          `P50 ${formatDurationMs(cur.latency.p50)}`,
          `${t("dashboard.kpi.speed")} ${cur.outputTokensPerSecond != null ? formatDecimal(cur.outputTokensPerSecond, 0) : "\u2014"} tok/s`,
        ],
        series: pick(cs, (s) => s.latency.p95),
        previous: pick(ps, (s) => s.latency.p95),
        lineClass: "text-chart-3",
      },
    ];
    // locale 驱动翻译刷新（t 引用稳定，见 lib/i18n.tsx）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, locale, range.startTime, range.endTime]);

  return (
    <div className="stagger-rise grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6">
      {specs.map((spec) => (
        <KpiCard key={spec.kind} spec={spec} loading={isLoading} />
      ))}
    </div>
  );
}
