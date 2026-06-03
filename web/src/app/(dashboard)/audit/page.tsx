"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import type { AuditLogItem, PageInfo } from "@/lib/types";
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
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ScrollText,
  Search,
  ListFilter,
  Check,
  Info,
  ArrowUp,
  ArrowDown,
  Copy,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

type SortDir = "asc" | "desc";

interface SortState {
  field: string;
  dir: SortDir;
}

type StatusFilter = "all" | "failed" | "success";

const SORTABLE_COLUMNS: Record<string, string> = {
  createdAt: "created_at",
  inputTokens: "input_tokens",
  outputTokens: "output_tokens",
  firstTokenLatencyMs: "first_token_latency_ms",
  streamDurationMs: "stream_duration_ms",
};

const LATENCY_THRESHOLD_MS = 3000;

function formatTokens(input: number, output: number): string {
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `${fmt(input)}↑ / ${fmt(output)}↓`;
}

function formatCache(creation: number, read: number): string {
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  const parts: string[] = [];
  if (creation > 0) parts.push(`${fmt(creation)}↑`);
  if (read > 0) parts.push(`${fmt(read)}↓`);
  return parts.join(" / ") || "—";
}

function isError(log: AuditLogItem): boolean {
  return log.upstreamStatusCode !== 200;
}

function isHighLatency(log: AuditLogItem): boolean {
  return log.firstTokenLatencyMs > LATENCY_THRESHOLD_MS;
}

function shouldAutoExpand(log: AuditLogItem): boolean {
  return isError(log) || isHighLatency(log);
}

