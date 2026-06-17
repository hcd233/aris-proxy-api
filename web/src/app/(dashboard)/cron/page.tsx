"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import type { CronJobItem, PageInfo } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ChevronLeft, ChevronRight, Search, Timer } from "lucide-react";

import { toast } from "sonner";
import { PermissionGuard } from "@/components/permission-guard";

function formatTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export default function CronPage() {
  const [jobs, setJobs] = useState<CronJobItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: 1, pageSize: 20, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [pageInputValue, setPageInputValue] = useState("1");
  const [updating, setUpdating] = useState<Record<string, boolean>>({});

  const fetchJobs = useCallback(async (page: number, pageSize: number, query: string) => {
    setLoading(true);
    try {
      const rsp = await api.listCronJobs({ page, pageSize, query: query || undefined });
      if (rsp.error) {
        toast.error(rsp.error.message ?? "Failed to load cron jobs");
        return;
      }
      setJobs(rsp.jobs ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPageInputValue(String(rsp.pageInfo.page));
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to load cron jobs");
    } finally {
      setLoading(false);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Initial data fetch on mount */
  useEffect(() => {
    fetchJobs(1, 20, "");
  }, [fetchJobs]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleToggle = async (job: CronJobItem) => {
    setUpdating((prev) => ({ ...prev, [job.name]: true }));
    try {
      const rsp = await api.updateCronJob({ name: job.name, enabled: !job.enabled });
      if (rsp.error) {
        toast.error(rsp.error.message ?? "Failed to update cron job");
        return;
      }
      setJobs((prev) =>
        prev.map((j) => (j.name === job.name ? { ...j, enabled: !j.enabled } : j))
      );
      toast.success(`${job.name} ${!job.enabled ? "enabled" : "disabled"}`);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update cron job");
    } finally {
      setUpdating((prev) => ({ ...prev, [job.name]: false }));
    }
  };

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize)),
    [pageInfo]
  );

  const refresh = (page: number, pageSize?: number) =>
    fetchJobs(page, pageSize ?? pageInfo.pageSize, searchQuery);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            Cron Jobs
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            View and enable or disable scheduled cron jobs.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display flex items-center gap-2">
              <Timer className="size-5" />
              Cron Jobs
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search by name or spec..."
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
                <p className="text-sm text-muted-foreground">No cron jobs found</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Spec</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Enabled</TableHead>
                    <TableHead>Updated At</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {jobs.map((job) => (
                    <TableRow key={job.name}>
                      <TableCell className="font-medium">{job.name}</TableCell>
                      <TableCell className="font-mono text-muted-foreground">{job.spec}</TableCell>
                      <TableCell className="text-muted-foreground">{job.description || "—"}</TableCell>
                      <TableCell>
                        <Switch
                          checked={job.enabled}
                          disabled={updating[job.name]}
                          onCheckedChange={() => handleToggle(job)}
                        />
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatTime(job.updatedAt)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            {pageInfo.total > 0 && (
              <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
                <p className="hidden text-sm text-muted-foreground md:block">
                  {pageInfo.total} job{pageInfo.total !== 1 ? "s" : ""} total
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pageInfo.page <= 1}
                    onClick={() => refresh(pageInfo.page - 1)}
                  >
                    <ChevronLeft className="size-4" />
                  </Button>
                  <div className="flex items-center gap-1.5 text-sm">
                    <span className="text-muted-foreground">Page</span>
                    <input
                      type="number"
                      min={1}
                      max={totalPages}
                      value={pageInputValue}
                      onChange={(e) => setPageInputValue(e.target.value)}
                      className="h-8 w-14 rounded-md border border-input bg-transparent px-2 py-1 text-center text-sm tabular-nums focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none dark:bg-input/30"
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          let page = parseInt(pageInputValue, 10);
                          if (Number.isNaN(page)) page = 1;
                          page = Math.max(1, Math.min(page, totalPages));
                          refresh(page);
                        }
                      }}
                      onBlur={() => {
                        let page = parseInt(pageInputValue, 10);
                        if (Number.isNaN(page)) page = 1;
                        page = Math.max(1, Math.min(page, totalPages));
                        refresh(page);
                      }}
                    />
                    <span className="text-muted-foreground">/ {totalPages}</span>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={pageInfo.page >= totalPages}
                    onClick={() => refresh(pageInfo.page + 1)}
                  >
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </PermissionGuard>
  );
}
