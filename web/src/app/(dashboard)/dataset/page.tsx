"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import {
  ChevronDown,
  Database,
  Download,
  Eye,
  FileJson,
  Filter,
  Gauge,
  Loader2,
  RotateCcw,
  SlidersHorizontal,
  ArrowRight,
} from "lucide-react";

import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import type { DatasetFormatPreviewRsp, DatasetPreviewRsp } from "@/lib/types";
import { computeRange, type TimeRangeKey } from "@/lib/time-range";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
import { Skeleton } from "@/components/ui/skeleton";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import { Separator } from "@/components/ui/separator";

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value);
}

function formatTimeRange(startTime: string, endTime: string): string {
  const start = new Date(startTime).toLocaleString();
  const end = new Date(endTime).toLocaleString();
  return `${start} - ${end}`;
}

function replaceVars(text: string, vars: Record<string, string>): string {
  return Object.entries(vars).reduce(
    (acc, [key, value]) => acc.replaceAll(`{${key}}`, value),
    text,
  );
}

function StepCard({
  index,
  title,
  description,
  children,
}: {
  index: number;
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-start gap-3">
          <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
            {index}
          </div>
          <div>
            <CardTitle>{title}</CardTitle>
            <CardDescription className="mt-1">{description}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <Separator className="mx-4" />
      <CardContent>{children}</CardContent>
    </Card>
  );
}

function MetricCard({
  label,
  value,
  hint,
  loading,
}: {
  label: string;
  value: string;
  hint?: string;
  loading: boolean;
}) {
  return (
    <div className="rounded-xl border border-border bg-muted/30 p-4">
      <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      {loading ? (
        <Skeleton className="mt-3 h-8 w-24" />
      ) : (
        <div className="mt-2 font-display text-3xl font-semibold text-foreground">
          {value}
        </div>
      )}
      {hint && <div className="mt-1 text-xs text-muted-foreground">{hint}</div>}
    </div>
  );
}

function StatPill({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-border bg-card px-3.5 py-2.5 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="ml-auto font-mono font-medium tabular-nums text-foreground">{value}</span>
    </div>
  );
}

