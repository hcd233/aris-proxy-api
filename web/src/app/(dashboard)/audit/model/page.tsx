"use client";

import { useCallback, useEffect, useState } from "react";
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
import { PaginationBar } from "@/components/pagination-bar";
import {
  ScrollText,
  Search,
  X,
} from "lucide-react";
import { ProviderIcon } from "@/components/provider-icon";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { MultiSelectPill } from "@/components/ui/multi-select-pill";

function formatTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function formatTokens(input: number, output: number): string {
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `${fmt(input)} / ${fmt(output)}`;
}

function formatCacheTokens(write: number, read: number): string | null {
  if (write === 0 && read === 0) return null;
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  return `c: ${fmt(read)} / ${fmt(write)}`;
}

function formatProtocol(protocol: string): string {
  if (!protocol) return "—";
  const labels: Record<string, string> = {
    "openai-chat-completion": "Chat Completions",
    "openai-response": "Response",
    "anthropic-message": "Messages",
  };
  return labels[protocol] || protocol;
}

function formatCompression(tokens: number, strategies?: string[]): string | null {
  if (tokens <= 0) return null;
  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  const label = `C: ${fmt(tokens)}`;
  if (!strategies?.length) return label;
  return `${label} (${strategies.join(", ")})`;
}

function buildAuditFilter(user: string[], model: string[], status: string[]): string | undefined {
  const parts: string[] = [];
  if (user.length) parts.push(`user:${user.join("|")}`);
  if (model.length) parts.push(`model:${model.join("|")}`);
  if (status.length) parts.push(`status:${status.join("|")}`);
  return parts.length > 0 ? parts.join(" ") : undefined;
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
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [filterUser, setFilterUser] = useState<string[]>([]);
  const [filterModel, setFilterModel] = useState<string[]>([]);
  const [filterStatus, setFilterStatus] = useState<string[]>([]);
  const [userOptions, setUserOptions] = useState<string[]>([]);
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const [statusOptions, setStatusOptions] = useState<string[]>([]);

  const fetchLogs = useCallback(
    async (
      page: number,
      pageSize: number,
      query: string,
      range: TimeRangeKey,
      cs: string,
      ce: string,
      user: string[],
      model: string[],
      status: string[],
    ) => {
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(range, cs, ce);
        const filter = buildAuditFilter(user, model, status);
        const rsp = await api.listAuditLogs({
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
          toast.error(rsp.error.message ?? "Failed to load audit logs");
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) setPageInfo(rsp.pageInfo);
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
    fetchLogs(1, 20, "", "24h", "", "", [], [], []);
  }, [fetchLogs]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const fetchOptions = useCallback(async (range: TimeRangeKey, cs: string, ce: string) => {
    const { startTime, endTime } = computeRange(range, cs, ce);
    const params = { startTime, endTime };
    try {
      const [userRsp, modelRsp, statusRsp] = await Promise.all([
        api.listAuditOptions({ field: "user", ...params }),
        api.listAuditOptions({ field: "model", ...params }),
        api.listAuditOptions({ field: "status", ...params }),
      ]);
      if (!userRsp.error && userRsp.items) setUserOptions(userRsp.items);
      if (!modelRsp.error && modelRsp.items) setModelOptions(modelRsp.items);
      if (!statusRsp.error && statusRsp.items) setStatusOptions(statusRsp.items);
    } catch (err) {
      console.error("Failed to load audit options:", err);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch filter options when the time range changes */
  useEffect(() => {
    fetchOptions(timeRange, customStart, customEnd);
  }, [timeRange, customStart, customEnd, fetchOptions]);
  /* eslint-enable react-hooks/set-state-in-effect */


  const refresh = (page: number, pageSize?: number) =>
    fetchLogs(page, pageSize ?? pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterUser, filterModel, filterStatus);

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
            Model Call Audit
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
                  fetchLogs(1, pageInfo.pageSize, searchQuery, key, cs, ce, filterUser, filterModel, filterStatus);
                }}
              />
              <MultiSelectPill
                label="User"
                options={userOptions}
                value={filterUser}
                onChange={(v) => {
                  setFilterUser(v);
                  fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, v, filterModel, filterStatus);
                }}
              />
              <MultiSelectPill
                label="Model"
                options={modelOptions}
                value={filterModel}
                onChange={(v) => {
                  setFilterModel(v);
                  fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterUser, v, filterStatus);
                }}
              />
              <MultiSelectPill
                label="Status"
                options={statusOptions}
                value={filterStatus}
                onChange={(v) => {
                  setFilterStatus(v);
                  fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, filterUser, filterModel, v);
                }}
              />
              {(filterUser.length > 0 || filterModel.length > 0 || filterStatus.length > 0) && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1 text-muted-foreground"
                  onClick={() => {
                    setFilterUser([]);
                    setFilterModel([]);
                    setFilterStatus([]);
                    fetchLogs(1, pageInfo.pageSize, searchQuery, timeRange, customStart, customEnd, [], [], []);
                  }}
                >
                  <X size={14} />
                  Clear filters
                </Button>
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
                const hasError = !!log.errorMessage;
                const isExpanded = expandedId === log.id;
                const cacheInfo = formatCacheTokens(log.cacheCreationInputTokens, log.cacheReadInputTokens);

                return (
                  <div
                    key={log.id}
                    className={`rounded-lg border border-border bg-card ${ok ? "" : "bg-destructive/5"}`}
                  >
                    <div
                      className="cursor-pointer p-4"
                      onClick={() => setExpandedId(isExpanded ? null : log.id)}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-medium">
                            <span className="inline-flex items-center gap-1.5">
                              <ProviderIcon protocol={log.model} size={14} />
                              {log.model || "—"}
                            </span>
                          </p>
                          <p className="mt-0.5 truncate text-xs text-muted-foreground">
                            {log.userName || "—"} · {log.apiKeyName || "—"}
                          </p>
                        </div>
                        <div className="shrink-0 text-right">
                          <Badge
                            variant={ok ? "secondary" : "destructive"}
                            className="text-xs"
                          >
                            {log.upstreamStatusCode}
                          </Badge>
                          {hasError && (
                            <p className="mt-1 max-w-[200px] truncate text-xs text-destructive">
                              {log.errorMessage}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                        <span>{formatTokens(log.inputTokens, log.outputTokens)}</span>
                        <span>I: {log.firstTokenLatencyMs}ms</span>
                        {log.streamDurationMs > 0 && (
                          <span>O: {(log.streamDurationMs / 1000).toFixed(1)}s</span>
                        )}
                        {cacheInfo && <span>{cacheInfo}</span>}
                        {log.compressionEnabled && log.compressedTokens > 0 && (
                          <span title={log.compressionStrategies?.length ? `Strategies: ${log.compressionStrategies.join(", ")}` : undefined}>
                            {formatCompression(log.compressedTokens, log.compressionStrategies)}
                          </span>
                        )}
                        <span
                          className="cursor-pointer font-mono underline-offset-2 hover:underline"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleCopyTrace(log.traceId);
                          }}
                          title="Click to copy full traceID"
                        >
                          {log.traceId.slice(-6) || "—"}
                        </span>
                      </div>
                      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground/70">
                        <span>{formatTime(log.createdAt)}</span>
                        <span
                          className="inline-block transition-transform duration-200 motion-reduce:transition-none"
                          style={{ transform: isExpanded ? "rotate(180deg)" : "rotate(0deg)" }}
                        >
                          ▾
                        </span>
                      </div>
                    </div>

                    <div
                      className="grid overflow-hidden transition-all duration-[250ms] ease-out motion-reduce:transition-none"
                      style={{ gridTemplateRows: isExpanded ? "1fr" : "0fr" }}
                    >
                      <div className="min-h-0">
                        <div className="border-t border-border px-4 pb-4 pt-3">
                          {hasError && (
                            <div className="mb-3 rounded-md bg-destructive/10 px-3 py-2 text-xs">
                              <span className="font-medium text-destructive">Error: </span>
                              <span className="text-destructive">{log.errorMessage}</span>
                            </div>
                          )}

                          <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
                            <div>
                              <span className="text-muted-foreground">Input Tokens</span>
                              <p>{log.inputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Output Tokens</span>
                              <p>{log.outputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Cache Read</span>
                              <p>{log.cacheReadInputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Cache Creation</span>
                              <p>{log.cacheCreationInputTokens.toLocaleString()}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">I (First Token)</span>
                              <p>{log.firstTokenLatencyMs}ms</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">O (Stream Duration)</span>
                              <p>{log.streamDurationMs > 0 ? `${(log.streamDurationMs / 1000).toFixed(1)}s` : "—"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Upstream</span>
                              <p className="flex items-center gap-1.5">
                                <ProviderIcon protocol={log.upstreamProtocol} size={14} />
                                {formatProtocol(log.upstreamProtocol)}
                              </p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">Endpoint</span>
                              <p>{log.endpoint || "—"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">User</span>
                              <p>{log.userName || "—"}</p>
                            </div>
                            <div>
                              <span className="text-muted-foreground">API Protocol</span>
                              <p className="flex items-center gap-1.5">
                                <ProviderIcon protocol={log.apiProtocol} size={14} />
                                {formatProtocol(log.apiProtocol)}
                              </p>
                            </div>
                            {log.compressionEnabled && log.compressedTokens > 0 && (
                              <>
                                <div>
                                  <span className="text-muted-foreground">Compression</span>
                                  <p>{log.compressedTokens.toLocaleString()} tokens saved</p>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Strategies</span>
                                  <p>{log.compressionStrategies?.join(", ") || "—"}</p>
                                </div>
                              </>
                            )}
                          </div>

                          <div className="mt-3 border-t border-border pt-2 text-xs">
                            <span className="text-muted-foreground">UA: </span>
                            <span className="break-all">{log.userAgent || "—"}</span>
                          </div>

                          <div className="mt-2 flex items-center justify-between border-t border-border pt-2 text-xs">
                            <span className="text-muted-foreground">
                              {formatTime(log.createdAt)}
                            </span>
                            <span
                              className="cursor-pointer font-mono text-muted-foreground underline-offset-2 hover:underline"
                              onClick={() => handleCopyTrace(log.traceId)}
                            >
                              Copy TraceID
                            </span>
                          </div>
                        </div>
                      </div>
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
                  <TableHead>Endpoint</TableHead>
                  <TableHead>Protocol</TableHead>
                  <TableHead>User</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Tokens</TableHead>
                  <TableHead>Compression</TableHead>
                  <TableHead>Latency</TableHead>
                  <TableHead>UserAgent</TableHead>
                  <TableHead>TraceID</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => {
                  const ok = log.upstreamStatusCode === 200;
                  const hasError = !!log.errorMessage;
                  const cacheInfo = formatCacheTokens(log.cacheCreationInputTokens, log.cacheReadInputTokens);
                  const uaShort = log.userAgent ? log.userAgent.slice(0, 30) + (log.userAgent.length > 30 ? "…" : "") : "—";
                  return (
                    <TableRow
                      key={log.id}
                      className={ok ? "" : "bg-destructive/5"}
                    >
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {formatTime(log.createdAt)}
                      </TableCell>
                      <TableCell className="max-w-[180px] truncate">
                        <span className="inline-flex items-center gap-1.5">
                          <ProviderIcon protocol={log.model} size={14} />
                          {log.model || "—"}
                        </span>
                      </TableCell>
                      <TableCell className="max-w-[140px] truncate text-muted-foreground">{log.endpoint || "—"}</TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        <div className="flex items-center gap-1.5 text-xs">
                          <ProviderIcon protocol={log.apiProtocol} size={14} />
                          {formatProtocol(log.apiProtocol)}
                        </div>
                        <div className="flex items-center gap-1.5 text-xs text-muted-foreground/70">
                          <ProviderIcon protocol={log.upstreamProtocol} size={14} />
                          {formatProtocol(log.upstreamProtocol)}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm">{log.userName || "—"}</div>
                        <div className="text-xs text-muted-foreground">
                          {log.apiKeyName || "—"}
                        </div>
                      </TableCell>
                      <TableCell>
                        {!ok && hasError ? (
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <button type="button">
                                    <Badge variant="destructive" className="text-xs">
                                      {log.upstreamStatusCode}
                                    </Badge>
                                  </button>
                                }
                              />
                              <TooltipContent side="top" className="max-w-xs">
                                <span>{log.errorMessage}</span>
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        ) : (
                          <Badge
                            variant={ok ? "secondary" : "destructive"}
                            className="text-xs"
                          >
                            {log.upstreamStatusCode}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        <div>{formatTokens(log.inputTokens, log.outputTokens)}</div>
                        {cacheInfo && (
                          <div className="text-xs text-muted-foreground">{cacheInfo}</div>
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-xs">
                        {log.compressionEnabled && log.compressedTokens > 0 ? (
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger render={
                                <button type="button" className="cursor-default text-muted-foreground">
                                  {formatCompression(log.compressedTokens, log.compressionStrategies)}
                                </button>
                              } />
                              <TooltipContent side="top" className="max-w-xs">
                                <span>Compressed {log.compressedTokens.toLocaleString()} tokens</span>
                                {log.compressionStrategies?.length ? (
                                  <span className="block text-xs opacity-70">
                                    Strategies: {log.compressionStrategies.join(", ")}
                                  </span>
                                ) : null}
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        ) : (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        <div className="text-xs">I: {log.firstTokenLatencyMs}ms</div>
                        {log.streamDurationMs > 0 && (
                          <div className="text-xs">O: {(log.streamDurationMs / 1000).toFixed(1)}s</div>
                        )}
                      </TableCell>
                      <TableCell>
                        {log.userAgent ? (
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <button type="button" className="max-w-[80px] cursor-default truncate text-xs text-muted-foreground">
                                    {uaShort}
                                  </button>
                                }
                              />
                              <TooltipContent side="top" className="max-w-xs">
                                <span className="break-all">{log.userAgent}</span>
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        ) : (
                          <span className="text-xs text-muted-foreground">—</span>
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

          <PaginationBar
            pageInfo={pageInfo}
            onChange={(page, pageSize) => refresh(page, pageSize)}
            totalLabel="logs"
          />
        </CardContent>
      </Card>
    </div>
  );
}
