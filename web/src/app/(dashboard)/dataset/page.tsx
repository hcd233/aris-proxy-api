"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import {
  Check,
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
} from "@/components/ui/card";
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
import { Skeleton } from "@/components/ui/skeleton";
import { TimeRangePicker } from "@/components/ui/time-range-picker";

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

function StepBadge({ index, title, description }: { index: number; title: string; description: string }) {
  return (
    <div className="flex min-w-0 items-center gap-3 rounded-xl border border-white/10 bg-white/8 px-3 py-3 text-white shadow-sm backdrop-blur-sm">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary text-sm font-semibold text-primary-foreground">
        {index}
      </div>
      <div className="min-w-0">
        <div className="truncate text-sm font-medium">{title}</div>
        <div className="truncate text-xs text-white/62">{description}</div>
      </div>
    </div>
  );
}

function SectionHeading({
  number,
  title,
  description,
}: {
  number: number;
  title: string;
  description: string;
}) {
  return (
    <div className="flex items-start gap-3">
      <div className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/12 text-sm font-semibold text-primary">
        {number}
      </div>
      <div>
        <h2 className="font-display text-lg font-semibold tracking-tight text-foreground">
          {title}
        </h2>
        <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      </div>
    </div>
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
    <div className="rounded-xl border border-border bg-card/70 p-4">
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
    <div className="rounded-xl border border-border bg-muted/20 p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 className="text-sm font-medium text-foreground">{title}</h3>
        <Badge variant="secondary">{items.length}</Badge>
      </div>
      <div className="space-y-2">
        {items.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border px-3 py-6 text-center text-sm text-muted-foreground">
            {emptyText}
          </div>
        ) : (
          items.map((item) => (
            <div key={item.label} className="space-y-1.5">
              <div className="flex items-center justify-between gap-3 text-xs">
                <span className="max-w-[22ch] truncate text-foreground" title={item.label}>{item.label}</span>
                <span className="font-mono tabular-nums text-muted-foreground">{formatNumber(item.value)}</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-background">
                <div
                  className="h-full rounded-full bg-primary"
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
    setExportProgress(0);
    const lines: string[] = [];
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
    <div className="space-y-6">
      <section className="relative overflow-hidden rounded-3xl border border-border bg-sidebar px-5 py-5 text-sidebar-foreground shadow-sm md:px-7 md:py-7">
        <div className="absolute inset-0 opacity-35 [background-image:radial-gradient(circle_at_20%_20%,rgba(217,119,87,0.45),transparent_28%),linear-gradient(135deg,rgba(255,255,255,0.14),transparent_36%)]" />
        <div className="relative space-y-6">
          <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
            <div className="max-w-2xl">
              <Badge variant="secondary" className="border-white/15 bg-white/10 text-white">
                <Database className="size-3" />
                {t("dataset.pipeline_badge")}
              </Badge>
              <h1 className="mt-4 font-display text-3xl font-semibold tracking-tight text-white md:text-4xl">
                {t("dataset.title")}
              </h1>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-white/70 md:text-base">
                {t("dataset.subtitle")}
              </p>
            </div>
            <div className="grid grid-cols-2 gap-2 text-white md:min-w-64">
              <div className="rounded-2xl border border-white/10 bg-white/8 p-3 backdrop-blur-sm">
                <div className="text-xs text-white/55">{t("dataset.total_sessions")}</div>
                <div className="mt-1 font-mono text-2xl font-semibold tabular-nums">
                  {loadingPreview ? "..." : formatNumber(totalSessions)}
                </div>
              </div>
              <div className="rounded-2xl border border-white/10 bg-white/8 p-3 backdrop-blur-sm">
                <div className="text-xs text-white/55">{t("dataset.active_filters")}</div>
                <div className="mt-1 font-mono text-2xl font-semibold tabular-nums">
                  {activeFilters}
                </div>
              </div>
            </div>
          </div>

          <div className="grid gap-2 md:grid-cols-3">
            <StepBadge index={1} title={t("dataset.step_configure")} description={t("dataset.step_configure_desc")} />
            <StepBadge index={2} title={t("dataset.step_review")} description={t("dataset.step_review_desc")} />
            <StepBadge index={3} title={t("dataset.step_export")} description={t("dataset.step_export_desc")} />
          </div>
        </div>
      </section>

      <Card>
        <CardHeader>
          <SectionHeading number={1} title={t("dataset.step_configure")} description={t("dataset.step_configure_desc")} />
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid gap-4 lg:grid-cols-[1fr_1.3fr]">
            <div className="rounded-xl border border-border bg-muted/20 p-4">
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
                className="mt-4 h-2 w-full cursor-pointer accent-primary"
                aria-label={t("dataset.min_score")}
              />
              <div className="mt-2 flex justify-between font-mono text-xs text-muted-foreground">
                {[0, 1, 2, 3, 4, 5].map((score) => <span key={score}>{score}</span>)}
              </div>
            </div>

            <div className="flex flex-col gap-3 rounded-xl border border-border bg-muted/20 p-4">
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
                <Button variant="ghost" size="sm" onClick={handleClearFilters} className="gap-1 text-muted-foreground">
                  <RotateCcw className="size-3.5" />
                  {t("dataset.clear_filters")}
                </Button>
              </div>

              <div className="grid gap-2 sm:grid-cols-3">
                <Badge variant="outline" className="min-h-9 justify-start gap-2 rounded-lg px-3">
                  <Filter className="size-3" />
                  {t("dataset.export_filter_score")}: {minScore}
                </Badge>
                <Badge variant="outline" className="min-h-9 justify-start gap-2 rounded-lg px-3">
                  <SlidersHorizontal className="size-3" />
                  {selectedModels.length > 0 ? `${selectedModels.length} ${t("dataset.export_filter_models")}` : t("dataset.export_filter_all_models")}
                </Badge>
                <Badge variant="outline" className="min-h-9 justify-start gap-2 rounded-lg px-3">
                  <Check className="size-3" />
                  {t("dataset.export_format_value")}
                </Badge>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <SectionHeading number={2} title={t("dataset.step_review")} description={t("dataset.step_review_desc")} />
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-3">
            <MetricCard label={t("dataset.total_sessions")} value={formatNumber(totalSessions)} loading={loadingPreview} />
            <MetricCard label={t("dataset.top_model")} value={topModel} loading={loadingPreview} />
            <MetricCard label={t("dataset.export_format")} value={t("dataset.export_format_value")} hint={t("dataset.retained_hint")} loading={loadingPreview} />
          </div>

          {totalSessions === 0 && !loadingPreview ? (
            <div className="rounded-xl border border-dashed border-border bg-muted/20 px-4 py-10 text-center">
              <div className="font-medium text-foreground">{t("dataset.no_data")}</div>
              <div className="mt-1 text-sm text-muted-foreground">{t("dataset.empty_hint")}</div>
            </div>
          ) : (
            <div className="grid gap-4 lg:grid-cols-2">
              <DistributionList title={t("dataset.score_distribution")} items={scoreItems} emptyText={t("dataset.no_data")} />
              <DistributionList title={t("dataset.model_distribution")} items={modelItems} emptyText={t("dataset.no_models")} />
            </div>
          )}

          <div className="rounded-xl border border-border bg-card">
            <button
              type="button"
              className="flex min-h-12 w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm font-medium text-foreground transition-colors hover:bg-muted/40"
              onClick={() => setShowFormatPreview((value) => !value)}
            >
              <span className="flex items-center gap-2">
                <Eye className="size-4 text-primary" />
                {showFormatPreview ? t("dataset.preview_toggle_hide") : t("dataset.preview_toggle_show")}
              </span>
              <ChevronDown className={cn("size-4 transition-transform", showFormatPreview && "rotate-180")} />
            </button>
            {showFormatPreview && (
              <div className="border-t border-border p-4">
                {loadingFormat ? (
                  <Skeleton className="h-48 w-full" />
                ) : formatPreview?.sharegptJson ? (
                  <div className="space-y-2">
                    <div className="text-xs text-muted-foreground">
                      {replaceVars(t("dataset.sample_of"), {
                        current: String((formatPreview.offset ?? 0) + 1),
                        total: String(formatPreview.totalCount ?? 0),
                      })}
                    </div>
                    <pre className="max-h-80 overflow-auto rounded-lg bg-sidebar p-4 font-mono text-xs leading-5 text-sidebar-foreground">
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
        </CardContent>
      </Card>

      <Card className="border-primary/25 bg-primary/5">
        <CardHeader>
          <SectionHeading number={3} title={t("dataset.step_export")} description={t("dataset.step_export_desc")} />
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 lg:grid-cols-[1fr_auto] lg:items-center">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div>
                <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t("dataset.export_filter_score")}</div>
                <div className="mt-1 text-sm text-foreground">{minScore}</div>
              </div>
              <div>
                <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t("dataset.export_filter_models")}</div>
                <div className="mt-1 max-w-[24ch] truncate text-sm text-foreground" title={selectedModels.join(", ") || t("dataset.export_filter_all_models")}>
                  {selectedModels.join(", ") || t("dataset.export_filter_all_models")}
                </div>
              </div>
              <div>
                <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t("dataset.export_filter_time")}</div>
                <div className="mt-1 max-w-[28ch] truncate text-sm text-foreground" title={formatTimeRange(params.startTime, params.endTime)}>
                  {formatTimeRange(params.startTime, params.endTime)}
                </div>
              </div>
              <div>
                <div className="text-xs font-medium uppercase tracking-wider text-muted-foreground">{t("dataset.export_format")}</div>
                <div className="mt-1 flex items-center gap-2 text-sm text-foreground">
                  <FileJson className="size-4 text-primary" />
                  {t("dataset.export_format_value")}
                </div>
              </div>
            </div>

            <div className="min-w-64 space-y-3 rounded-xl border border-primary/20 bg-card p-4 shadow-sm">
              <CardDescription>
                {replaceVars(t("dataset.export_confirm_total"), { total: formatNumber(totalSessions) })}
              </CardDescription>
              {exporting && (
                <div className="h-2 overflow-hidden rounded-full bg-muted">
                  <div className="h-full rounded-full bg-primary transition-[width] duration-200" style={{ width: `${exportProgress}%` }} />
                </div>
              )}
              <Button className="h-11 w-full gap-2" size="lg" onClick={handleExport} disabled={exporting || loadingPreview || totalSessions === 0}>
                {exporting ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
                {exporting ? t("dataset.exporting") : t("dataset.export")}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
