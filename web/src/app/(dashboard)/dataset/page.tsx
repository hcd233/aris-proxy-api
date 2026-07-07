"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  type LucideIcon,
  Database,
  Download,
  ChevronLeft,
  ChevronRight,
  Check,
  Hash,
  Star,
  ChevronDown,
  SlidersHorizontal,
  BarChart3,
  FileDown,
} from "lucide-react";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
import { ScoreSlider } from "@/components/dataset/score-slider";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { cn } from "@/lib/utils";

function useDebouncedCallback(cb: () => void, delay: number) {
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const cbRef = useRef(cb);
  useEffect(() => {
    cbRef.current = cb;
  });

  return useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => cbRef.current(), delay);
  }, [delay]);
}

function MetricCard({
  label,
  value,
  hint,
  icon: Icon,
}: {
  label: string;
  value: string;
  hint: string;
  icon: LucideIcon;
}) {
  return (
    <Card className="border-border/70 bg-card/80 py-0 shadow-sm shadow-border/20">
      <CardContent className="p-4">
        <div className="mb-4 flex items-center justify-between gap-3">
          <span className="text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
            {label}
          </span>
          <span className="flex size-8 items-center justify-center rounded-lg bg-secondary text-primary">
            <Icon className="size-4" />
          </span>
        </div>
        <div
          className="truncate font-display text-2xl font-semibold tracking-tight text-foreground md:text-3xl"
          title={value}
        >
          {value}
        </div>
        <p className="mt-1 truncate text-xs text-muted-foreground" title={hint}>
          {hint}
        </p>
      </CardContent>
    </Card>
  );
}

function EmptyDataset({ title, hint }: { title: string; hint: string }) {
  return (
    <Card className="min-h-[420px] justify-center border-dashed bg-card/70">
      <CardContent className="flex flex-col items-center justify-center py-16 text-center">
        <div className="mb-5 grid size-16 place-items-center rounded-2xl border border-border bg-secondary/60 shadow-inner">
          <Database className="size-8 text-muted-foreground/50" />
        </div>
        <p className="font-display text-xl font-semibold text-foreground">
          {title}
        </p>
        <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
          {hint}
        </p>
      </CardContent>
    </Card>
  );
}

