"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useOptimisticUpdate } from "@/hooks/use-optimistic-update";
import { useRequestSeq } from "@/hooks/use-request-seq";
import cronstrue from "cronstrue";
import "cronstrue/locales/zh_CN";
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
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";

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
import { RowDetailDrawer } from "@/components/row-detail-drawer";
import { rowDetailProps } from "@/lib/row-detail";
import { useIsCompact } from "@/hooks/use-mobile";

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
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.cron.pageSize",
    20,
  );
  const [jobs, setJobs] = useState<CronJobItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [editingJob, setEditingJob] = useState<CronJobItem | null>(null);
  const [triggeringJob, setTriggeringJob] = useState<CronJobItem | null>(null);
  const [triggering, setTriggering] = useState(false);
  // 窄屏（<1024px）表格只留关键列，点行看全部字段；按名称取当前行，开关切换后抽屉同步
  const isCompact = useIsCompact();
  const [detailName, setDetailName] = useState<string | null>(null);
  const detail = jobs.find((j) => j.name === detailName) ?? null;

  const filterBar = useFilterBar({
    persistKey: "dashboard.cron",
    facets: [],
    freeTextPlaceholder: t("cron.search_placeholder"),
  });
  const { queryParams } = filterBar;

  // 竞态守卫：关键词/筛选连点时慢响应不得覆盖新响应
  const requestSeq = useRequestSeq();
  const fetchJobs = useCallback(
    async (page: number, pageSize: number, query: string) => {
      const seq = requestSeq.begin();
      setLoading(true);
      try {
        const rsp = await api.listCronJobs({ page, pageSize, query: query || undefined });
        if (!requestSeq.isCurrent(seq)) return;
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
        if (!requestSeq.isCurrent(seq)) return;
        showErrorToast(err, { title: t("cron.load_error") });
      } finally {
        if (requestSeq.isCurrent(seq)) setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t, requestSeq],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时以持久化关键词发起首次查询 */
  useEffect(() => {
    fetchJobs(1, pageInfo.pageSize, queryParams.freeText);
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

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
    [editingJob, t],
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

  const typeBadge = (job: CronJobItem) => (
    <Badge variant={job.type === "core" ? "default" : "secondary"}>
      {job.type === "core" ? t("cron.core") : t("cron.functional")}
    </Badge>
  );

  const enabledSwitch = (job: CronJobItem) => (
    <div className="flex items-center gap-1.5">
      <Switch
        checked={job.enabled}
        disabled={toggleEnabled.updatingKey !== null || job.type === "core"}
        onCheckedChange={() => handleToggle(job)}
        aria-label={`${t("cron.enabled")}: ${cronJobLabel(job.name, t)}`}
      />
      {job.type === "core" && <Lock className="size-3.5 text-muted-foreground" />}
    </div>
  );

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
                      <TableHead>{t("cron.name")}</TableHead>
                      <TableHead priority="md" className="w-[4.5rem]">
                        {t("cron.type")}
                      </TableHead>
                      <TableHead priority="md" className="w-[12rem]">
                        {t("cron.spec")}
                      </TableHead>
                      <TableHead priority="lg">{t("cron.description")}</TableHead>
                      <TableHead className="w-[5rem]">{t("cron.enabled")}</TableHead>
                      <TableHead priority="lg" className="w-[10.5rem]">
                        {t("cron.updated_at")}
                      </TableHead>
                      <TableHead priority="md" className="w-[6rem] text-right">
                        {t("common.actions")}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {jobs.map((job) => (
                      <TableRow
                        key={job.name}
                        {...rowDetailProps(
                          isCompact,
                          () => setDetailName(job.name),
                          `${t("row_detail.view")}: ${cronJobLabel(job.name, t)}`,
                        )}
                      >
                        <TableCell className="font-medium">
                          <TooltipRoot>
                            <TooltipTrigger
                              render={
                                <span className="block max-w-[16ch] truncate">
                                  {cronJobLabel(job.name, t)}
                                </span>
                              }
                            />
                            <TooltipContent side="top" align="start" className="max-w-xs break-all">
                              {/* 悬停给原始模块名：界面看中文，排查时仍能拿到后端标识 */}
                              {job.name}
                            </TooltipContent>
                          </TooltipRoot>
                        </TableCell>
                        <TableCell priority="md">{typeBadge(job)}</TableCell>
                        <TableCell priority="md">
                          <div className="min-w-0">
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <p className="max-w-[24ch] truncate text-sm">
                                    {specToHuman(job.spec, locale)}
                                  </p>
                                }
                              />
                              <TooltipContent
                                side="top"
                                align="start"
                                className="max-w-xs break-all"
                              >
                                {specToHuman(job.spec, locale)}
                              </TooltipContent>
                            </TooltipRoot>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <p className="max-w-[20ch] truncate font-mono text-xs text-muted-foreground">
                                    {job.spec}
                                  </p>
                                }
                              />
                              <TooltipContent
                                side="top"
                                align="start"
                                className="max-w-xs break-all"
                              >
                                {job.spec}
                              </TooltipContent>
                            </TooltipRoot>
                          </div>
                        </TableCell>
                        <TableCell priority="lg" className="text-muted-foreground">
                          <TooltipRoot>
                            <TooltipTrigger
                              render={
                                <span className="block max-w-[30ch] truncate">
                                  {job.description || "—"}
                                </span>
                              }
                            />
                            <TooltipContent side="top" align="start" className="max-w-xs break-all">
                              {job.description || "—"}
                            </TooltipContent>
                          </TooltipRoot>
                        </TableCell>
                        <TableCell>{enabledSwitch(job)}</TableCell>
                        <TableCell priority="lg" className="text-muted-foreground">
                          {formatDateTime(job.updatedAt)}
                        </TableCell>
                        <TableCell priority="md" className="text-right">
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
              <RowDetailDrawer
                open={detail !== null}
                onOpenChange={(open) => !open && setDetailName(null)}
                title={detail ? cronJobLabel(detail.name, t) : ""}
                fields={
                  detail
                    ? [
                        {
                          label: t("cron.name"),
                          value: <span className="font-mono text-xs">{detail.name}</span>,
                        },
                        { label: t("cron.type"), value: typeBadge(detail) },
                        {
                          label: t("cron.spec"),
                          value: (
                            <div className="flex flex-col gap-0.5">
                              <span>{specToHuman(detail.spec, locale)}</span>
                              <span className="font-mono text-xs text-muted-foreground">
                                {detail.spec}
                              </span>
                            </div>
                          ),
                        },
                        { label: t("cron.description"), value: detail.description || "—" },
                        { label: t("cron.enabled"), value: enabledSwitch(detail) },
                        { label: t("cron.updated_at"), value: formatDateTime(detail.updatedAt) },
                      ]
                    : []
                }
                actions={
                  detail && (
                    <>
                      <Button
                        variant="outline"
                        onClick={() => {
                          setDetailName(null);
                          setEditingJob(detail);
                        }}
                      >
                        <Pencil data-icon="inline-start" />
                        {t("common.edit")}
                      </Button>
                      <Button
                        variant="outline"
                        onClick={() => {
                          setDetailName(null);
                          setTriggeringJob(detail);
                        }}
                      >
                        <Play data-icon="inline-start" />
                        {t("cron.trigger")}
                      </Button>
                    </>
                  )
                }
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
