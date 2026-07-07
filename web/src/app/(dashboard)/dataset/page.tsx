"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
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
      <Card className="relative overflow-hidden border-primary/10 bg-gradient-to-br from-card via-card to-secondary/50 py-0">
        <CardContent className="relative p-5 md:p-7">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div className="max-w-2xl space-y-3">
              <Badge variant="secondary" className="w-fit gap-1.5">
                <Database className="size-3" />
                {t("dataset.export_format_value")}
              </Badge>
              <div>
                <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
                  {t("dataset.title")}
                </h1>
                <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground md:text-base">
                  {t("dataset.subtitle")}
                </p>
              </div>
            </div>

            <div className="grid grid-cols-3 gap-2 rounded-xl border border-border/60 bg-background/60 p-2 shadow-sm backdrop-blur sm:min-w-[360px]">
              {steps.map((step) => {
                const Icon = step.icon;
                return (
                  <div
                    key={step.num}
                    className={cn(
                      "rounded-lg border px-3 py-2 transition-colors",
                      step.active
                        ? "border-primary/20 bg-primary/10 text-foreground"
                        : "border-transparent bg-secondary/40 text-muted-foreground/60"
                    )}
                  >
                    <div className="mb-1 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.08em]">
                      <Icon className="size-3" />
                      {step.num}
                    </div>
                    <div className="truncate text-sm font-medium">
                      {step.label}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-6 lg:grid-cols-[360px_minmax(0,1fr)] lg:items-start">
        <Card className="lg:sticky lg:top-4">
          <CardHeader className="px-5 md:px-6">
            <div className="flex items-center gap-2.5">
              <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <SlidersHorizontal className="size-4" />
              </div>
              <div>
                <CardTitle className="font-display">
                  {t("dataset.step_configure")}
                </CardTitle>
                <CardDescription>
                  {t("dataset.step_configure_desc")}
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-5 px-5 md:px-6">
            <ScoreSlider
              distribution={preview?.scoreDistribution ?? {}}
              value={minScore}
              onChange={setMinScore}
            />

            <div className="space-y-2">
              <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
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
              <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
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

            <div className="space-y-2 rounded-xl border border-border/60 bg-secondary/30 p-3">
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-muted-foreground">
                  {t("dataset.export_filter_score")}
                </span>
                <span className="flex items-center gap-1 font-semibold tabular-nums">
                  ≥ {minScore}
                  <Star className="size-3.5 fill-amber-400 text-amber-400" />
                </span>
              </div>
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-muted-foreground">
                  {t("dataset.export_filter_models")}
                </span>
                <span className="max-w-[180px] truncate text-right font-semibold">
                  {modelFilterLabel}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-muted-foreground">
                  {t("dataset.export_filter_time")}
                </span>
                <span className="text-right font-semibold">
                  {timeRangeLabel}
                </span>
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
          {previewLoading && !preview ? (
            <Card>
              <CardContent className="space-y-4 p-5 md:p-6">
                <Skeleton className="h-9 w-48" />
                <Skeleton className="h-40 w-full" />
              </CardContent>
            </Card>
          ) : !hasData ? (
            <Card className="min-h-[420px] justify-center">
              <CardContent className="flex flex-col items-center justify-center py-16 text-center">
                <div className="mb-4 flex size-14 items-center justify-center rounded-2xl border border-border bg-secondary/50">
                  <Database className="size-7 text-muted-foreground/50" />
                </div>
                <p className="font-display text-lg font-semibold text-foreground">
                  {t("dataset.no_data")}
                </p>
                <p className="mt-2 max-w-sm text-sm leading-6 text-muted-foreground">
                  {t("dataset.empty_hint")}
                </p>
              </CardContent>
            </Card>
          ) : (
            <>
              <Card className="py-0">
                <CardContent className="p-0">
                  <div className="grid lg:grid-cols-[minmax(0,1fr)_280px]">
                    <div className="space-y-5 p-5 md:p-6">
                      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <div className="flex items-center gap-2.5">
                          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
                            <BarChart3 className="size-4" />
                          </div>
                          <div>
                            <h2 className="font-display text-lg font-semibold">
                              {t("dataset.step_review")}
                            </h2>
                            <p className="text-sm text-muted-foreground">
                              {t("dataset.step_review_desc")}
                            </p>
                          </div>
                        </div>
                        {previewLoading && (
                          <Badge variant="secondary">
                            {t("dataset.preview")}
                          </Badge>
                        )}
                      </div>

                      <div className="rounded-2xl border border-border/60 bg-secondary/20 p-4">
                        <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                          <Database className="size-3.5" />
                          {t("dataset.total_sessions")}
                        </div>
                        <div className="mt-2 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
                          <div className="font-display text-4xl font-bold tabular-nums text-foreground md:text-5xl">
                            {totalSessions.toLocaleString()}
                          </div>
                          <div className="text-sm text-muted-foreground">
                            {t("dataset.conversations_label")}
                          </div>
                        </div>
                      </div>

                      <div className="grid gap-4 md:grid-cols-2">
                        {preview?.scoreDistribution &&
                          Object.keys(preview.scoreDistribution).length > 0 && (
                            <div className="rounded-xl border border-border/60 p-4">
                              <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                                {t("dataset.score_distribution")}
                              </span>
                              <div className="mt-3 flex flex-wrap gap-1.5">
                                {Object.entries(preview.scoreDistribution)
                                  .sort(([a], [b]) => Number(a) - Number(b))
                                  .map(([score, count]) => (
                                    <Badge
                                      key={score}
                                      variant={
                                        Number(score) >= minScore
                                          ? "default"
                                          : "secondary"
                                      }
                                      className="gap-1"
                                    >
                                      <Star className="size-3" />
                                      {score}: {count}
                                    </Badge>
                                  ))}
                              </div>
                            </div>
                          )}

                        {preview?.modelDistribution &&
                          Object.keys(preview.modelDistribution).length > 0 && (
                            <div className="rounded-xl border border-border/60 p-4">
                              <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                                {t("dataset.model_distribution")}
                              </span>
                              <div className="mt-3 flex flex-wrap gap-1.5">
                                {Object.entries(preview.modelDistribution)
                                  .sort(([, a], [, b]) => b - a)
                                  .slice(0, 5)
                                  .map(([model, count]) => (
                                    <Badge
                                      key={model}
                                      variant="outline"
                                      className="max-w-full font-mono text-[10px]"
                                      title={`${model}: ${count}`}
                                    >
                                      <span className="truncate">
                                        {model}: {count}
                                      </span>
                                    </Badge>
                                  ))}
                              </div>
                            </div>
                          )}
                      </div>
                    </div>

                    <div className="border-t border-border bg-secondary/25 p-5 lg:border-l lg:border-t-0 md:p-6">
                      {exporting ? (
                        <div className="flex h-full min-h-[260px] flex-col justify-center gap-6">
                          <div className="flex items-center gap-3">
                            <div className="flex size-12 items-center justify-center rounded-2xl border border-border bg-card">
                              <Download className="size-6 animate-pulse text-primary" />
                            </div>
                            <div>
                              <h3 className="font-display text-lg font-semibold">
                                {t("dataset.exporting")}
                              </h3>
                              <p className="text-sm text-muted-foreground tabular-nums">
                                {exportCurrent} / {exportTotal}{" "}
                                {t("dataset.conversations_label")}
                              </p>
                            </div>
                          </div>

                          <div className="space-y-2">
                            <div className="h-2 w-full overflow-hidden rounded-full bg-background">
                              <div
                                className="h-full rounded-full bg-primary transition-all duration-500 ease-out"
                                style={{ width: `${exportProgress}%` }}
                              />
                            </div>
                            <div className="flex justify-between text-[11px] text-muted-foreground tabular-nums">
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
                        <div className="flex h-full min-h-[260px] flex-col justify-between gap-5">
                          <div className="space-y-4">
                            <div className="flex items-center gap-2.5">
                              <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
                                <FileDown className="size-4" />
                              </div>
                              <div>
                                <h2 className="font-display text-lg font-semibold">
                                  {t("dataset.step_export")}
                                </h2>
                                <p className="text-sm text-muted-foreground">
                                  {t("dataset.step_export_desc")}
                                </p>
                              </div>
                            </div>

                            <div className="grid gap-2 text-sm">
                              <div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-card px-3 py-2.5">
                                <span className="text-muted-foreground">
                                  {t("dataset.export_format")}
                                </span>
                                <span className="font-mono font-semibold">
                                  {t("dataset.export_format_value")}
                                </span>
                              </div>
                              <div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-card px-3 py-2.5">
                                <span className="text-muted-foreground">
                                  {t("dataset.total_sessions")}
                                </span>
                                <span className="font-semibold tabular-nums">
                                  {totalSessions.toLocaleString()}
                                </span>
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
                            <p className="text-xs leading-5 text-muted-foreground/80">
                              {t("dataset.export_confirm_total").replace(
                                "{total}",
                                totalSessions.toLocaleString()
                              )}
                            </p>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
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
                              {formatOffset + 1} /{" "}
                              {formatData.totalCount?.toLocaleString()}
                            </span>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              disabled={
                                formatLoading ||
                                formatOffset >=
                                  (formatData.totalCount ?? 1) - 1
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
