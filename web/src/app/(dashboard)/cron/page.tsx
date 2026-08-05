"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import cronstrue from "cronstrue";
import { api } from "@/lib/api-client";
import type { CronJobItem, PageInfo } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Timer, Pencil, Lock, Play } from "lucide-react";

import { useI18n } from "@/lib/i18n";
import { PaginationBar } from "@/components/pagination-bar";
import { PageHeader } from "@/components/page-header";
import { SearchInput } from "@/components/search-input";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { toast } from "sonner";
import { PermissionGuard } from "@/components/permission-guard";
import { showErrorToast } from "@/lib/api-error-handler";
import { BusinessErrorCode, parseError } from "@/lib/api-errors";
import type { ApiError } from "@/lib/api-client";
import { ScheduleEditorDialog } from "@/components/cron/schedule-editor";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";

function formatTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function specToHuman(spec: string, locale: string): string {
  try {
    const cronLocale = locale === "zh" ? "zh_CN" : "en";
    return cronstrue.toString(spec, { locale: cronLocale });
  } catch {
    return spec;
  }
}

export default function CronPage() {
  const { t, locale } = useI18n();
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.cron.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.cron.pageSize", 20);
  const [jobs, setJobs] = useState<CronJobItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [updating, setUpdating] = useState<Record<string, boolean>>({});
  const [editingJob, setEditingJob] = useState<CronJobItem | null>(null);
  const [triggeringJob, setTriggeringJob] = useState<CronJobItem | null>(null);
  const [triggering, setTriggering] = useState(false);

  const fetchJobs = useCallback(async (page: number, pageSize: number, query: string) => {
    setLoading(true);
    try {
      const rsp = await api.listCronJobs({ page, pageSize, query: query || undefined });
      if (rsp.error) {
        showErrorToast(rsp.error, { title: t("cron.load_error") });
        return;
      }
      setJobs(rsp.jobs ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPersistedPage(rsp.pageInfo.page);
        setPersistedPageSize(rsp.pageInfo.pageSize);
      }
    } catch (err) {
      showErrorToast(err, { title: t("cron.load_error") });
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize, t]);

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchJobs(persistedPage, persistedPageSize, "");
  }, [fetchJobs]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const handleToggle = async (job: CronJobItem) => {
    if (job.type === "core") {
        toast.error(t("cron.core_cannot_disable"));
      return;
    }
    setUpdating((prev) => ({ ...prev, [job.name]: true }));
    try {
      const rsp = await api.updateCronJob({ name: job.name, enabled: !job.enabled });
      if (rsp.error) {
        showErrorToast(rsp.error, { title: t("cron.update_error") });
        return;
      }
      setJobs((prev) =>
        prev.map((j) => (j.name === job.name ? { ...j, enabled: !j.enabled } : j))
      );
      toast.success(`${job.name} ${!job.enabled ? t("cron.enabled") : t("cron.disabled")}`);
    } catch (err) {
      showErrorToast(err, { title: t("cron.update_error") });
    } finally {
      setUpdating((prev) => ({ ...prev, [job.name]: false }));
    }
  };

  const handleSaveSpec = useCallback(async (spec: string) => {
    if (!editingJob) return;
    const rsp = await api.updateCronJob({ name: editingJob.name, spec });
    if (rsp.error) {
      showErrorToast(rsp.error, { title: t("cron.update_schedule_error") });
      return;
    }
    setJobs((prev) =>
      prev.map((j) => (j.name === editingJob.name ? { ...j, spec } : j))
    );
    toast.success(`${editingJob.name} ${t("cron.schedule_updated")}`);
  }, [editingJob, t]);

  const handleTrigger = async () => {
    if (!triggeringJob) return;
    setTriggering(true);
    try {
      await api.triggerCronJob(triggeringJob.name);
      toast.success(t("cron.triggered"));
    } catch (err) {
      // ApiError 构造时已将 {error:{code,message}} 解析进 structured，优先使用
      const parsed = (err as ApiError).structured ?? parseError(err);
      if (parsed.code === BusinessErrorCode.ResourceLocked) {
        toast.error(t("cron.running"));
      } else {
        showErrorToast(err, { title: t("cron.trigger_error") });
      }
    } finally {
      setTriggering(false);
      setTriggeringJob(null);
    }
  };

  const refresh = (page: number, pageSize?: number) =>
    fetchJobs(page, pageSize ?? pageInfo.pageSize, searchQuery);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <PageHeader
          title={t("cron.title")}
          description={t("cron.subtitle")}
        />

        <Card>
          <CardHeader>
            <CardTitle className="font-display flex items-center gap-2">
              <Timer className="size-5" />
              {t("cron.all_jobs")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <SearchInput
                placeholder={t("cron.search_placeholder")}
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={() => refresh(1)}
              />
            </div>

            {loading ? (
              <TableSkeleton rows={5} rowClassName="h-10" />
            ) : jobs.length === 0 ? (
              <ListEmptyState icon={<Timer className="mb-3 size-10 text-muted-foreground/50" />} message={t("cron.no_jobs")} />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("cron.name")}</TableHead>
                    <TableHead>{t("cron.type")}</TableHead>
                    <TableHead>{t("cron.spec")}</TableHead>
                    <TableHead>{t("cron.description")}</TableHead>
                    <TableHead>{t("cron.enabled")}</TableHead>
                    <TableHead>{t("cron.updated_at")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {jobs.map((job) => (
                    <TableRow key={job.name}>
                      <TableCell className="font-medium">{job.name}</TableCell>
                      <TableCell>
                        <Badge variant={job.type === "core" ? "default" : "secondary"}>
                          {job.type === "core" ? t("cron.core") : t("cron.functional")}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <div className="min-w-0">
                            <p className="text-sm">{specToHuman(job.spec, locale)}</p>
                            <p className="font-mono text-xs text-muted-foreground">{job.spec}</p>
                          </div>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7 shrink-0"
                            onClick={() => setEditingJob(job)}
                          >
                            <Pencil className="size-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-7 shrink-0"
                            onClick={() => setTriggeringJob(job)}
                            aria-label={t("cron.trigger")}
                          >
                            <Play className="size-3.5" />
                          </Button>
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">{job.description || "—"}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1.5">
                          <Switch
                            checked={job.enabled}
                            disabled={updating[job.name] || job.type === "core"}
                            onCheckedChange={() => handleToggle(job)}
                          />
                          {job.type === "core" && (
                            <Lock className="size-3.5 text-muted-foreground" />
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatTime(job.updatedAt)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <PaginationBar
              pageInfo={pageInfo}
              onChange={(page, pageSize) => refresh(page, pageSize)}
              totalLabel={t("pagination.jobs")}
            />
          </CardContent>
        </Card>

        <ScheduleEditorDialog
          open={editingJob !== null}
          onOpenChange={(open) => { if (!open) setEditingJob(null); }}
          job={editingJob}
          onSave={handleSaveSpec}
        />

        <DeleteConfirmDialog
          open={triggeringJob !== null}
          onOpenChange={(open) => { if (!open && !triggering) setTriggeringJob(null); }}
          title={t("cron.trigger_confirm_title")}
          description={t("cron.trigger_confirm_desc")}
          confirmLabel={t("cron.trigger")}
          loadingLabel={t("cron.triggering")}
          loading={triggering}
          onConfirm={handleTrigger}
        />
      </div>
    </PermissionGuard>
  );
}