export default function DatasetPage() {
  const t = useT();

  const [minScore, setMinScore] = useState(4);
  const [filterModels, setFilterModels] = usePersistentState<string[]>(
    "dashboard.dataset.filterModels",
    []
  );
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>(
    "dashboard.dataset.timeRange",
    "30d"
  );
  const [customStart, setCustomStart] = usePersistentState(
    "dashboard.dataset.customStart",
    ""
  );
  const [customEnd, setCustomEnd] = usePersistentState(
    "dashboard.dataset.customEnd",
    ""
  );

  const [preview, setPreview] = useState<{
    totalSessions: number;
    scoreDistribution: Record<number, number>;
    modelDistribution: Record<string, number>;
  } | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const [formatOffset, setFormatOffset] = useState(0);
  const [formatData, setFormatData] = useState<{
    sessionId?: number;
    offset?: number;
    totalCount?: number;
    sharegptJson?: string;
  } | null>(null);
  const [formatLoading, setFormatLoading] = useState(false);
  const [formatExpanded, setFormatExpanded] = useState(false);

  const [exporting, setExporting] = useState(false);
  const [exportProgress, setExportProgress] = useState(0);
  const [exportCurrent, setExportCurrent] = useState(0);
  const [exportTotal, setExportTotal] = useState(0);

  const [modelOptions, setModelOptions] = useState<string[]>([]);

  const buildFilterParams = useCallback(() => {
    const { startTime, endTime } = computeRange(
      timeRange,
      customStart,
      customEnd
    );
    return {
      minScore,
      models: filterModels.length > 0 ? filterModels : undefined,
      startTime,
      endTime,
    };
  }, [minScore, filterModels, timeRange, customStart, customEnd]);

  const buildExportParams = useCallback(() => {
    const { startTime, endTime } = computeRange(
      timeRange,
      customStart,
      customEnd
    );
    return {
      minScore,
      models: filterModels.length > 0 ? filterModels : undefined,
      startTime,
      endTime,
    };
  }, [minScore, filterModels, timeRange, customStart, customEnd]);

  const fetchModelOptions = useCallback(async () => {
    const { startTime, endTime } = computeRange(
      timeRange,
      customStart,
      customEnd
    );
    try {
      const rsp = await api.listSessionOptions({
        field: "model",
        startTime,
        endTime,
      });
      if (!rsp.error && rsp.items) setModelOptions(rsp.items);
    } catch {
      // silent
    }
  }, [timeRange, customStart, customEnd]);

  const fetchPreview = useCallback(async () => {
    setPreviewLoading(true);
    try {
      const rsp = await api.previewDataset(buildFilterParams());
      if (rsp.error) {
        toast.error(rsp.error.message ?? t("dataset.preview_error"));
        setPreview(null);
        return;
      }
      setPreview({
        totalSessions: rsp.totalSessions ?? 0,
        scoreDistribution: rsp.scoreDistribution ?? {},
        modelDistribution: rsp.modelDistribution ?? {},
      });
    } catch {
      // silent
    } finally {
      setPreviewLoading(false);
    }
  }, [buildFilterParams, t]);

  const fetchFormatPreview = useCallback(
    async (offset: number) => {
      setFormatLoading(true);
      try {
        const rsp = await api.previewDatasetFormat({
          ...buildFilterParams(),
          offset,
        });
        if (rsp.error) {
          toast.error(rsp.error.message ?? t("dataset.preview_error"));
          return;
        }
        setFormatData({
          sessionId: rsp.sessionId,
          offset: rsp.offset ?? offset,
          totalCount: rsp.totalCount,
          sharegptJson: rsp.sharegptJson,
        });
      } catch {
        // silent
      } finally {
        setFormatLoading(false);
      }
    },
    [buildFilterParams, t]
  );

  const debouncedRefresh = useDebouncedCallback(() => {
    fetchPreview();
    fetchModelOptions();
  }, 300);

  useEffect(() => {
    debouncedRefresh();
  }, [minScore, filterModels, timeRange, customStart, customEnd, debouncedRefresh]);

  /* eslint-disable react-hooks/set-state-in-effect -- Fetch format preview when expanded or offset changes */
  useEffect(() => {
    if (formatExpanded && (preview?.totalSessions ?? 0) > 0) {
      fetchFormatPreview(formatOffset);
    } else if ((preview?.totalSessions ?? 0) === 0) {
      setFormatData(null);
    }
  }, [preview?.totalSessions, formatOffset, fetchFormatPreview, formatExpanded]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handlePrevSession = useCallback(() => {
    setFormatOffset((prev) => Math.max(0, prev - 1));
  }, []);

  const handleNextSession = useCallback(() => {
    setFormatOffset((prev) => {
      const max = (formatData?.totalCount ?? 1) - 1;
      return Math.min(max, prev + 1);
    });
  }, [formatData?.totalCount]);

  const handleExport = useCallback(async () => {
    const params = buildExportParams();
    setExporting(true);
    setExportProgress(0);
    setExportCurrent(0);
    setExportTotal(0);

    const lines: string[] = [];

    try {
      await api.exportDatasetStream(params, (event) => {
        switch (event.event) {
          case "start":
            setExportTotal(event.data.totalSessions);
            break;
          case "data":
            lines.push(event.data.json);
            setExportCurrent(event.data.current);
            setExportProgress(event.data.progress);
            break;
          case "done":
            setExportCurrent(event.data.totalSessions);
            setExportProgress(100);
            break;
          case "error":
            throw new Error(event.data.message);
        }
      });

      const blob = new Blob([lines.join("\n")], {
        type: "application/jsonl",
      });

      const fmtMinScore = params.minScore || 0;
      const fmtModels = params.models?.length ?? 0;
      const dateStr = new Date()
        .toISOString()
        .slice(0, 10)
        .replace(/-/g, "");
      const filename = `dataset_${dateStr}_minScore${fmtMinScore}_models${fmtModels}_${lines.length}sessions.jsonl`;

      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      toast.success(t("dataset.export_success"));
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("dataset.export_error")
      );
    } finally {
      setTimeout(() => {
        setExporting(false);
      }, 1500);
    }
  }, [buildExportParams, t]);

  const sharegptJson = formatData?.sharegptJson;
  const formattedJSON = useMemo(() => {
    if (!sharegptJson) return "";
    try {
      return JSON.stringify(JSON.parse(sharegptJson), null, 2);
    } catch {
      return sharegptJson;
    }
  }, [sharegptJson]);

  const hasFilters =
    minScore > 0 || filterModels.length > 0 || customStart || customEnd;
  const totalSessions = preview?.totalSessions ?? 0;
  const hasData = totalSessions > 0;
  const scoreEntries = Object.entries(preview?.scoreDistribution ?? {})
    .map(([score, count]) => [Number(score), count] as const)
    .sort(([a], [b]) => a - b);
  const modelEntries = Object.entries(preview?.modelDistribution ?? {}).sort(
    ([, a], [, b]) => b - a
  );
  const maxScoreCount = Math.max(...scoreEntries.map(([, count]) => count), 1);
  const maxModelCount = Math.max(...modelEntries.map(([, count]) => count), 1);
  const activeFilterCount =
    (minScore > 0 ? 1 : 0) +
    (filterModels.length > 0 ? 1 : 0) +
    (timeRange !== "30d" || customStart || customEnd ? 1 : 0);
  const visibleScoreCount = scoreEntries
    .filter(([score]) => score >= minScore)
    .reduce((sum, [, count]) => sum + count, 0);
  const retainedPct = totalSessions
    ? Math.round((visibleScoreCount / totalSessions) * 100)
    : 0;
  const topModelLabel = modelEntries[0]?.[0] ?? "--";
  const modelFilterLabel =
    filterModels.length > 0
      ? filterModels.join(", ")
      : t("dataset.export_filter_all_models");

  const timeRangeLabel = useMemo(() => {
    const labels: Record<TimeRangeKey, string> = {
      "1h": t("time.last_1h"),
      "24h": t("time.last_24h"),
      "7d": t("time.last_7d"),
      "30d": t("time.last_30d"),
      custom:
        customStart && customEnd
          ? `${customStart} ~ ${customEnd}`
          : t("time.custom"),
    };
    return labels[timeRange];
  }, [timeRange, customStart, customEnd, t]);

  const steps = [
    {
      num: 1,
      label: t("dataset.step_configure"),
      desc: t("dataset.step_configure_desc"),
      icon: SlidersHorizontal,
      active: true,
    },
    {
      num: 2,
      label: t("dataset.step_review"),
      desc: t("dataset.step_review_desc"),
      icon: BarChart3,
      active: hasData || previewLoading,
    },
    {
      num: 3,
      label: t("dataset.step_export"),
      desc: t("dataset.step_export_desc"),
      icon: FileDown,
      active: hasData && !previewLoading,
    },
  ];

  return (
    <div className="space-y-6">
      <section className="relative overflow-hidden rounded-3xl border border-border/70 bg-card shadow-sm">
        <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-primary via-amber-300 to-chart-2" />
        <div className="absolute right-0 top-0 size-64 translate-x-20 -translate-y-24 rounded-full bg-primary/10 blur-3xl" />
        <div className="relative grid gap-6 p-5 md:p-7 lg:grid-cols-[minmax(0,1fr)_390px] lg:items-end">
          <div className="space-y-4">
            <Badge variant="secondary" className="w-fit gap-1.5">
              <Database className="size-3.5" />
              {t("dataset.pipeline_badge", "Lakehouse training pipeline")}
            </Badge>
            <div>
              <h1 className="font-display text-4xl font-semibold tracking-tight text-foreground md:text-5xl">
                {t("dataset.title")}
              </h1>
              <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground md:text-base">
                {t("dataset.subtitle")}
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Badge variant="outline" className="gap-1.5 bg-background/70">
                <FileDown className="size-3" />
                {t("dataset.export_format_value")}
              </Badge>
              <Badge variant="outline" className="gap-1.5 bg-background/70">
                <SlidersHorizontal className="size-3" />
                {activeFilterCount} {t("dataset.active_filters", "active filters")}
              </Badge>
              {previewLoading && (
                <Badge variant="secondary" className="gap-1.5">
                  <BarChart3 className="size-3" />
                  {t("dataset.preview")}
                </Badge>
              )}
            </div>
          </div>

          <div className="grid gap-2 rounded-2xl border border-border/70 bg-background/70 p-2 shadow-inner backdrop-blur">
            {steps.map((step, idx) => {
              const Icon = step.icon;
              return (
                <div key={step.num} className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3">
                  <div className="flex flex-col items-center">
                    <div
                      className={cn(
                        "grid size-8 place-items-center rounded-full border transition-colors",
                        step.active
                          ? "border-primary/30 bg-primary text-primary-foreground"
                          : "border-border bg-card text-muted-foreground"
                      )}
                    >
                      <Icon className="size-3.5" />
                    </div>
                    {idx < steps.length - 1 && (
                      <div className="h-5 w-px bg-border" />
                    )}
                  </div>
                  <div className="min-w-0 pb-2">
                    <p className="truncate text-sm font-semibold text-foreground">
                      {step.label}
                    </p>
                    <p className="truncate text-xs text-muted-foreground">
                      {step.desc}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      <div className="grid gap-6 lg:grid-cols-[340px_minmax(0,1fr)] lg:items-start">
        <Card className="border-border/70 bg-card/90 py-0 shadow-sm lg:sticky lg:top-4">
          <CardContent className="space-y-5 p-5">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="font-display text-xl font-semibold tracking-tight">
                  {t("dataset.step_configure")}
                </p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("dataset.step_configure_desc")}
                </p>
              </div>
              <span className="grid size-9 place-items-center rounded-xl bg-secondary text-primary">
                <SlidersHorizontal className="size-4" />
              </span>
            </div>

            <ScoreSlider
              distribution={preview?.scoreDistribution ?? {}}
              value={minScore}
              onChange={setMinScore}
            />

            <div className="space-y-2">
              <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                {t("sessions.filter_model")}
              </span>
              <MultiSelectPill
                label={t("sessions.filter_model")}
                options={modelOptions}
                value={filterModels}
                onChange={setFilterModels}
                emptyText={t("dataset.no_models")}
                className="min-h-9 rounded-lg"
              />
            </div>

            <div className="space-y-2">
              <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                {t("dataset.export_filter_time")}
              </span>
              <TimeRangePicker
                value={timeRange}
                customStart={customStart}
                customEnd={customEnd}
                onChange={(key, cs, ce) => {
                  setTimeRange(key);
                  setCustomStart(cs);
                  setCustomEnd(ce);
                }}
              />
            </div>

            <div className="rounded-2xl border border-border/70 bg-secondary/30 p-3">
              <div className="mb-3 flex items-center justify-between gap-3">
                <span className="text-xs font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                  {t("dataset.export_summary")}
                </span>
                <Badge variant="outline" className="bg-card font-mono text-[10px]">
                  JSONL
                </Badge>
              </div>
              <div className="grid gap-2 text-sm">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">
                    {t("dataset.export_filter_score")}
                  </span>
                  <span className="flex items-center gap-1 font-semibold tabular-nums">
                    ≥ {minScore}
                    <Star className="size-3.5 fill-amber-400 text-amber-400" />
                  </span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">
                    {t("dataset.export_filter_models")}
                  </span>
                  <span className="max-w-[180px] truncate text-right font-semibold" title={modelFilterLabel}>
                    {modelFilterLabel}
                  </span>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <span className="text-muted-foreground">
                    {t("dataset.export_filter_time")}
                  </span>
                  <span className="text-right font-semibold">
                    {timeRangeLabel}
                  </span>
                </div>
              </div>
            </div>

            {hasFilters && (
              <Button
                variant="ghost"
                size="sm"
                className="w-full text-muted-foreground"
                onClick={() => {
                  setMinScore(0);
                  setFilterModels([]);
                  setTimeRange("30d");
                  setCustomStart("");
                  setCustomEnd("");
                }}
              >
                {t("dataset.clear_filters")}
              </Button>
            )}
          </CardContent>
        </Card>

        <div className="space-y-6">
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              label={t("dataset.total_sessions")}
              value={previewLoading && !preview ? "--" : totalSessions.toLocaleString()}
              hint={t("dataset.conversations_label")}
              icon={Database}
            />
            <MetricCard
              label={t("dataset.export_filter_score")}
              value={`≥ ${minScore}`}
              hint={t("dataset.quality_gate", "quality gate")}
              icon={Star}
            />
            <MetricCard
              label={t("dataset.retained", "Retained")}
              value={hasData ? `${retainedPct}%` : "--"}
              hint={t("dataset.retained_hint", "sessions above score gate")}
              icon={Check}
            />
            <MetricCard
              label={t("dataset.top_model", "Top model")}
              value={topModelLabel}
              hint={t("dataset.model_distribution")}
              icon={Hash}
            />
          </div>

          {previewLoading && !preview ? (
            <Card className="border-border/70">
              <CardContent className="space-y-4 p-5 md:p-6">
                <Skeleton className="h-8 w-56" />
                <Skeleton className="h-48 w-full" />
              </CardContent>
            </Card>
          ) : !hasData ? (
            <EmptyDataset title={t("dataset.no_data")} hint={t("dataset.empty_hint")} />
          ) : (
            <>
              <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px] xl:items-stretch">
                <Card className="border-border/70 py-0">
                  <CardContent className="space-y-5 p-5 md:p-6">
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="font-display text-xl font-semibold tracking-tight">
                          {t("dataset.step_review")}
                        </p>
                        <p className="text-sm text-muted-foreground">
                          {t("dataset.step_review_desc")}
                        </p>
                      </div>
                      <Badge variant="secondary" className="w-fit gap-1.5">
                        <BarChart3 className="size-3.5" />
                        {t("dataset.summary_title")}
                      </Badge>
                    </div>

                    <div className="rounded-2xl border border-border/70 bg-secondary/20 p-4">
                      <div className="mb-4 flex items-center justify-between gap-3">
                        <span className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                          {t("dataset.score_distribution")}
                        </span>
                        <span className="text-xs text-muted-foreground tabular-nums">
                          {visibleScoreCount.toLocaleString()} / {totalSessions.toLocaleString()}
                        </span>
                      </div>
                      <div className="grid grid-cols-5 gap-2">
                        {scoreEntries.map(([score, count]) => {
                          const selected = score >= minScore;
                          return (
                            <div key={score} className="space-y-2">
                              <div className="flex h-28 items-end rounded-xl bg-background p-1.5 shadow-inner">
                                <div
                                  className={cn(
                                    "w-full rounded-lg transition-all duration-300",
                                    selected ? "bg-primary" : "bg-muted-foreground/20"
                                  )}
                                  style={{ height: `${Math.max(8, (count / maxScoreCount) * 100)}%` }}
                                />
                              </div>
                              <div className="flex items-center justify-between gap-1 text-xs">
                                <span className={cn("flex items-center gap-1 font-semibold", selected ? "text-foreground" : "text-muted-foreground")}>
                                  <Star className={cn("size-3", selected && "fill-amber-400 text-amber-400")} />
                                  {score}
                                </span>
                                <span className="tabular-nums text-muted-foreground">
                                  {count}
                                </span>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>

                    {modelEntries.length > 0 && (
                      <div className="overflow-hidden rounded-2xl border border-border/70">
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead>{t("dataset.model_distribution")}</TableHead>
                              <TableHead className="w-28 text-right">{t("common.total")}</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {modelEntries.slice(0, 8).map(([model, count]) => (
                              <TableRow key={model}>
                                <TableCell className="min-w-[220px] py-3">
                                  <div className="space-y-1.5">
                                    <div className="truncate font-mono text-xs font-medium" title={model}>
                                      {model}
                                    </div>
                                    <div className="h-1.5 overflow-hidden rounded-full bg-secondary">
                                      <div
                                        className="h-full rounded-full bg-primary/80"
                                        style={{ width: `${Math.max(5, (count / maxModelCount) * 100)}%` }}
                                      />
                                    </div>
                                  </div>
                                </TableCell>
                                <TableCell className="py-3 text-right font-mono text-xs tabular-nums">
                                  {count.toLocaleString()}
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </div>
                    )}
                  </CardContent>
                </Card>

                <Card className="border-primary/20 bg-gradient-to-b from-card to-secondary/30 py-0">
                  <CardContent className="flex h-full min-h-[320px] flex-col justify-between gap-5 p-5 md:p-6">
                    {exporting ? (
                      <div className="flex flex-1 flex-col justify-center gap-6">
                        <div className="flex items-center gap-3">
                          <div className="grid size-12 place-items-center rounded-2xl border border-primary/20 bg-background">
                            <Download className="size-6 animate-pulse text-primary" />
                          </div>
                          <div>
                            <h2 className="font-display text-xl font-semibold">
                              {t("dataset.exporting")}
                            </h2>
                            <p className="text-sm text-muted-foreground tabular-nums">
                              {exportCurrent} / {exportTotal} {t("dataset.conversations_label")}
                            </p>
                          </div>
                        </div>
                        <div className="space-y-2">
                          <div className="h-2.5 w-full overflow-hidden rounded-full bg-background">
                            <div
                              className="h-full rounded-full bg-primary transition-all duration-500 ease-out"
                              style={{ width: `${exportProgress}%` }}
                            />
                          </div>
                          <div className="flex justify-between text-xs text-muted-foreground tabular-nums">
                            <span>{exportProgress}%</span>
                            <span>{exportTotal} total</span>
                          </div>
                        </div>
                        {exportProgress >= 100 && (
                          <Badge variant="secondary" className="w-fit gap-1.5">
                            <Check className="size-3" />
                            {t("dataset.export_complete")}
                          </Badge>
                        )}
                      </div>
                    ) : (
                      <>
                        <div className="space-y-5">
                          <div className="flex items-start justify-between gap-3">
                            <div>
                              <p className="font-display text-xl font-semibold tracking-tight">
                                {t("dataset.step_export")}
                              </p>
                              <p className="mt-1 text-sm text-muted-foreground">
                                {t("dataset.step_export_desc")}
                              </p>
                            </div>
                            <span className="grid size-9 place-items-center rounded-xl bg-primary text-primary-foreground">
                              <FileDown className="size-4" />
                            </span>
                          </div>
                          <div className="grid gap-2 text-sm">
                            <div className="flex items-center justify-between gap-3 rounded-xl border border-border/70 bg-card px-3 py-2.5">
                              <span className="text-muted-foreground">{t("dataset.export_format")}</span>
                              <span className="font-mono font-semibold">{t("dataset.export_format_value")}</span>
                            </div>
                            <div className="flex items-center justify-between gap-3 rounded-xl border border-border/70 bg-card px-3 py-2.5">
                              <span className="text-muted-foreground">{t("dataset.total_sessions")}</span>
                              <span className="font-semibold tabular-nums">{totalSessions.toLocaleString()}</span>
                            </div>
                          </div>
                        </div>
                        <div className="space-y-3">
                          <Button
                            onClick={handleExport}
                            disabled={exporting || totalSessions === 0}
                            size="lg"
                            className="w-full gap-2"
                          >
                            <Download className="size-4" />
                            {t("dataset.export")}
                          </Button>
                          <p className="text-xs leading-5 text-muted-foreground">
                            {t("dataset.export_confirm_total").replace(
                              "{total}",
                              totalSessions.toLocaleString()
                            )}
                          </p>
                        </div>
                      </>
                    )}
                  </CardContent>
                </Card>
              </div>

              <Card className="border-border/70">
                <CardContent className="space-y-4 px-5 md:px-6">
                  <button
                    type="button"
                    onClick={() => setFormatExpanded((v) => !v)}
                    className="flex min-h-11 w-full cursor-pointer items-center justify-between gap-4 rounded-lg text-left text-sm font-medium text-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                    aria-expanded={formatExpanded}
                  >
                    <span className="flex items-center gap-2">
                      <Hash className="size-4 text-muted-foreground" />
                      {t("dataset.format_preview")}
                    </span>
                    <ChevronDown
                      className={cn(
                        "size-4 text-muted-foreground transition-transform duration-200",
                        formatExpanded && "rotate-180"
                      )}
                    />
                  </button>

                  {formatExpanded && (
                    <div className="space-y-3 border-t border-border/60 pt-4">
                      <div className="flex items-center justify-between gap-3">
                        <span className="text-xs text-muted-foreground">
                          {t("dataset.preview_toggle_show")}
                        </span>
                        {formatData && (
                          <div className="flex items-center gap-2">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              disabled={formatOffset === 0 || formatLoading}
                              onClick={handlePrevSession}
                              className="size-8"
                            >
                              <ChevronLeft className="size-3.5" />
                            </Button>
                            <span className="min-w-[64px] text-center text-xs tabular-nums text-muted-foreground">
                              {formatOffset + 1} / {formatData.totalCount?.toLocaleString()}
                            </span>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              disabled={
                                formatLoading ||
                                formatOffset >= (formatData.totalCount ?? 1) - 1
                              }
                              onClick={handleNextSession}
                              className="size-8"
                            >
                              <ChevronRight className="size-3.5" />
                            </Button>
                          </div>
                        )}
                      </div>

                      {formatLoading ? (
                        <div className="space-y-2">
                          <Skeleton className="h-4 w-3/4" />
                          <Skeleton className="h-4 w-1/2" />
                          <Skeleton className="h-4 w-2/3" />
                          <Skeleton className="h-4 w-full" />
                          <Skeleton className="h-4 w-3/4" />
                        </div>
                      ) : formatData?.sharegptJson ? (
                        <div className="space-y-2">
                          <Badge variant="outline" className="gap-1.5 font-mono">
                            <Hash className="size-3" />
                            {formatData.sessionId}
                          </Badge>
                          <pre className="max-h-[440px] min-w-0 overflow-auto rounded-xl border border-border bg-foreground p-4 text-[12px] leading-[1.65] text-background">
                            <code className="block whitespace-pre font-mono">
                              {formattedJSON}
                            </code>
                          </pre>
                        </div>
                      ) : null}
                    </div>
                  )}
                </CardContent>
              </Card>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
