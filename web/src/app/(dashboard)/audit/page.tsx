"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
  Clock,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type TimeRangeKey = "1h" | "24h" | "7d" | "custom";

const TIME_RANGE_LABELS: Record<TimeRangeKey, string> = {
  "1h": "Last 1 hour",
  "24h": "Last 24 hours",
  "7d": "Last 7 days",
  custom: "Custom",
};

function computeRange(
  key: TimeRangeKey,
  customStart?: string,
  customEnd?: string,
): { startTime?: string; endTime?: string } {
  if (key === "custom") {
    return {
      startTime: customStart ? new Date(customStart).toISOString() : undefined,
      endTime: customEnd ? new Date(customEnd).toISOString() : undefined,
    };
  }
  const now = new Date();
  const start = new Date(now);
  if (key === "1h") start.setHours(start.getHours() - 1);
  else if (key === "24h") start.setHours(start.getHours() - 24);
  else if (key === "7d") start.setDate(start.getDate() - 7);
  return { startTime: start.toISOString(), endTime: now.toISOString() };
}

function formatTokens(input: number, output: number): string {
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `${fmt(input)} / ${fmt(output)}`;
}

function formatCacheTokens(write: number, read: number): string | null {
  if (write === 0 && read === 0) return null;
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `c: ${fmt(write)} / ${fmt(read)}`;
}

export default function AuditPage() {
  const isMobile = useIsMobile();
  const [logs, setLogs] = useState<AuditLogItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: 1, pageSize: 20, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("24h");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [pageInputValue, setPageInputValue] = useState("1");

  const fetchLogs = useCallback(
    async (
      page: number,
      pageSize: number,
      query: string,
      range: TimeRangeKey,
      cs: string,
      ce: string,
    ) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(range, cs, ce);
        const rsp = await api.listAuditLogs({
          page,
          pageSize,
          query: query || undefined,
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
        }
      } catch (err) {
        toast.error(err instanceof Error ? err.message : "Failed to load audit logs");
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Initial data fetch on mount */
  useEffect(() => {
    fetchLogs(1, 20, "", "24h", "", "");
  }, [fetchLogs]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize)),
    [pageInfo],
  );

  const refresh = (page: number, pageSize?: number) =>
    fetchLogs(page, pageSize ?? pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd);

  const handleCopyTrace = (traceId: string) => {
    if (!traceId) return;
    navigator.clipboard.writeText(traceId).then(
      () => toast.success("TraceID copied"),
      () => toast.error("Copy failed"),
    );
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
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={<Button variant="outline" size="sm" className="gap-1.5" />}
                >
                  <Clock className="size-3.5" />
                  {TIME_RANGE_LABELS[timeRange]}
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  {(Object.keys(TIME_RANGE_LABELS) as TimeRangeKey[]).map((k) => (
                    <DropdownMenuItem
                      key={k}
                      onClick={() => {
                        setTimeRange(k);
                        if (k !== "custom") {
                          fetchLogs(1, pageInfo.pageSize, searchQuery, k, customStart, customEnd);
                        }
                      }}
                    >
                      {k === timeRange && <Check className="size-4" />}
                      <span className={k === timeRange ? "ml-0" : "ml-6"}>
                        {TIME_RANGE_LABELS[k]}
                      </span>
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
              {timeRange === "custom" && (
                <div className="flex items-center gap-2">
                  <input
                    type="datetime-local"
                    value={customStart}
                    onChange={(e) => setCustomStart(e.target.value)}
                    onBlur={() =>
                      fetchLogs(1, pageInfo.pageSize, searchQuery, "custom", customStart, customEnd)
                    }
                    className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
                  />
                  <span className="text-xs text-muted-foreground">–</span>
                  <input
                    type="datetime-local"
                    value={customEnd}
                    onChange={(e) => setCustomEnd(e.target.value)}
                    onBlur={() =>
                      fetchLogs(1, pageInfo.pageSize, searchQuery, "custom", customStart, customEnd)
                    }
                    className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
                  />
                </div>
              )}
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
                return (
                  <div key={log.id} className="rounded-lg border border-border bg-card p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{log.model || "—"}</p>
                        <p className="mt-0.5 truncate text-xs text-muted-foreground">
                          {log.userName || "—"} · {log.apiKeyName || "—"}
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
                    <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                      <span>{new Date(log.createdAt).toLocaleString()}</span>
                      <span>{formatTokens(log.inputTokens, log.outputTokens)}</span>
                      <span>{log.firstTokenLatencyMs}ms</span>
                      <span
                        className="cursor-pointer font-mono underline-offset-2 hover:underline"
                        onClick={() => handleCopyTrace(log.traceId)}
                        title="Click to copy full traceID"
                      >
                        {log.traceId.slice(-6) || "—"}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Model</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Tokens</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>UA</TableHead>
                  <TableHead>TraceID</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => {
                  const ok = log.upstreamStatusCode === 200;
                  const hasError = !!log.errorMessage;
                  const cacheInfo = formatCacheTokens(log.cacheCreationInputTokens, log.cacheReadInputTokens);
                  return (
                    <TableRow
                      key={log.id}
                      className={ok ? "" : "bg-destructive/5"}
                    >
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        <div>{new Date(log.createdAt).toLocaleTimeString()}</div>
                        <div className="text-xs text-muted-foreground/70">
                          {new Date(log.createdAt).toLocaleDateString(undefined, { month: "short", day: "numeric" })}
                        </div>
                      </TableCell>
                      <TableCell className="max-w-[180px] truncate">{log.model || "—"}</TableCell>
                      <TableCell>
                        <div className="text-sm">{log.userName || "—"}</div>
                        <div className="text-xs text-muted-foreground">
                          {log.apiKeyName || ""}{log.userEmail ? ` · ${log.userEmail}` : ""}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">{log.apiProvider || "—"}</div>
                        <div className="text-xs text-muted-foreground">
                          upstream: {log.upstreamProvider || "—"}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={ok ? "secondary" : "destructive"}
                          className="text-xs"
                          title={hasError ? log.errorMessage : undefined}
                        >
                          {log.upstreamStatusCode}
                        </Badge>
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        <div>{formatTokens(log.inputTokens, log.outputTokens)}</div>
                        {cacheInfo && (
                          <div className="text-xs text-muted-foreground">{cacheInfo}</div>
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        <div>{log.firstTokenLatencyMs}ms</div>
                        {log.streamDurationMs > 0 && (
                          <div className="text-xs text-muted-foreground/70">{(log.streamDurationMs / 1000).toFixed(1)}s</div>
                        )}
                      </TableCell>
                      <TableCell
                        className="max-w-[160px] truncate text-xs"
                        title={log.userAgent || undefined}
                      >
                        {log.userAgent || "—"}
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
