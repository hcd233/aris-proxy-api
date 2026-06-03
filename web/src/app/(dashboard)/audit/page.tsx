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
  ChevronLeft,
  ChevronRight,
  ScrollText,
  Search,
  ListFilter,
  Check,
  Info,
  ArrowUp,
  ArrowDown,
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

const SORTABLE_COLUMNS: Record<string, string> = {
  createdAt: "created_at",
  inputTokens: "input_tokens",
  outputTokens: "output_tokens",
  firstTokenLatencyMs: "first_token_latency_ms",
  streamDurationMs: "stream_duration_ms",
};

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
          ) : logs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <ScrollText className="mb-3 size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">No audit logs in selected range</p>
            </div>
          ) : isMobile ? (
            <div className="space-y-3">
              {logs.map((log) => {
                const ok = log.upstreamStatusCode === 200;
                const hasCache =
                  (log.cacheCreationInputTokens > 0) || (log.cacheReadInputTokens > 0);
                return (
                  <div key={log.id} className="rounded-lg border border-border bg-card p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{log.model || "—"}</p>
                        <p className="mt-0.5 truncate text-xs text-muted-foreground">
                          {log.apiProvider || "—"} · {log.upstreamProvider || ""}
                        </p>
                      </div>
                      <Badge
                        variant={ok ? "secondary" : "destructive"}
                        className="shrink-0 text-xs"
                        title={ok ? undefined : log.errorMessage}
                      >
                        {log.upstreamStatusCode}
                      </Badge>
                    </div>
                    <p className="mt-1 truncate text-xs text-muted-foreground">
                      {log.userName || "—"} · {log.apiKeyName || "—"}
                    </p>
                    <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                      <span>{new Date(log.createdAt).toLocaleString()}</span>
                      <span>{formatTokens(log.inputTokens, log.outputTokens)}</span>
                      {hasCache && <span>cache {formatCache(log.cacheCreationInputTokens, log.cacheReadInputTokens)}</span>}
                      <span>{log.firstTokenLatencyMs}ms</span>
                      <span
                        className="cursor-pointer font-mono underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                        title="Click to copy full traceID"
                      >
                        {log.traceId.slice(-6) || "—"}
                      </span>
                    </div>
                    {(log.userAgent) && (
                      <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground/70">
                          <span className="truncate" title={log.userAgent}>
                            UA: {log.userAgent}
                          </span>
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
                  <TableHead
                    className="cursor-pointer select-none whitespace-nowrap"
                    onClick={() => handleSort(SORTABLE_COLUMNS.createdAt)}
                  >
                    <span className="inline-flex items-center gap-1">Time {renderSortIcon(SORTABLE_COLUMNS.createdAt)}</span>
                  </TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>API Key</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead
                    className="cursor-pointer select-none whitespace-nowrap"
                    onClick={() => handleSort(SORTABLE_COLUMNS.inputTokens)}
                  >
                    <span className="inline-flex items-center gap-1">Tokens {renderSortIcon(SORTABLE_COLUMNS.inputTokens)}</span>
                  </TableHead>
                  <TableHead>Cache</TableHead>
                  <TableHead
                    className="cursor-pointer select-none whitespace-nowrap"
                    onClick={() => handleSort(SORTABLE_COLUMNS.firstTokenLatencyMs)}
                  >
                    <span className="inline-flex items-center gap-1">Latency {renderSortIcon(SORTABLE_COLUMNS.firstTokenLatencyMs)}</span>
                  </TableHead>
                  <TableHead>TraceID</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => {
                  const ok = log.upstreamStatusCode === 200;
                  return (
                    <TableRow key={log.id}>
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
                      <TableCell>
                        <div className="text-sm">{log.apiProvider || "—"}</div>
                        <div className="text-xs text-muted-foreground">{log.upstreamProvider || ""}</div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">{log.userName || "—"}</div>
                        <div className="text-xs text-muted-foreground">{log.userEmail || ""}</div>
                      </TableCell>
                      <TableCell className="max-w-[140px] truncate">
                        {log.apiKeyName || "—"}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={ok ? "secondary" : "destructive"}
                          className="text-xs"
                          title={ok ? undefined : log.errorMessage}
                        >
                          {log.upstreamStatusCode}
                        </Badge>
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        {formatTokens(log.inputTokens, log.outputTokens)}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatCache(log.cacheCreationInputTokens, log.cacheReadInputTokens)}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {log.firstTokenLatencyMs}ms
                        {log.streamDurationMs > 0 && (
                          <span className="ml-1 text-xs">/ {log.streamDurationMs}ms</span>
                        )}
                      </TableCell>
                      <TableCell
                        className="cursor-pointer font-mono text-xs underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                        title="Click to copy full traceID"
                      >
                        {log.traceId.slice(-6) || "—"}
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
