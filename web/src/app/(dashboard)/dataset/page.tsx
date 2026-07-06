"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
      <div>
        <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
          {t("dataset.title")}
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          {t("dataset.subtitle")}
        </p>
      </div>

      <div className="flex items-center gap-2 md:gap-4">
        {steps.map((step, i) => {
          const Icon = step.icon;
          return (
            <div key={step.num} className="flex flex-1 items-center gap-2 md:gap-4">
              <div className="flex items-center gap-2.5 md:gap-3">
                <div
                  className={cn(
                    "flex size-9 shrink-0 items-center justify-center rounded-full border-2 transition-colors",
                    step.active
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-border bg-secondary/50 text-muted-foreground/50"
                  )}
                >
                  <Icon className="size-4" />
                </div>
                <div className="hidden sm:block">
                  <div
                    className={cn(
                      "text-sm font-semibold leading-tight",
                      step.active ? "text-foreground" : "text-muted-foreground/50"
                    )}
                  >
                    {step.label}
                  </div>
                  <div className="text-xs text-muted-foreground/70 leading-tight">
                    {step.desc}
                  </div>
                </div>
              </div>
              {i < steps.length - 1 && (
                <div
                  className={cn(
                    "h-px flex-1 transition-colors",
                    steps[i + 1].active ? "bg-primary/30" : "bg-border"
                  )}
                />
              )}
            </div>
          );
        })}
      </div>

      {/* Step 1: Configure */}
      <Card>
        <CardContent className="p-5 md:p-6 space-y-5">
          <div className="flex items-center gap-2.5">
            <div className="flex size-7 items-center justify-center rounded-full bg-primary/10 text-primary">
              <SlidersHorizontal className="size-3.5" />
            </div>
            <h2 className="font-display text-lg font-semibold">
              {t("dataset.step_configure")}
            </h2>
            <span className="text-sm text-muted-foreground/70 hidden md:inline">
              {t("dataset.step_configure_desc")}
            </span>
          </div>

          <ScoreSlider
            distribution={preview?.scoreDistribution ?? {}}
            value={minScore}
            onChange={setMinScore}
          />

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
          </div>

          {hasFilters && (
            <Button
              variant="ghost"
              size="sm"
              className="text-muted-foreground"
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

      {/* Step 2: Review */}
      {previewLoading && !preview ? (
        <Card>
          <CardContent className="p-6">
            <Skeleton className="h-32 w-full" />
          </CardContent>
        </Card>
      ) : !hasData ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <Database className="mb-3 size-10 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">
              {t("dataset.no_data")}
            </p>
            <p className="mt-1 text-xs text-muted-foreground/60">
              {t("dataset.empty_hint")}
            </p>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardContent className="p-5 md:p-6 space-y-5">
            <div className="flex items-center gap-2.5">
              <div className="flex size-7 items-center justify-center rounded-full bg-primary/10 text-primary">
                <BarChart3 className="size-3.5" />
              </div>
              <h2 className="font-display text-lg font-semibold">
                {t("dataset.step_review")}
              </h2>
              <span className="text-sm text-muted-foreground/70 hidden md:inline">
                {t("dataset.step_review_desc")}
              </span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="space-y-1.5">
                <div className="flex items-center gap-1.5">
                  <Database className="size-3.5 text-muted-foreground" />
                  <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                    {t("dataset.total_sessions")}
                  </span>
                </div>
                <div className="font-display text-2xl md:text-3xl font-bold tabular-nums">
                  {totalSessions.toLocaleString()}
                </div>
                <div className="text-xs text-muted-foreground">
                  {t("dataset.conversations_label")}
                </div>
              </div>

              {preview?.scoreDistribution &&
                Object.keys(preview.scoreDistribution).length > 0 && (
                  <div className="space-y-1.5">
                    <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("dataset.score_distribution")}
                    </span>
                    <div className="flex flex-wrap gap-1.5">
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
                  <div className="space-y-1.5">
                    <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("dataset.model_distribution")}
                    </span>
                    <div className="flex flex-wrap gap-1.5">
                      {Object.entries(preview.modelDistribution)
                        .sort(([, a], [, b]) => b - a)
                        .slice(0, 5)
                        .map(([model, count]) => (
                          <Badge
                            key={model}
                            variant="outline"
                            className="text-[10px] font-mono"
                          >
                            {model}: {count}
                          </Badge>
                        ))}
                    </div>
                  </div>
                )}
            </div>

            <div className="border-t border-border/60 pt-4">
              <button
                type="button"
                onClick={() => setFormatExpanded((v) => !v)}
                className="flex w-full items-center justify-between text-sm font-medium text-foreground transition-colors hover:text-primary"
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
                <div className="mt-4 space-y-3">
                  <div className="flex items-center justify-between">
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
                          className="size-7"
                        >
                          <ChevronLeft className="size-3.5" />
                        </Button>
                        <span className="text-xs text-muted-foreground tabular-nums min-w-[64px] text-center">
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
                          className="size-7"
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
                    <div className="flex items-start gap-3">
                      <div className="flex shrink-0 flex-col items-center gap-1 pt-0.5">
                        <Hash className="size-3.5 text-muted-foreground/50" />
                        <span className="text-[10px] font-mono text-muted-foreground/50">
                          #{formatData.sessionId}
                        </span>
                      </div>
                      <pre className="flex-1 min-w-0 overflow-x-auto rounded-lg border border-border bg-[#1e1e2e] p-4 text-[12px] leading-[1.65] text-[#cdd6f4] max-h-[400px] overflow-y-auto">
                        <code className="block whitespace-pre font-mono">
                          {formattedJSON}
                        </code>
                      </pre>
                    </div>
                  ) : null}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Step 3: Export */}
      {hasData && !previewLoading && (
        <Card>
          <CardContent className="p-5 md:p-6 space-y-5">
            <div className="flex items-center gap-2.5">
              <div className="flex size-7 items-center justify-center rounded-full bg-primary/10 text-primary">
                <FileDown className="size-3.5" />
              </div>
              <h2 className="font-display text-lg font-semibold">
                {t("dataset.step_export")}
              </h2>
              <span className="text-sm text-muted-foreground/70 hidden md:inline">
                {t("dataset.step_export_desc")}
              </span>
            </div>

            {exporting ? (
              <div className="flex flex-col items-center gap-6 py-6">
                <div className="flex size-16 items-center justify-center rounded-2xl border border-border bg-secondary/50">
                  <Download className="size-7 text-primary animate-pulse" />
                </div>

                <div className="text-center space-y-1.5">
                  <h3 className="font-display text-lg font-semibold">
                    {t("dataset.exporting")}
                  </h3>
                  <p className="text-sm text-muted-foreground tabular-nums">
                    {exportCurrent} / {exportTotal}{" "}
                    {t("dataset.conversations_label")}
                  </p>
                </div>

                <div className="w-full max-w-md space-y-2">
                  <div className="h-2 w-full rounded-full bg-secondary overflow-hidden">
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
                  <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
                    <Check className="size-4" />
                    {t("dataset.export_complete")}
                  </div>
                )}
              </div>
            ) : (
              <>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div className="flex items-center justify-between rounded-lg border border-border/60 bg-secondary/30 px-4 py-3">
                    <span className="text-sm text-muted-foreground">
                      {t("dataset.export_filter_score")}
                    </span>
                    <span className="flex items-center gap-1 text-sm font-semibold tabular-nums">
                      ≥ {minScore}
                      <Star className="size-3.5 fill-amber-400 text-amber-400" />
                    </span>
                  </div>

                  <div className="flex items-center justify-between rounded-lg border border-border/60 bg-secondary/30 px-4 py-3">
                    <span className="text-sm text-muted-foreground">
                      {t("dataset.export_filter_models")}
                    </span>
                    <span className="text-sm font-semibold text-right max-w-[60%] truncate">
                      {filterModels.length > 0
                        ? filterModels.join(", ")
                        : t("dataset.export_filter_all_models")}
                    </span>
                  </div>

                  <div className="flex items-center justify-between rounded-lg border border-border/60 bg-secondary/30 px-4 py-3">
                    <span className="text-sm text-muted-foreground">
                      {t("dataset.export_filter_time")}
                    </span>
                    <span className="text-sm font-semibold text-right">
                      {timeRangeLabel}
                    </span>
                  </div>

                  <div className="flex items-center justify-between rounded-lg border border-border/60 bg-secondary/30 px-4 py-3">
                    <span className="text-sm text-muted-foreground">
                      {t("dataset.export_format")}
                    </span>
                    <span className="text-sm font-semibold font-mono">
                      {t("dataset.export_format_value")}
                    </span>
                  </div>
                </div>

                <div className="flex items-center justify-between rounded-lg border border-primary/20 bg-primary/5 px-4 py-4">
                  <div>
                    <div className="text-xs font-medium uppercase tracking-[0.08em] text-primary/80">
                      {t("dataset.total_sessions")}
                    </div>
                    <div className="font-display text-2xl font-bold tabular-nums text-foreground">
                      {totalSessions.toLocaleString()}
                    </div>
                  </div>
                  <Button
                    onClick={handleExport}
                    disabled={exporting || totalSessions === 0}
                    size="lg"
                    className="gap-2 px-8"
                  >
                    <Download className="size-4" />
                    {t("dataset.export")}
                  </Button>
                </div>

                <p className="text-xs text-muted-foreground/70">
                  {t("dataset.export_confirm_total").replace(
                    "{total}",
                    totalSessions.toLocaleString()
                  )}
                </p>
              </>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
