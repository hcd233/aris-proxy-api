"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { CronCallAuditItem, PageInfo } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ScrollText } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { PermissionGuard } from "@/components/permission-guard";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef, FilterBarQueryParams } from "@/components/filter-bar/types";
import { copyTextToClipboard } from "@/lib/clipboard";
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

export default function CronAuditPage() {
  const { t, locale } = useI18n();
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
    const entries = Object.entries(metadata).filter(([, val]) => val !== 0);
    if (entries.length === 0) return "—";
    return entries.map(([key, val]) => `${metadataLabelMap[key] ?? key}: ${val}`).join(" | ");
  }
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.cronAudit.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.cronAudit.pageSize",
    20,
  );
  const [logs, setLogs] = useState<CronCallAuditItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("24h");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");

  const fetchOptionsFor = useCallback(
    (field: "type" | "status") => async () => {
      const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.listCronCallAuditOptions({ field, startTime, endTime });
      return rsp.items ?? [];
    },
    [timeRange, customStart, customEnd],
  );

  const facets = useMemo<FacetDef[]>(
    () => [
      { key: "type", label: t("cron_audit.filter_type"), options: fetchOptionsFor("type") },
      {
        key: "status",
        label: t("cron_audit.filter_status"),
        options: fetchOptionsFor("status"),
        formatValue: (v) => statusLabelMap[v] ?? v,
      },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, fetchOptionsFor],
  );

  const filterBar = useFilterBar({
    persistKey: "dashboard.cronAudit",
    facets,
    freeTextPlaceholder: t("cron_audit.search_placeholder"),
    optionsCacheKey: `${timeRange}:${customStart}:${customEnd}`,
  });
  const { queryParams } = filterBar;

  interface CronAuditQuery {
    page: number;
    pageSize: number;
    range: TimeRangeKey;
    cs: string;
    ce: string;
    qp: FilterBarQueryParams;
  }

  const fetchLogs = useCallback(
    async (q: CronAuditQuery) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(q.range, q.cs, q.ce);
        const rsp = await api.listCronCallAudits({
          page: q.page,
          pageSize: q.pageSize,
          query: q.qp.freeText || undefined,
          sort: "desc",
          sortField: "created_at",
          startTime,
          endTime,
          filter: q.qp.filter,
        });
        if (rsp.error) {
          showErrorToast(rsp.error, { title: t("common.error") });
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch (err) {
        showErrorToast(err, { title: t("common.error") });
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t],
  );

  const currentQuery = (): Omit<CronAuditQuery, "page" | "pageSize"> => ({
    range: timeRange,
    cs: customStart,
    ce: customEnd,
    qp: queryParams,
  });

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- token 变化回到第 1 页查询；挂载时以持久化筛选发起首次查询 */
  useEffect(() => {
    fetchLogs({ page: 1, pageSize: pageInfo.pageSize, ...currentQuery() });
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchLogs({ page, pageSize: pageSize ?? pageInfo.pageSize, ...currentQuery() });

  const handleCopyTrace = (traceId: string) => {
    if (!traceId) return;
    void copyTextToClipboard(traceId).then((ok) =>
      ok ? toast.success(t("cron_audit.trace_copied")) : toast.error(t("cron_audit.copy_failed")),
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
    <PermissionGuard adminOnly module="cron_audit">
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            {t("cron_audit.page_title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{t("cron_audit.page_subtitle")}</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("cron_audit.logs_title")}</CardTitle>
          </CardHeader>
          <CardContent>
            {/* Filters — faceted bar */}
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center">
              <TimeRangePicker
                value={timeRange}
                customStart={customStart}
                customEnd={customEnd}
                onChange={(key, cs, ce) => {
                  setTimeRange(key);
                  setCustomStart(cs);
                  setCustomEnd(ce);
                  fetchLogs({
                    page: 1,
                    pageSize: pageInfo.pageSize,
                    range: key,
                    cs,
                    ce,
                    qp: queryParams,
                  });
                }}
              />
              <FilterBar
                {...filterBar}
                facets={facets}
                placeholder={t("cron_audit.search_placeholder")}
              />
            </div>
            {filterBar.tokens.length > 0 && (
              <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
              </p>
            )}

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
                    <TableHead>{t("cron_audit.trigger_source")}</TableHead>
                    <TableHead>{t("cron_audit.trace_id")}</TableHead>
                    <TableHead>{t("cron_audit.filter_status")}</TableHead>
                    <TableHead>{t("cron_audit.duration")}</TableHead>
                    <TableHead>{t("cron_audit.metadata")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log) => (
                    <TableRow
                      key={log.id}
                      className={log.status === "success" ? "" : "bg-destructive/5"}
                    >
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatTime(log.createdAt)}
                      </TableCell>
                      <TableCell className="font-medium">{log.cronName}</TableCell>
                      <TableCell>
                        <Badge
                          variant={log.triggerSource === "manual" ? "default" : "secondary"}
                          className="text-xs"
                        >
                          {log.triggerSource === "manual"
                            ? t("cron_audit.trigger_manual")
                            : t("cron_audit.trigger_scheduled")}
                        </Badge>
                      </TableCell>
                      <TableCell
                        className="cursor-pointer font-mono text-xs underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                      >
                        <TooltipProvider>
                          <TooltipRoot>
                            <TooltipTrigger render={<span>{log.traceId.slice(-6) || "—"}</span>} />
                            <TooltipContent side="top">
                              {t("cron_audit.copy_traceid_title")}
                            </TooltipContent>
                          </TooltipRoot>
                        </TooltipProvider>
                      </TableCell>
                      <TableCell>
                        {log.status !== "success" && log.message ? (
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <button type="button">
                                    <Badge
                                      variant={statusBadgeVariant(log.status)}
                                      className="text-xs"
                                    >
                                      {statusLabelMap[log.status] ?? log.status}
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
                            {statusLabelMap[log.status] ?? log.status}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{log.durationMs} ms</TableCell>
                      <TableCell className="max-w-[250px] text-xs text-muted-foreground">
                        <TooltipProvider>
                          <TooltipRoot>
                            <TooltipTrigger
                              render={
                                <span className="block truncate text-left">
                                  {formatMetadata(log.metadata)}
                                </span>
                              }
                            />
                            <TooltipContent side="top" className="max-w-xs break-all">
                              {formatMetadata(log.metadata)}
                            </TooltipContent>
                          </TooltipRoot>
                        </TooltipProvider>
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
