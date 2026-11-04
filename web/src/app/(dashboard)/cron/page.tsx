"use client";

import { useCallback, useEffect, useState } from "react";
import { usePaginatedList } from "@/hooks/use-paginated-list";
import { useOptimisticUpdate } from "@/hooks/use-optimistic-update";
import { specToHuman } from "@/lib/cron";
import { api } from "@/lib/api-client";
import type { CronJobItem } from "@/lib/types";
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
import { TooltipProvider } from "@/components/ui/tooltip";

import { useI18n } from "@/lib/i18n";
import { cronJobLabel } from "@/lib/utils";
import { PaginationBar } from "@/components/pagination-bar";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { toast } from "sonner";
import { PermissionGuard } from "@/components/permission-guard";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import { showErrorToast } from "@/lib/api-error-handler";
import { BusinessErrorCode, parseError } from "@/lib/api-errors";
import type { ApiError } from "@/lib/api-client";
import { ScheduleEditorDialog } from "@/components/cron/schedule-editor";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { formatDateTime } from "@/lib/utils";
import { TruncatedText } from "@/components/truncated-text";

export default function CronPage() {
  const { t, locale } = useI18n();
  const [editingJob, setEditingJob] = useState<CronJobItem | null>(null);
  const [triggeringJob, setTriggeringJob] = useState<CronJobItem | null>(null);
  const [triggering, setTriggering] = useState(false);

  const filterBar = useFilterBar({
    persistKey: "dashboard.cron",
    facets: [],
    freeTextPlaceholder: t("cron.search_placeholder"),
  });
  const { queryParams } = filterBar;

  const {
    items: jobs,
    setItems: setJobs,
    pageInfo,
    loading,
    load: fetchJobs,
  } = usePaginatedList({
    persistKey: "dashboard.cron",
    errorTitle: t("cron.load_error"),
    fetchPage: async (page, pageSize, query: string) => {
      const rsp = await api.listCronJobs({ page, pageSize, query: query || undefined });
      return { items: rsp.jobs, pageInfo: rsp.pageInfo, error: rsp.error };
    },
  });

  /* eslint-disable react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时以持久化关键词发起首次查询 */
  useEffect(() => {
    fetchJobs(1, pageInfo.pageSize, queryParams.freeText);
  }, [queryParams]);
  /* eslint-enable react-hooks/exhaustive-deps */

  // enabled 开关：乐观更新 + 失败回滚，避免整表重拉导致闪烁
  const toggleEnabled = useOptimisticUpdate<CronJobItem>({
    setItems: setJobs,
    getKey: (job) => job.name,
    update: async (job) => {
      const rsp = await api.updateCronJob({ name: job.name, enabled: job.enabled });
      if (rsp.error) throw rsp.error;
    },
    onSuccess: (job) =>
      toast.success(`${job.name} ${job.enabled ? t("cron.enabled") : t("cron.disabled")}`),
    onError: (err) => showErrorToast(err, { title: t("cron.update_error") }),
  });

  const handleToggle = (job: CronJobItem) => {
    if (job.type === "core") {
      toast.error(t("cron.core_cannot_disable"));
      return;
    }
    toggleEnabled.apply(job, { enabled: !job.enabled });
  };

  const handleSaveSpec = useCallback(
    async (spec: string) => {
      if (!editingJob) return;
      const rsp = await api.updateCronJob({ name: editingJob.name, spec });
      if (rsp.error) {
        showErrorToast(rsp.error, { title: t("cron.update_schedule_error") });
        return;
      }
      setJobs((prev) => prev.map((j) => (j.name === editingJob.name ? { ...j, spec } : j)));
      toast.success(`${editingJob.name} ${t("cron.schedule_updated")}`);
    },
    [editingJob, setJobs, t],
  );

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
    fetchJobs(page, pageSize ?? pageInfo.pageSize, queryParams.freeText);

  return (
    <PermissionGuard adminOnly module="cron">
      <TooltipProvider>
        <div className="space-y-8">
          <PageHeader title={t("cron.title")} description={t("cron.subtitle")} />

          <Card>
            <CardHeader>
              <CardTitle className="font-display flex items-center gap-2">
                <Timer className="size-5" />
                {t("cron.all_jobs")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {/* Search — faceted bar */}
              <div className="mb-4 flex">
                <FilterBar {...filterBar} facets={[]} placeholder={t("cron.search_placeholder")} />
              </div>
              {filterBar.tokens.length > 0 && (
                <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                  {t("filter_bar.applied_count").replace(
                    "{count}",
                    String(filterBar.tokens.length),
                  )}
                </p>
              )}

              {loading ? (
                <TableSkeleton rows={5} rowClassName="h-10" />
              ) : jobs.length === 0 ? (
                <ListEmptyState icon={Timer} message={t("cron.no_jobs")} />
              ) : (
                <Table fit>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-[8rem]">{t("cron.name")}</TableHead>
                      <TableHead className="w-[4.5rem]">{t("cron.type")}</TableHead>
                      <TableHead className="w-[10.5rem]">{t("cron.spec")}</TableHead>
                      {/* 描述不声明宽度：fixed 布局只允许一列吃剩余空间，两列都留空
                          会把剩余宽度对半分，描述只剩 ~150px（英文句子只剩三个词） */}
                      <TableHead>{t("cron.description")}</TableHead>
                      <TableHead className="w-[5rem]">{t("cron.enabled")}</TableHead>
                      <TableHead className="w-[10.5rem]">{t("cron.updated_at")}</TableHead>
                      <TableHead className="w-[6rem] text-right">{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {jobs.map((job) => (
                      <TableRow key={job.name}>
                        <TableCell className="font-medium">
                          {/* 悬停给原始模块名：界面看中文，排查时仍能拿到后端标识 */}
                          <TruncatedText
                            className="block max-w-[16ch]"
                            align="start"
                            tooltip={job.name}
                          >
                            {cronJobLabel(job.name, t)}
                          </TruncatedText>
                        </TableCell>
                        <TableCell>
                          <Badge variant={job.type === "core" ? "outline" : "secondary"}>
                            {job.type === "core" ? t("cron.core") : t("cron.functional")}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="min-w-0">
                            <TruncatedText
                              render={<p />}
                              className="max-w-[24ch] text-sm"
                              align="start"
                            >
                              {specToHuman(job.spec, locale)}
                            </TruncatedText>
                            <TruncatedText
                              render={<p />}
                              className="max-w-[20ch] font-mono text-xs text-muted-foreground"
                              align="start"
                            >
                              {job.spec}
                            </TruncatedText>
                          </div>
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          <TruncatedText className="block max-w-full" align="start">
                            {job.description || "—"}
                          </TruncatedText>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-1.5">
                            <Switch
                              checked={job.enabled}
                              disabled={toggleEnabled.updatingKey !== null || job.type === "core"}
                              onCheckedChange={() => handleToggle(job)}
                            />
                            {job.type === "core" && (
                              <Lock className="size-3.5 text-muted-foreground" />
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {formatDateTime(job.updatedAt)}
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => setEditingJob(job)}
                              aria-label={t("common.edit")}
                            >
                              <Pencil className="size-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => setTriggeringJob(job)}
                              aria-label={t("cron.trigger")}
                            >
                              <Play className="size-4" />
                            </Button>
                          </div>
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
            onOpenChange={(open) => {
              if (!open) setEditingJob(null);
            }}
            job={editingJob}
            onSave={handleSaveSpec}
          />

          <DeleteConfirmDialog
            open={triggeringJob !== null}
            onOpenChange={(open) => {
              if (!open && !triggering) setTriggeringJob(null);
            }}
            title={t("cron.trigger_confirm_title")}
            description={t("cron.trigger_confirm_desc")}
            confirmLabel={t("cron.trigger")}
            loadingLabel={t("cron.triggering")}
            loading={triggering}
            onConfirm={handleTrigger}
          />
        </div>
      </TooltipProvider>
    </PermissionGuard>
  );
}
