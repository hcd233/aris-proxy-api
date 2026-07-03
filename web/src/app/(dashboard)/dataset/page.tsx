"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api-client";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { Database, Download, BarChart3 } from "lucide-react";
import { toast } from "sonner";

interface PreviewData {
  totalSessions: number;
  scoreDistribution: Record<number, number>;
  modelDistribution: Record<string, number>;
}

export default function DatasetPage() {
  const t = useT();
  const [minScore, setMinScore] = useState("4");
  const [models, setModels] = useState("");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [preview, setPreview] = useState<PreviewData | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [exporting, setExporting] = useState(false);

  const buildParams = useCallback(() => {
    const params: {
      minScore?: number;
      models?: string[];
      startTime?: string;
      endTime?: string;
    } = {};
    const score = parseInt(minScore, 10);
    if (score >= 1 && score <= 5) params.minScore = score;
    const modelList = models
      .split(",")
      .map((m) => m.trim())
      .filter(Boolean);
    if (modelList.length > 0) params.models = modelList;
    if (startTime) params.startTime = new Date(startTime).toISOString();
    if (endTime) params.endTime = new Date(endTime).toISOString();
    return params;
  }, [minScore, models, startTime, endTime]);

  const handlePreview = useCallback(async () => {
    setLoadingPreview(true);
    try {
      const rsp = await api.previewDataset(buildParams());
      if (rsp.error) {
        toast.error(rsp.error.message ?? t("dataset.preview_error"));
        return;
      }
      setPreview({
        totalSessions: rsp.totalSessions ?? 0,
        scoreDistribution: rsp.scoreDistribution ?? {},
        modelDistribution: rsp.modelDistribution ?? {},
      });
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("dataset.preview_error"));
    } finally {
      setLoadingPreview(false);
    }
  }, [buildParams, t]);

  const handleExport = useCallback(async () => {
    setExporting(true);
    try {
      const blob = await api.exportDataset(buildParams());
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "dataset.jsonl";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      toast.success(t("dataset.export_success"));
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("dataset.export_error"));
    } finally {
      setExporting(false);
    }
  }, [buildParams, t]);

  const scoreEntries = preview
    ? Object.entries(preview.scoreDistribution).sort(([a], [b]) => Number(a) - Number(b))
    : [];
  const modelEntries = preview
    ? Object.entries(preview.modelDistribution).sort(([, a], [, b]) => b - a)
    : [];

  return (
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
          {t("dataset.title")}
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground">{t("dataset.subtitle")}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display flex items-center gap-2">
            <Database className="size-5" />
            {t("dataset.filter_title")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            <div className="space-y-2">
              <Label htmlFor="minScore">{t("dataset.min_score")}</Label>
              <Input
                id="minScore"
                type="number"
                min={1}
                max={5}
                value={minScore}
                onChange={(e) => setMinScore(e.target.value)}
                placeholder="1-5"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="models">{t("dataset.models")}</Label>
              <Input
                id="models"
                value={models}
                onChange={(e) => setModels(e.target.value)}
                placeholder="claude-3.5-sonnet,gpt-4o"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="startTime">{t("dataset.start_time")}</Label>
              <Input
                id="startTime"
                type="datetime-local"
                value={startTime}
                onChange={(e) => setStartTime(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="endTime">{t("dataset.end_time")}</Label>
              <Input
                id="endTime"
                type="datetime-local"
                value={endTime}
                onChange={(e) => setEndTime(e.target.value)}
              />
            </div>
          </div>

          <div className="flex flex-wrap gap-3">
            <Button onClick={handlePreview} disabled={loadingPreview} variant="outline">
              <BarChart3 className="size-4" />
              {loadingPreview ? t("common.loading") : t("dataset.preview")}
            </Button>
            <Button onClick={handleExport} disabled={exporting}>
              <Download className="size-4" />
              {exporting ? t("dataset.exporting") : t("dataset.export")}
            </Button>
          </div>
        </CardContent>
      </Card>

      {loadingPreview && (
        <Card>
          <CardContent className="pt-6">
            <Skeleton className="h-40 w-full" />
          </CardContent>
        </Card>
      )}

      {preview && !loadingPreview && (
        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("dataset.preview_title")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex items-center gap-3">
              <span className="text-sm text-muted-foreground">{t("dataset.total_sessions")}</span>
              <span className="font-display text-2xl font-bold">{preview.totalSessions}</span>
            </div>

            {scoreEntries.length > 0 && (
              <div>
                <h3 className="mb-2 text-sm font-medium text-muted-foreground">
                  {t("dataset.score_distribution")}
                </h3>
                <div className="flex flex-wrap gap-2">
                  {scoreEntries.map(([score, count]) => (
                    <Badge key={score} variant="secondary" className="text-sm">
                      {t("dataset.score_label", `${score}★`)}: {count}
                    </Badge>
                  ))}
                </div>
              </div>
            )}

            {modelEntries.length > 0 && (
              <div>
                <h3 className="mb-2 text-sm font-medium text-muted-foreground">
                  {t("dataset.model_distribution")}
                </h3>
                <div className="flex flex-wrap gap-2">
                  {modelEntries.map(([model, count]) => (
                    <Badge key={model} variant="outline" className="text-sm">
                      {model}: {count}
                    </Badge>
                  ))}
                </div>
              </div>
            )}

            {preview.totalSessions === 0 && (
              <div className="flex flex-col items-center justify-center py-8 text-center">
                <Database className="mb-3 size-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">{t("dataset.no_data")}</p>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
