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
  AlertTriangle,
  Loader2,
  Check,
  Hash,
  Star,
} from "lucide-react";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
import { ScoreSlider } from "@/components/dataset/score-slider";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

function useDebouncedCallback(cb: () => void, delay: number) {
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const cbRef = useRef(cb);
  cbRef.current = cb;

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

  useEffect(() => {
    if ((preview?.totalSessions ?? 0) > 0) {
      fetchFormatPreview(formatOffset);
    } else {
      setFormatData(null);
    }
  }, [preview?.totalSessions, formatOffset, fetchFormatPreview]);

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
    const total = preview?.totalSessions ?? 0;
    setExporting(true);
    setExportProgress(0);
    setExportCurrent(0);
    setExportTotal(total);

    try {
      setExportCurrent(Math.floor(total * 0.3));
      setExportProgress(30);

      const blob = await api.exportDataset(params);

      setExportCurrent(Math.floor(total * 0.8));
      setExportProgress(80);

      const fmtMinScore = params.minScore ?? 1;
      const fmtModels = params.models?.length ?? 0;
      const dateStr = new Date().toISOString().slice(0, 10).replace(/-/g, "");
      const filename = `dataset_${dateStr}_minScore${fmtMinScore}_models${fmtModels}_${total}sessions.jsonl`;

      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      setExportCurrent(total);
      setExportProgress(100);
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
  }, [buildExportParams, preview?.totalSessions, t]);

  const formattedJSON = useMemo(() => {
    if (!formatData?.sharegptJson) return "";
    try {
      return JSON.stringify(JSON.parse(formatData.sharegptJson), null, 2);
    } catch {
      return formatData.sharegptJson;
    }
  }, [formatData?.sharegptJson]);

  const hasFilters =
    minScore > 1 || filterModels.length > 0 || customStart || customEnd;
  const totalSessions = preview?.totalSessions ?? 0;

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

      <div className="flex flex-col lg:flex-row lg:gap-6 gap-4">
        {/* ─── Left sidebar: Filters ─── */}
        <div className="lg:w-[320px] lg:shrink-0">
          <Card>
            <CardContent className="p-5 space-y-5">
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
                />
              </div>

              <div className="space-y-2">
                <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                  Time Range
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

              {hasFilters && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full text-muted-foreground"
                  onClick={() => {
                    setMinScore(1);
                    setFilterModels([]);
                    setTimeRange("30d");
                    setCustomStart("");
                    setCustomEnd("");
                  }}
                >
                  Clear all filters
                </Button>
              )}
            </CardContent>
          </Card>

          <Card className="mt-4">
            <CardContent className="p-5">
              {previewLoading ? (
                <div className="space-y-3">
                  <Skeleton className="h-4 w-24" />
                  <Skeleton className="h-8 w-16" />
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                    <Database className="size-4 text-muted-foreground" />
                    <span className="text-xs font-medium text-muted-foreground">
                      {t("dataset.total_sessions")}
                    </span>
                  </div>
                  <div className="font-display text-3xl font-bold tabular-nums">
                    {totalSessions.toLocaleString()}
                  </div>

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
              )}
            </CardContent>
          </Card>
        </div>

        {/* ─── Right panel: Stats → Preview → Export ─── */}
        <div className="flex-1 min-w-0 space-y-4">
          {exporting ? (
            <Card>
              <CardContent className="p-8">
                <div className="flex flex-col items-center gap-6">
                  <div className="flex size-16 items-center justify-center rounded-2xl border border-border bg-secondary/50">
                    <Download className="size-7 text-primary animate-pulse" />
                  </div>

                  <div className="text-center space-y-1.5">
                    <h2 className="font-display text-lg font-semibold">
                      Exporting Dataset
                    </h2>
                    <p className="text-sm text-muted-foreground">
                      {exportCurrent} / {exportTotal} sessions
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
                      Export complete
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          ) : (
            <>
              {/* Stats summary */}
              {preview && !previewLoading && totalSessions > 0 && (
                <Card>
                  <CardContent className="p-5">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-sm font-semibold text-foreground">
                        Dataset Summary
                      </h3>
                      <Badge variant="secondary" className="font-mono">
                        {totalSessions.toLocaleString()} conversations
                      </Badge>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      {Object.keys(preview.scoreDistribution).length > 0 && (
                        <div className="space-y-1.5">
                          <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                            {t("dataset.score_distribution")}
                          </span>
                          <div className="flex flex-wrap gap-2">
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

                      {Object.keys(preview.modelDistribution).length > 0 && (
                        <div className="space-y-1.5">
                          <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                            {t("dataset.model_distribution")}
                          </span>
                          <div className="flex flex-wrap gap-2">
                            {Object.entries(preview.modelDistribution)
                              .sort(([, a], [, b]) => b - a)
                              .slice(0, 8)
                              .map(([model, count]) => (
                                <Badge
                                  key={model}
                                  variant="outline"
                                  className="text-xs"
                                >
                                  {model}: {count}
                                </Badge>
                              ))}
                          </div>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              )}

              {previewLoading && (
                <Card>
                  <CardContent className="p-6">
                    <Skeleton className="h-24 w-full" />
                  </CardContent>
                </Card>
              )}

              {preview && !previewLoading && totalSessions === 0 && (
                <Card>
                  <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                    <Database className="mb-3 size-10 text-muted-foreground/30" />
                    <p className="text-sm text-muted-foreground">
                      {t("dataset.no_data")}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground/60">
                      Try lowering the minimum score or adjusting the time range
                    </p>
                  </CardContent>
                </Card>
              )}

              {/* Format preview */}
              {preview && !previewLoading && totalSessions > 0 && (
                <Card>
                  <CardContent className="p-5">
                    <div className="flex items-center justify-between mb-4">
                      <h3 className="text-sm font-semibold text-foreground">
                        Format Preview
                      </h3>
                      {formatData && (
                        <div className="flex items-center gap-2">
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={formatOffset === 0}
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
                        <pre className="flex-1 min-w-0 overflow-x-auto rounded-lg border border-border bg-[#1e1e2e] p-4 text-[12px] leading-[1.65] text-[#cdd6f4]">
                          <code className="block whitespace-pre font-mono">
                            {formattedJSON}
                          </code>
                        </pre>
                      </div>
                    ) : null}
                  </CardContent>
                </Card>
              )}

              {/* Export action */}
              {preview &&
                !previewLoading &&
                totalSessions > 0 &&
                formatData?.sharegptJson && (
                  <div className="flex items-center justify-between">
                    <div className="text-xs text-muted-foreground">
                      This will export{" "}
                      <span className="font-semibold text-foreground">
                        {totalSessions.toLocaleString()}
                      </span>{" "}
                      conversations as ShareGPT JSONL for SFT fine-tuning.
                    </div>
                    <Button
                      onClick={handleExport}
                      disabled={exporting || totalSessions === 0}
                      size="lg"
                      className="gap-2"
                    >
                      <Download className="size-4" />
                      Export JSONL
                    </Button>
                  </div>
                )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