export default function AuditPage() {
  const isMobile = useIsMobile();
  const [logs, setLogs] = useState<AuditLogItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.audit.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.audit.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>("dashboard.audit.timeRange", "24h");
  const [customStart, setCustomStart] = usePersistentState("dashboard.audit.customStart", "");
  const [customEnd, setCustomEnd] = usePersistentState("dashboard.audit.customEnd", "");
  const [pageInputValue, setPageInputValue] = useState(String(persistedPage));
  const [sort, setSort] = useState<SortState>({ field: "created_at", dir: "desc" });
  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  const fetchLogs = useCallback(
    async (
      page: number,
      pageSize: number,
      query: string,
      range: TimeRangeKey,
      cs: string,
      ce: string,
      sortState: SortState,
    ) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(range, cs, ce);
        const rsp = await api.listAuditLogs({
          page,
          pageSize,
          query: query || undefined,
          sort: sortState.dir,
          sortField: sortState.field,
          startTime,
          endTime,
        });
        if (rsp.error) {
          toast.error(rsp.error.message ?? "Failed to load audit logs");
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPageInputValue(String(rsp.pageInfo.page));
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "Failed to load audit logs");
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Initial data fetch on mount */
  useEffect(() => {
    fetchLogs(persistedPage, persistedPageSize, "", "24h", "", "", { field: "created_at", dir: "desc" });
  }, [fetchLogs]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const filteredLogs = useMemo(() => {
    if (statusFilter === "all") return logs;
    if (statusFilter === "failed") return logs.filter((l) => isError(l));
    return logs.filter((l) => !isError(l));
  }, [logs, statusFilter]);

  const expanded = useMemo(() => {
    const result = new Set(expandedIds);
    for (const log of filteredLogs) {
      if (shouldAutoExpand(log)) result.add(log.id);
    }
    return result;
  }, [expandedIds, filteredLogs]);

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize)),
    [pageInfo],
  );

  const refresh = (page: number, pageSize?: number) =>
    fetchLogs(page, pageSize ?? pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, sort);

  const handleCopyTrace = (traceId: string) => {
    if (!traceId) return;
    navigator.clipboard.writeText(traceId).then(
      () => toast.success("TraceID copied"),
      () => toast.error("Copy failed"),
    );
  };

  const handleSort = (field: string) => {
    const newSort: SortState =
      sort.field === field
        ? { field, dir: sort.dir === "asc" ? "desc" : "asc" }
        : { field, dir: "desc" };
    setSort(newSort);
    fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, newSort);
  };

  const renderSortIcon = (field: string) => {
    if (sort.field !== field) return null;
    return sort.dir === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  const toggleExpand = (id: number) => {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
          Audit
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          Inspect model call records, latency, errors, and trace IDs.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">Audit Logs</CardTitle>
        </CardHeader>
        <CardContent>
          {/* 筛选区 */}
          <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex flex-wrap items-center gap-2">
              <div className="flex items-center gap-1 rounded-lg border border-border bg-muted/50 p-0.5">
                {(["all", "failed", "success"] as const).map((f) => (
                  <button
                    key={f}
                    type="button"
                    onClick={() => {
                      setStatusFilter(f);
                      setExpandedIds(new Set());
                    }}
                    className={`rounded-md px-3 py-1 text-xs font-medium transition-colors ${
                      statusFilter === f
                        ? "bg-background text-foreground shadow-sm"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {f === "all" ? "All" : f === "failed" ? "Failed" : "Success"}
                  </button>
                ))}
              </div>
              <TimeRangePicker
                value={timeRange}
                customStart={customStart}
                customEnd={customEnd}
                onChange={(key, cs, ce) => {
                  setTimeRange(key);
                  setCustomStart(cs);
                  setCustomEnd(ce);
                  if (key !== "custom") {
                    fetchLogs(1, pageInfo.pageSize, searchQuery, key, cs, ce, sort);
                  }
                }}
              />
            </div>
            <div className="relative w-full md:max-w-sm">
              <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search by traceID or model..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") refresh(1);
                }}
                className="pl-9"
              />
            </div>
          </div>

          {/* 列表 */}
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : filteredLogs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <ScrollText className="mb-3 size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">
                {logs.length > 0 ? "No matching logs in current filter" : "No audit logs in selected range"}
              </p>
            </div>
          ) : isMobile ? (
            <div className="space-y-3">
              {filteredLogs.map((log) => {
                const err = isError(log);
                const highLat = isHighLatency(log);
                const isExpanded = expanded.has(log.id);
                return (
                  <div
                    key={log.id}
                    className="overflow-hidden rounded-lg border border-border bg-card"
                  >
                    <button
                      type="button"
                      onClick={() => toggleExpand(log.id)}
                      className="flex w-full items-start gap-3 p-4 text-left"
                    >
                      <Badge
                        variant={err ? "destructive" : "secondary"}
                        className="mt-0.5 shrink-0 text-xs"
                        title={err ? log.errorMessage : undefined}
                      >
                        {log.upstreamStatusCode}
                      </Badge>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{log.model || "—"}</p>
                        <p className="mt-0.5 truncate text-xs text-muted-foreground">
                          {log.userName || "—"} · {log.apiKeyName || "—"}
                        </p>
                        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                          <span>{new Date(log.createdAt).toLocaleString()}</span>
                          <span>{formatTokens(log.inputTokens, log.outputTokens)}</span>
                          {highLat && (
                            <span className="font-medium text-destructive">
                              {log.firstTokenLatencyMs}ms
                            </span>
                          )}
                        </div>
                      </div>
                      <ChevronDown
                        className={`mt-0.5 size-4 shrink-0 text-muted-foreground transition-transform ${
                          isExpanded ? "rotate-180" : ""
                        }`}
                      />
                    </button>
                    {isExpanded && (
                      <div className="border-t border-border bg-muted/30 px-4 py-3">
                        <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
                          <div>
                            <span className="text-muted-foreground">Provider</span>
                            <p className="mt-0.5 text-foreground">
                              {log.apiProvider || "—"} · {log.upstreamProvider || "—"}
                            </p>
                          </div>
                          <div>
                            <span className="text-muted-foreground">Cache</span>
                            <p className="mt-0.5 text-foreground">
                              {formatCache(log.cacheCreationInputTokens, log.cacheReadInputTokens)}
                            </p>
                          </div>
                          <div>
                            <span className="text-muted-foreground">Latency</span>
                            <p
                              className={`mt-0.5 ${
                                highLat ? "font-medium text-destructive" : "text-foreground"
                              }`}
                            >
                              {log.firstTokenLatencyMs}ms
                              {log.streamDurationMs > 0 && (
                                <span className="ml-1">/ {log.streamDurationMs}ms</span>
                              )}
                            </p>
                          </div>
                          <div>
                            <span className="text-muted-foreground">TraceID</span>
                            <p className="mt-0.5">
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleCopyTrace(log.traceId);
                                }}
                                className="flex items-center gap-1 font-mono text-foreground hover:underline"
                              >
                                {log.traceId.slice(-12) || "—"}
                                <Copy className="size-3 text-muted-foreground" />
                              </button>
                            </p>
                          </div>
                        </div>
                        {err && log.errorMessage && (
                          <p className="mt-2 rounded bg-destructive/10 px-2 py-1 text-xs font-medium text-destructive">
                            {log.errorMessage}
                          </p>
                        )}
                        {log.userAgent && (
                          <p className="mt-2 flex items-center gap-1 truncate text-xs text-muted-foreground/70">
                            <Info className="size-3 shrink-0" />
                            <span className="truncate" title={log.userAgent}>
                              {log.userAgent}
                            </span>
                          </p>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[72px]">Status</TableHead>
                  <TableHead
                    className="cursor-pointer select-none whitespace-nowrap"
                    onClick={() => handleSort(SORTABLE_COLUMNS.createdAt)}
                  >
                    <span className="inline-flex items-center gap-1">
                      Time {renderSortIcon(SORTABLE_COLUMNS.createdAt)}
                    </span>
                  </TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>API Key</TableHead>
                  <TableHead
                    className="cursor-pointer select-none whitespace-nowrap"
                    onClick={() => handleSort(SORTABLE_COLUMNS.inputTokens)}
                  >
                    <span className="inline-flex items-center gap-1">
                      Tokens {renderSortIcon(SORTABLE_COLUMNS.inputTokens)}
                    </span>
                  </TableHead>
                  <TableHead className="w-[40px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredLogs.map((log) => {
                  const err = isError(log);
                  const isExpanded = expanded.has(log.id);
                  return (
                    <TableRow
                      key={log.id}
                      className={`cursor-pointer border-l-2 hover:bg-muted/50 ${
                        err ? "border-l-destructive" : "border-l-transparent"
                      }`}
                      onClick={() => toggleExpand(log.id)}
                    >
                      <TableCell>
                        <Badge
                          variant={err ? "destructive" : "secondary"}
                          className="text-xs"
                          title={err ? log.errorMessage : undefined}
                        >
                          {log.upstreamStatusCode}
                        </Badge>
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {new Date(log.createdAt).toLocaleString()}
                      </TableCell>
                      <TableCell className="max-w-[180px] truncate">
                        <div className="flex items-center gap-1">
                          <span className="truncate">{log.model || "—"}</span>
                          {log.userAgent && (
                            <span title={log.userAgent}>
                              <Info className="size-3 shrink-0 text-muted-foreground/50" />
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="max-w-[140px] truncate text-sm">
                        {log.userName || "—"}
                      </TableCell>
                      <TableCell className="max-w-[140px] truncate text-sm text-muted-foreground">
                        {log.apiKeyName || "—"}
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        {formatTokens(log.inputTokens, log.outputTokens)}
                      </TableCell>
                      <TableCell>
                        <ChevronDown
                          className={`size-4 text-muted-foreground transition-transform ${
                            isExpanded ? "rotate-180" : ""
                          }`}
                        />
                      </TableCell>
                    </TableRow>
                  );
                })}
                {/* 详情行单独渲染，不参与 TableRow 的点击事件 */}
                {filteredLogs
                  .filter((log) => expanded.has(log.id))
                  .map((log) => {
                    const err = isError(log);
                    const highLat = isHighLatency(log);
                    return (
                      <TableRow
                        key={`detail-${log.id}`}
                        className="border-l-2 border-l-transparent bg-muted/30 hover:bg-muted/30"
                      >
                        <TableCell colSpan={7} className="p-0">
                          <div className="grid grid-cols-4 gap-x-6 gap-y-2 px-4 py-3 text-sm">
                            <div>
                              <span className="text-xs text-muted-foreground">Provider</span>
                              <p className="mt-0.5 text-foreground">
                                {log.apiProvider || "—"} · {log.upstreamProvider || "—"}
                              </p>
                            </div>
                            <div>
                              <span className="text-xs text-muted-foreground">Cache</span>
                              <p className="mt-0.5 text-foreground">
                                {formatCache(
                                  log.cacheCreationInputTokens,
                                  log.cacheReadInputTokens,
                                )}
                              </p>
                            </div>
                            <div>
                              <span className="text-xs text-muted-foreground">Latency</span>
                              <p
                                className={`mt-0.5 ${
                                  highLat
                                    ? "font-medium text-destructive"
                                    : "text-foreground"
                                }`}
                              >
                                {log.firstTokenLatencyMs}ms
                                {log.streamDurationMs > 0 && (
                                  <span className="ml-1 text-xs">
                                    / {log.streamDurationMs}ms
                                  </span>
                                )}
                              </p>
                            </div>
                            <div>
                              <span className="text-xs text-muted-foreground">TraceID</span>
                              <p className="mt-0.5">
                                <button
                                  type="button"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    handleCopyTrace(log.traceId);
                                  }}
                                  className="flex items-center gap-1 font-mono text-xs text-foreground hover:underline"
                                >
                                  {log.traceId.slice(-12) || "—"}
                                  <Copy className="size-3 text-muted-foreground" />
                                </button>
                              </p>
                            </div>
                          </div>
                          {err && log.errorMessage && (
                            <div className="mx-4 mb-3 rounded bg-destructive/10 px-3 py-1.5">
                              <p className="text-xs font-medium text-destructive">
                                {log.errorMessage}
                              </p>
                            </div>
                          )}
                          {log.userAgent && (
                            <div className="mx-4 mb-3 flex items-center gap-1 truncate text-xs text-muted-foreground/70">
                              <Info className="size-3 shrink-0" />
                              <span className="truncate" title={log.userAgent}>
                                UA: {log.userAgent}
                              </span>
                            </div>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
              </TableBody>
            </Table>
          )}

          {/* 分页 */}
          {pageInfo.total > 0 && (
            <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
              <div className="hidden items-center gap-3 md:flex">
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={<Button variant="outline" size="sm" className="gap-1.5" />}
                  >
                    <ListFilter className="size-3.5" />
                    {pageInfo.pageSize} / page
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    {[20, 50, 100].map((size) => (
                      <DropdownMenuItem key={size} onClick={() => refresh(1, size)}>
                        {size === pageInfo.pageSize && <Check className="size-4" />}
                        <span className={size === pageInfo.pageSize ? "ml-0" : "ml-6"}>
                          {size} per page
                        </span>
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
                <p className="hidden text-sm text-muted-foreground md:block">
                  {pageInfo.total} log{pageInfo.total !== 1 ? "s" : ""} total
                </p>
              </div>
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
  );
}