function DistributionList({
  title,
  items,
  emptyText,
}: {
  title: string;
  items: { label: string; value: number }[];
  emptyText: string;
}) {
  const max = Math.max(1, ...items.map((item) => item.value));

  return (
    <div className="rounded-xl border border-border bg-muted/20 p-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h3 className="text-sm font-medium text-foreground">{title}</h3>
        <Badge variant="secondary" className="font-mono tabular-nums">{items.length}</Badge>
      </div>
      <div className="space-y-3">
        {items.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border px-3 py-8 text-center text-sm text-muted-foreground">
            {emptyText}
          </div>
        ) : (
          items.map((item) => (
            <div key={item.label} className="space-y-1.5">
              <div className="flex items-center justify-between gap-3 text-xs">
                <span className="max-w-[24ch] truncate text-foreground" title={item.label}>{item.label}</span>
                <span className="font-mono tabular-nums text-muted-foreground">{formatNumber(item.value)}</span>
              </div>
              <div className="h-2.5 overflow-hidden rounded-full bg-background">
                <div
                  className="h-full rounded-full bg-primary/70 transition-all duration-500"
                  style={{ width: `${Math.max(6, (item.value / max) * 100)}%` }}
                />
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

export default function DatasetPage() {
  const t = useT();
  const [minScore, setMinScore] = usePersistentState("dashboard.dataset.minScore", 3);
  const [selectedModels, setSelectedModels] = usePersistentState<string[]>("dashboard.dataset.models", []);
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>("dashboard.dataset.timeRange", "30d");
  const [customStart, setCustomStart] = usePersistentState("dashboard.dataset.customStart", "");
  const [customEnd, setCustomEnd] = usePersistentState("dashboard.dataset.customEnd", "");
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const [preview, setPreview] = useState<DatasetPreviewRsp | null>(null);
  const [formatPreview, setFormatPreview] = useState<DatasetFormatPreviewRsp | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(true);
  const [loadingFormat, setLoadingFormat] = useState(false);
  const [showFormatPreview, setShowFormatPreview] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [exportProgress, setExportProgress] = useState(0);

  const buildParams = useCallback(() => {
    const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
    return {
      minScore,
      models: selectedModels,
      startTime,
      endTime,
    };
  }, [customEnd, customStart, minScore, selectedModels, timeRange]);

  const loadPreview = useCallback(async () => {
    setLoadingPreview(true);
    try {
      const params = buildParams();
      const rsp = await api.previewDataset(params);
      if (rsp.error) {
        toast.error(rsp.error.message ?? t("dataset.preview_error"));
        return;
      }
      setPreview(rsp);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("dataset.preview_error"));
    } finally {
      setLoadingPreview(false);
    }
  }, [buildParams, t]);

  const loadFormatPreview = useCallback(async () => {
    if (!showFormatPreview) return;
    setLoadingFormat(true);
    try {
      const rsp = await api.previewDatasetFormat({ ...buildParams(), offset: 0 });
      if (rsp.error) {
        toast.error(rsp.error.message ?? t("dataset.preview_error"));
        return;
      }
      setFormatPreview(rsp);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("dataset.preview_error"));
    } finally {
      setLoadingFormat(false);
    }
  }, [buildParams, showFormatPreview, t]);

  const loadModelOptions = useCallback(async () => {
    try {
      const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.listSessionOptions({ field: "model", startTime, endTime });
      if (!rsp.error) setModelOptions(rsp.items ?? []);
    } catch {
      setModelOptions([]);
    }
  }, [customEnd, customStart, timeRange]);

  /* eslint-disable react-hooks/set-state-in-effect -- Dataset filters refetch server previews. */
  useEffect(() => {
    loadPreview();
  }, [loadPreview]);

  useEffect(() => {
    loadFormatPreview();
  }, [loadFormatPreview]);

  useEffect(() => {
    loadModelOptions();
  }, [loadModelOptions]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleClearFilters = () => {
    setMinScore(0);
    setSelectedModels([]);
    setTimeRange("30d");
    setCustomStart("");
    setCustomEnd("");
  };

  const handleExport = async () => {
    if (exporting || (preview?.totalSessions ?? 0) === 0) return;

    setExporting(true);
    const lines: string[] = [];
    setExportProgress(0);
    let total = preview?.totalSessions ?? 0;
    let exportError = "";

    try {
      await api.exportDatasetStream(buildParams(), (event) => {
        if (event.event === "start") {
          total = event.data.totalSessions;
        } else if (event.event === "data") {
          lines.push(event.data.json);
          setExportProgress(event.data.progress);
        } else if (event.event === "done") {
          total = event.data.totalSessions;
          setExportProgress(100);
        } else if (event.event === "error") {
          exportError = event.data.message;
        }
      });

      if (exportError) throw new Error(exportError);

      const blob = new Blob([`${lines.join("\n")}\n`], { type: "application/x-ndjson" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `aris-dataset-${new Date().toISOString().slice(0, 10)}.jsonl`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);

      toast.success(`${t("dataset.export_success")} · ${formatNumber(total)}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("dataset.export_error"));
    } finally {
      setExporting(false);
    }
  };

  const params = buildParams();
  const totalSessions = preview?.totalSessions ?? 0;
  const activeFilters = (minScore > 0 ? 1 : 0) + selectedModels.length + (timeRange !== "30d" ? 1 : 0);
  const scoreItems = Object.entries(preview?.scoreDistribution ?? {})
    .map(([score, value]) => ({
      label: replaceVars(t("dataset.score_label"), { score }),
      value,
    }))
    .sort((a, b) => Number(b.label.replace(/\D/g, "")) - Number(a.label.replace(/\D/g, "")));
  const modelItems = Object.entries(preview?.modelDistribution ?? {})
    .map(([label, value]) => ({ label, value }))
    .sort((a, b) => b.value - a.value);
  const topModel = modelItems[0]?.label ?? "-";

  return (
    <div className="space-y-8">
      {/* Page Header — consistent with other dashboard pages */}
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="max-w-2xl">
          
          <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
            {t("dataset.title")}
          </h1>
          <p className="mt-2 max-w-xl text-sm leading-relaxed text-muted-foreground md:text-base">
            {t("dataset.subtitle")}
          </p>
        </div>

        {/* Quick stats */}
        <div className="grid grid-cols-2 gap-3 md:min-w-64">
          <div className="rounded-xl border border-border bg-muted/20 p-3.5">
            <div className="text-xs text-muted-foreground">{t("dataset.total_sessions")}</div>
            <div className="mt-1 font-mono text-2xl font-semibold tabular-nums text-foreground">
              {loadingPreview ? "..." : formatNumber(totalSessions)}
            </div>
          </div>
          <div className="rounded-xl border border-border bg-muted/20 p-3.5">
            <div className="text-xs text-muted-foreground">{t("dataset.active_filters")}</div>
            <div className="mt-1 font-mono text-2xl font-semibold tabular-nums text-foreground">
              {activeFilters}
            </div>
          </div>
        </div>
      </div>

      {/* Step flow indicator */}
      <div className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
        <span className="flex items-center gap-1.5 text-primary">
          <span className="flex size-5 items-center justify-center rounded-full bg-primary/10 text-[11px] font-semibold">1</span>
          {t("dataset.step_configure")}
        </span>
        <ArrowRight className="size-3" />
        <span className="flex items-center gap-1.5">
          <span className="flex size-5 items-center justify-center rounded-full bg-muted text-[11px]">2</span>
          {t("dataset.step_review")}
        </span>
        <ArrowRight className="size-3" />
        <span className="flex items-center gap-1.5">
          <span className="flex size-5 items-center justify-center rounded-full bg-muted text-[11px]">3</span>
          {t("dataset.step_export")}
        </span>
      </div>

      {/* Step 1: Configure */}
      <StepCard index={1} title={t("dataset.step_configure")} description={t("dataset.step_configure_desc")}>
        <div className="space-y-5 pt-2">
          <div className="grid gap-4 lg:grid-cols-[1fr_1.3fr]">
            {/* Quality gate */}
            <div className="rounded-xl border border-border bg-muted/20 p-5">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2 text-sm font-medium text-foreground">
                    <Gauge className="size-4 text-primary" />
                    {t("dataset.quality_gate")}
                  </div>
                  <div className="mt-1 text-xs text-muted-foreground">{t("dataset.min_score")}</div>
                </div>
                <div className="font-mono text-3xl font-semibold tabular-nums text-foreground">
                  {minScore}
                </div>
              </div>
              <input
                type="range"
                min="0"
                max="5"
                step="1"
                value={minScore}
                onChange={(event) => setMinScore(Number(event.target.value))}
                className="mt-5 h-2 w-full cursor-pointer accent-primary"
                aria-label={t("dataset.min_score")}
              />
              <div className="mt-2 flex justify-between font-mono text-xs text-muted-foreground">
                {[0, 1, 2, 3, 4, 5].map((score) => <span key={score}>{score}</span>)}
              </div>
            </div>

            {/* Filters */}
            <div className="flex flex-col gap-4 rounded-xl border border-border bg-muted/20 p-5">
              <div className="flex flex-wrap items-center gap-2">
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
                <MultiSelectPill
                  label={t("sessions.filter_model")}
                  options={modelOptions}
                  value={selectedModels}
                  onChange={setSelectedModels}
                  searchable
                  emptyText={t("dataset.no_models")}
                />
                {activeFilters > 0 && (
                  <Button variant="ghost" size="sm" onClick={handleClearFilters} className="gap-1.5 text-muted-foreground hover:text-foreground">
                    <RotateCcw className="size-3.5" />
                    {t("dataset.clear_filters")}
                  </Button>
                )}
              </div>

              <div className="flex flex-wrap gap-2">
                <Badge variant="outline" className="gap-1.5 rounded-lg px-3 py-1.5">
                  <Filter className="size-3 text-primary" />
                  {t("dataset.export_filter_score")}: {minScore}
                </Badge>
                <Badge variant="outline" className="gap-1.5 rounded-lg px-3 py-1.5">
                  <SlidersHorizontal className="size-3 text-primary" />
                  {selectedModels.length > 0 ? `${selectedModels.length} ${t("dataset.export_filter_models")}` : t("dataset.export_filter_all_models")}
                </Badge>
                <Badge variant="outline" className="gap-1.5 rounded-lg px-3 py-1.5">
                  <FileJson className="size-3 text-primary" />
                  {t("dataset.export_format_value")}
                </Badge>
              </div>
            </div>
          </div>
        </div>
      </StepCard>

      {/* Step 2: Review */}
      <StepCard index={2} title={t("dataset.step_review")} description={t("dataset.step_review_desc")}>
        <div className="space-y-5 pt-2">
          {/* Metric cards */}
          <div className="grid gap-3 md:grid-cols-3">
            <MetricCard label={t("dataset.total_sessions")} value={formatNumber(totalSessions)} loading={loadingPreview} />
            <MetricCard label={t("dataset.top_model")} value={topModel} loading={loadingPreview} />
            <MetricCard label={t("dataset.export_format")} value={t("dataset.export_format_value")} loading={loadingPreview} />
          </div>

          {totalSessions === 0 && !loadingPreview ? (
            <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border bg-muted/10 px-4 py-12 text-center">
              <Database className="mb-3 size-8 text-muted-foreground/30" />
              <div className="font-medium text-foreground">{t("dataset.no_data")}</div>
              <div className="mt-1 text-sm text-muted-foreground">{t("dataset.empty_hint")}</div>
            </div>
          ) : (
            <div className="grid gap-4 lg:grid-cols-2">
              <DistributionList title={t("dataset.score_distribution")} items={scoreItems} emptyText={t("dataset.no_data")} />
              <DistributionList title={t("dataset.model_distribution")} items={modelItems} emptyText={t("dataset.no_models")} />
            </div>
          )}

          {/* Format preview */}
          <div className="overflow-hidden rounded-xl border border-border">
            <button
              type="button"
              className="flex min-h-12 w-full items-center justify-between gap-3 bg-muted/20 px-5 py-3.5 text-left text-sm font-medium text-foreground transition-colors hover:bg-muted/40"
              onClick={() => setShowFormatPreview((value) => !value)}
            >
              <span className="flex items-center gap-2.5">
                <Eye className="size-4 text-primary" />
                {showFormatPreview ? t("dataset.preview_toggle_hide") : t("dataset.preview_toggle_show")}
              </span>
              <ChevronDown className={cn("size-4 text-muted-foreground transition-transform", showFormatPreview && "rotate-180")} />
            </button>
            {showFormatPreview && (
              <div className="border-t border-border px-5 py-4">
                {loadingFormat ? (
                  <Skeleton className="h-48 w-full rounded-lg" />
                ) : formatPreview?.sharegptJson ? (
                  <div className="space-y-3">
                    <div className="text-xs text-muted-foreground">
                      {replaceVars(t("dataset.sample_of"), {
                        current: String((formatPreview.offset ?? 0) + 1),
                        total: String(formatPreview.totalCount ?? 0),
                      })}
                    </div>
                    <pre className="max-h-96 overflow-auto rounded-lg border border-border bg-muted/10 p-4 font-mono text-xs leading-6 text-foreground/85">
                      {formatPreview.sharegptJson}
                    </pre>
                  </div>
                ) : (
                  <div className="rounded-lg border border-dashed border-border px-3 py-8 text-center text-sm text-muted-foreground">
                    {t("dataset.no_data")}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </StepCard>

      {/* Step 3: Export */}
      <Card className="border-primary/20 bg-primary/[0.03]">
        <CardHeader>
          <div className="flex items-start gap-3">
            <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
              3
            </div>
            <div>
              <CardTitle>{t("dataset.step_export")}</CardTitle>
              <CardDescription className="mt-1">{t("dataset.step_export_desc")}</CardDescription>
            </div>
          </div>
        </CardHeader>
        <Separator className="mx-4" />
        <CardContent>
          <div className="grid gap-5 pt-2 lg:grid-cols-[1fr_auto] lg:items-end">
            {/* Export summary */}
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <StatPill label={t("dataset.export_filter_score")} value={String(minScore)} />
              <StatPill
                label={t("dataset.export_filter_models")}
                value={selectedModels.join(", ") || t("dataset.export_filter_all_models")}
              />
              <StatPill
                label={t("dataset.export_filter_time")}
                value={formatTimeRange(params.startTime, params.endTime)}
              />
              <div className="flex items-center gap-2 rounded-lg border border-border bg-card px-3.5 py-2.5 text-sm">
                <FileJson className="size-4 text-primary" />
                <span className="font-mono text-xs font-medium text-foreground">{t("dataset.export_format_value")}</span>
              </div>
            </div>

            {/* Export action */}
            <div className="min-w-64 space-y-3 rounded-xl border border-primary/15 bg-card p-4">
              <CardDescription>
                {replaceVars(t("dataset.export_confirm_total"), { total: formatNumber(totalSessions) })}
              </CardDescription>
              {exporting && (
                <div className="h-2.5 overflow-hidden rounded-full bg-muted">
                  <div className="h-full rounded-full bg-primary transition-[width] duration-300 ease-out" style={{ width: `${exportProgress}%` }} />
                </div>
              )}
              <Button
                className="h-11 w-full gap-2"
                size="lg"
                onClick={handleExport}
                disabled={exporting || loadingPreview || totalSessions === 0}
              >
                {exporting ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <Download className="size-4" />
                )}
                {exporting ? t("dataset.exporting") : t("dataset.export")}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
