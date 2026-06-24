"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import cronstrue from "cronstrue";
import { api } from "@/lib/api-client";
import type { CronJobItem, PageInfo } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Search, Timer, Pencil, Lock } from "lucide-react";

import { useT } from "@/lib/i18n";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { PermissionGuard } from "@/components/permission-guard";
import { ScheduleEditorDialog } from "@/components/cron/schedule-editor";

function formatTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function specToHuman(spec: string): string {
  try {
    return cronstrue.toString(spec, { locale: "en" });
  } catch {
    return spec;
  }
}

export default function CronPage() {
  const t = useT();
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.cron.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.cron.pageSize", 20);
  const [jobs, setJobs] = useState<CronJobItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [updating, setUpdating] = useState<Record<string, boolean>>({});
  const [editingJob, setEditingJob] = useState<CronJobItem | null>(null);

  const fetchJobs = useCallback(async (page: number, pageSize: number, query: string) => {
    setLoading(true);
    try {
      const rsp = await api.listCronJobs({ page, pageSize, query: query || undefined });
      if (rsp.error) {
        toast.error(rsp.error.message ?? t("cron.load_error"));
        return;
      }
      setJobs(rsp.jobs ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPersistedPage(rsp.pageInfo.page);
        setPersistedPageSize(rsp.pageInfo.pageSize);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("cron.load_error"));
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize]);

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
        toast.error(rsp.error.message ?? t("cron.update_error"));
        return;
      }
      setJobs((prev) =>
        prev.map((j) => (j.name === job.name ? { ...j, enabled: !j.enabled } : j))
      );
      toast.success(`${job.name} ${!job.enabled ? t("cron.enabled") : t("cron.disabled")}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("cron.update_error"));
    } finally {
      setUpdating((prev) => ({ ...prev, [job.name]: false }));
    }
  };

  const handleSaveSpec = useCallback(async (spec: string) => {
    if (!editingJob) return;
    const rsp = await api.updateCronJob({ name: editingJob.name, spec });
    if (rsp.error) {
      toast.error(rsp.error.message ?? t("cron.update_schedule_error"));
      return;
    }
    setJobs((prev) =>
      prev.map((j) => (j.name === editingJob.name ? { ...j, spec } : j))
    );
    toast.success(`${editingJob.name} ${t("cron.schedule_updated")}`);
  }, [editingJob]);

  const refresh = (page: number, pageSize?: number) =>
    fetchJobs(page, pageSize ?? pageInfo.pageSize, searchQuery);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            {t("cron.title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {t("cron.subtitle")}
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display flex items-center gap-2">
              <Timer className="size-5" />
              {t("cron.all_jobs")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder={t("cron.search_placeholder")}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") refresh(1);
                  }}
                  className="pl-9"
                />
              </div>
            </div>

            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : jobs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Timer className="mb-3 size-10 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">{t("cron.no_jobs")}</p>
              </div>
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
                            <p className="text-sm">{specToHuman(job.spec)}</p>
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
              totalLabel="jobs"
            />
          </CardContent>
        </Card>

        <ScheduleEditorDialog
          open={editingJob !== null}
          onOpenChange={(open) => { if (!open) setEditingJob(null); }}
          job={editingJob}
          onSave={handleSaveSpec}
        />
      </div>
    </PermissionGuard>
  );
}
