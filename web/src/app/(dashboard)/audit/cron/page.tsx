"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import type { CronCallAuditItem, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ScrollText, Search, X } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { useT } from "@/lib/i18n";
import { PermissionGuard } from "@/components/permission-guard";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";

function formatTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function buildCronAuditFilter(type: string[], status: string[]): string | undefined {
  const parts: string[] = [];
  if (type.length) parts.push(`type:${type.join("|")}`);
  if (status.length) parts.push(`status:${status.join("|")}`);
  return parts.length > 0 ? parts.join(" ") : undefined;
}



export default function CronAuditPage() {
  const t = useT();
  const statusLabelMap: Record<string, string> = {
    success: t("cron_audit.status_success"),
    failed: t("cron_audit.status_failed"),
    panic: t("cron_audit.status_panic"),
    skipped: t("cron_audit.status_skipped"),
  };
  const metadataLabelMap: Record<string, string> = {
    checked_sessions_count: "Checked",
    deduped_sessions_count: "Deduped",
    purged_messages_count: "Messages",
    purged_tools_count: "Tools",
    scanned_messages_count: "Scanned",
    extracted_messages_count: "Extracted",
    synced_hits_count: "Synced Hits",
  };
  function formatMetadata(metadata: Record<string, number> | undefined | null): string {
    if (!metadata || Object.keys(metadata).length === 0) return "—";
    return Object.entries(metadata)
      .map(([key, val]) => `${metadataLabelMap[key] ?? key}: ${val}`)
      .join(" | ");
  }
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.cronAudit.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.cronAudit.pageSize", 20);
  const [logs, setLogs] = useState<CronCallAuditItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("24h");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [filterType, setFilterType] = useState<string[]>([]);
  const [filterStatus, setFilterStatus] = useState<string[]>([]);
  const [typeOptions, setTypeOptions] = useState<string[]>([]);
  const [statusOptions, setStatusOptions] = useState<string[]>([]);

  const fetchLogs = useCallback(
    async (
      page: number,
      pageSize: number,
      query: string,
      range: TimeRangeKey,
      cs: string,
      ce: string,
      typeFilter: string[],
      statusFilter: string[],
    ) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(range, cs, ce);
        const filter = buildCronAuditFilter(typeFilter, statusFilter);
        const rsp = await api.listCronCallAudits({
          page,
          pageSize,
          query: query || undefined,
          sort: "desc",
          sortField: "created_at",
          startTime,
          endTime,
          filter,
        });
        if (rsp.error) {
          toast.error(rsp.error.message ?? t("common.error"));
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : t("common.error"));
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchLogs(persistedPage, persistedPageSize, "", "24h", "", "", [], []);
  }, [fetchLogs]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const fetchOptions = useCallback(async (range: TimeRangeKey, cs: string, ce: string) => {
    const { startTime, endTime } = computeRange(range, cs, ce);
    try {
      const [typeRsp, statusRsp] = await Promise.all([
        api.listCronCallAuditOptions({ field: "type", startTime, endTime }),
        api.listCronCallAuditOptions({ field: "status", startTime, endTime }),
      ]);
      if (!typeRsp.error && typeRsp.items) setTypeOptions(typeRsp.items);
      if (!statusRsp.error && statusRsp.items) setStatusOptions(statusRsp.items);
    } catch (err) {
      console.error("Failed to load cron audit options:", err);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Refresh filter options when range changes */
  useEffect(() => {
    fetchOptions(timeRange, customStart, customEnd);
  }, [timeRange, customStart, customEnd, fetchOptions]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const refresh = (page: number, pageSize?: number) =>
    fetchLogs(page, pageSize ?? pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterType, filterStatus);

  const handleCopyTrace = (traceId: string) => {
    if (!traceId) return;
    navigator.clipboard.writeText(traceId).then(
      () => toast.success(t("cron_audit.trace_copied")),
      () => toast.error(t("cron_audit.copy_failed")),
    );
  };

  const statusBadgeVariant = (status: string) => {
    switch (status) {
      case "success":
        return "secondary";
      case "skipped":
        return "outline";
      case "failed":
      case "panic":
        return "destructive";
      default:
        return "secondary";
    }
  };

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            {t("cron_audit.page_title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {t("cron_audit.page_subtitle")}
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("cron_audit.logs_title")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="flex flex-wrap items-center gap-2">
                <TimeRangePicker
                  value={timeRange}
                  customStart={customStart}
                  customEnd={customEnd}
                  onChange={(key, cs, ce) => {
                    setTimeRange(key);
                    setCustomStart(cs);
                    setCustomEnd(ce);
                    fetchLogs(1, pageInfo.pageSize, searchQuery, key, cs, ce, filterType, filterStatus);
                  }}
                />
                <MultiSelectPill
                  label={t("cron_audit.filter_type")}
                  options={typeOptions}
                  value={filterType}
                  onChange={(v) => {
                    setFilterType(v);
                    fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, v, filterStatus);
                  }}
                />
                <MultiSelectPill
                  label={t("cron_audit.filter_status")}
                  options={statusOptions}
                  value={filterStatus}
                  formatOption={(v) => statusLabelMap[v] ?? v}
                  onChange={(v) => {
                    setFilterStatus(v);
                    fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterType, v);
                  }}
                />
                {(filterType.length > 0 || filterStatus.length > 0) && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="gap-1 text-muted-foreground"
                    onClick={() => {
                      setFilterType([]);
                      setFilterStatus([]);
                      fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, [], []);
                    }}
                  >
                    <X size={14} />
                  {t("cron_audit.clear_filters")}
                </Button>
                )}
              </div>
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder={t("cron_audit.search_placeholder")}
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
            ) : logs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <ScrollText className="mb-3 size-10 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">{t("cron_audit.no_logs")}</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("cron_audit.time")}</TableHead>
                    <TableHead>{t("cron_audit.cron_name")}</TableHead>
                    <TableHead>{t("cron_audit.traceid")}</TableHead>
                    <TableHead>{t("cron_audit.filter_status")}</TableHead>
                    <TableHead>{t("cron_audit.duration")}</TableHead>
                    <TableHead>{t("cron_audit.metadata")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log) => (
                    <TableRow key={log.id} className={log.status === "success" ? "" : "bg-destructive/5"}>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatTime(log.createdAt)}
                      </TableCell>
                      <TableCell className="font-medium">{log.cronName}</TableCell>
                      <TableCell
                        className="cursor-pointer font-mono text-xs underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                        title={t("cron_audit.copy_traceid_title")}
                      >
                        {log.traceId.slice(-6) || "—"}
                      </TableCell>
                      <TableCell>
                        {log.status !== "success" && log.message ? (
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <button type="button">
                                    <Badge variant={statusBadgeVariant(log.status)} className="text-xs">
                                      {log.status}
                                    </Badge>
                                  </button>
                                }
                              />
                              <TooltipContent side="top" className="max-w-xs">
                                <span>{log.message}</span>
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        ) : (
                          <Badge variant={statusBadgeVariant(log.status)} className="text-xs">
                            {log.status}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{log.durationMs} ms</TableCell>
                      <TableCell className="max-w-[250px] truncate text-xs text-muted-foreground">
                        {formatMetadata(log.metadata)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}

            <PaginationBar
              pageInfo={pageInfo}
              onChange={(page, pageSize) => refresh(page, pageSize)}
              totalLabel={t("pagination.logs")}
            />
          </CardContent>
        </Card>
      </div>
    </PermissionGuard>
  );
}
