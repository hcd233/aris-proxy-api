"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { AuditLogItem, PageInfo } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Item, ItemGroup } from "@/components/ui/item";
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
import { ScrollText } from "lucide-react";
import { ProviderIcon } from "@/components/provider-icon";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import { useRequestSeq } from "@/hooks/use-request-seq";
import { useI18n } from "@/lib/i18n";
import { PermissionGuard } from "@/components/permission-guard";
import { formatCost } from "@/lib/money";
import { cn, formatDateTime, formatDurationMs, formatInteger, formatTokenCount } from "@/lib/utils";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef, FilterBarQueryParams } from "@/components/filter-bar/types";
import { copyTextToClipboard } from "@/lib/clipboard";

function formatTokenPair(input: number, output: number): string {
  return `${formatTokenCount(input)} / ${formatTokenCount(output)}`;
}

function formatCacheTokens(write: number, read: number): string | null {
  if (write === 0 && read === 0) return null;
  return `ca: ${formatTokenCount(read)} / ${formatTokenCount(write)}`;
}

function formatProtocol(protocol: string): string {
  if (!protocol) return "—";
  const labels: Record<string, string> = {
    "openai-chat-completion": "Chat Completions",
    "openai-response": "Response",
    "anthropic-message": "Messages",
    "openai-decision": "Decision",
  };
  return labels[protocol] || protocol;
}

/**
 * 上游状态码的筛选项文案：选项列表里只有裸码（200/429）看不出含义，
 * 补一个语义前缀；码本身保留在括号里，方便与表格状态徽章对得上。
 */
function formatStatusCode(code: string, t: (key: string, fallback?: string) => string): string {
  const n = Number(code);
  if (!Number.isFinite(n)) return code;
  const label =
    n < 300
      ? t("audit.status_success")
      : n === 429
        ? t("audit.status_rate_limited")
        : n < 500
          ? t("audit.status_client_error")
          : t("audit.status_upstream_error");
  return `${label} (${code})`;
}

export default function AuditPage() {
  const isMobile = useIsMobile();
  const { t, locale } = useI18n();
  const [logs, setLogs] = useState<AuditLogItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: 1, pageSize: 20, total: 0 });
  const [loading, setLoading] = useState(true);
  const [timeRange, setTimeRange] = useState<TimeRangeKey>("24h");
  const [customStart, setCustomStart] = useState("");
  const [customEnd, setCustomEnd] = useState("");
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const fetchOptionsFor = useCallback(
    (field: "user" | "model" | "status" | "ua") => async () => {
      const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.listAuditOptions({ field, startTime, endTime });
      return rsp.items ?? [];
    },
    [timeRange, customStart, customEnd],
  );

  const facets = useMemo<FacetDef[]>(
    () => [
      { key: "user", label: t("audit.filter_user"), options: fetchOptionsFor("user") },
      { key: "model", label: t("audit.filter_model"), options: fetchOptionsFor("model") },
      {
        key: "status",
        label: t("audit.filter_status"),
        options: fetchOptionsFor("status"),
        formatValue: (v) => formatStatusCode(v, t),
      },
      { key: "ua", label: t("audit.filter_ua"), options: fetchOptionsFor("ua") },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, fetchOptionsFor],
  );

  const filterBar = useFilterBar({
    persistKey: "dashboard.auditModel",
    facets,
    freeTextPlaceholder: t("audit.search_placeholder"),
    optionsCacheKey: `${timeRange}:${customStart}:${customEnd}`,
  });
  const { queryParams } = filterBar;

  interface AuditLogsQuery {
    page: number;
    pageSize: number;
    range: TimeRangeKey;
    cs: string;
    ce: string;
    qp: FilterBarQueryParams;
  }

  // 竞态守卫：筛选/翻页连点时慢响应不得覆盖新响应
  const requestSeq = useRequestSeq();
  const fetchLogs = useCallback(
    async (q: AuditLogsQuery) => {
      const seq = requestSeq.begin();
      setLoading(true);
      try {
        const { startTime, endTime } = computeRange(q.range, q.cs, q.ce);
        const rsp = await api.listAuditLogs({
          page: q.page,
          pageSize: q.pageSize,
          query: q.qp.freeText || undefined,
          sort: "desc",
          sortField: "created_at",
          startTime,
          endTime,
          filter: q.qp.filter,
        });
        if (!requestSeq.isCurrent(seq)) return;
        if (rsp.error) {
          showErrorToast(rsp.error, { title: t("common.error") });
          return;
        }
        setLogs(rsp.logs ?? []);
        if (rsp.pageInfo) setPageInfo(rsp.pageInfo);
      } catch (err) {
        if (!requestSeq.isCurrent(seq)) return;
        showErrorToast(err, { title: t("common.error") });
      } finally {
        if (requestSeq.isCurrent(seq)) setLoading(false);
      }
    },
    [t, requestSeq],
  );

  const currentQuery = (): Omit<AuditLogsQuery, "page" | "pageSize"> => ({
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
      ok ? toast.success(t("audit.trace_copied")) : toast.error(t("audit.copy_failed")),
    );
  };

  return (
    <PermissionGuard module="audit">
      <div className="space-y-8">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            {t("audit.model_page_title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{t("audit.model_page_subtitle")}</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("audit.logs_title")}</CardTitle>
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
                placeholder={t("audit.search_placeholder")}
              />
            </div>
            {filterBar.tokens.length > 0 && (
              <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
              </p>
            )}

            {/* List */}
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 5 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : logs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <ScrollText className="mb-3 size-10 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">{t("audit.no_logs_range")}</p>
              </div>
            ) : isMobile ? (
              <ItemGroup className="gap-3">
                {logs.map((log) => {
                  const ok = log.upstreamStatusCode === 200;
                  const hasError = log.isError;
                  const isExpanded = expandedId === log.id;
                  const cacheInfo = formatCacheTokens(
                    log.cacheCreationInputTokens,
                    log.cacheReadInputTokens,
                  );

                  return (
                    <Item
                      key={log.id}
                      variant="outline"
                      role="listitem"
                      className={cn(
                        "flex-col flex-nowrap items-stretch gap-0 p-0",
                        !ok && "bg-destructive/5",
                      )}
                    >
                      <div
                        className="cursor-pointer p-4"
                        onClick={() => setExpandedId(isExpanded ? null : log.id)}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <p className="truncate text-sm font-medium">
                                      <span className="inline-flex items-center gap-1.5">
                                        <ProviderIcon protocol={log.modelId} size={14} />
                                        {log.modelId || "—"}
                                      </span>
                                    </p>
                                  }
                                />
                                <TooltipContent side="top" className="max-w-xs break-all">
                                  {log.modelId || "—"}
                                </TooltipContent>
                              </TooltipRoot>
                            </TooltipProvider>
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <p className="mt-0.5 truncate text-xs text-muted-foreground">
                                      {log.userName || "—"} · {log.apiKeyName || "—"}
                                    </p>
                                  }
                                />
                                <TooltipContent side="top" className="max-w-xs break-all">
                                  {log.userName || "—"} · {log.apiKeyName || "—"}
                                </TooltipContent>
                              </TooltipRoot>
                            </TooltipProvider>
                          </div>
                          <div className="shrink-0 text-right">
                            <Badge variant={ok ? "secondary" : "destructive"} className="text-xs">
                              {log.upstreamStatusCode}
                            </Badge>
                            {log.errorMessage && (
                              <TooltipProvider>
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <p
                                        className={cn(
                                          "mt-1 max-w-[200px] truncate text-xs",
                                          hasError ? "text-destructive" : "text-muted-foreground",
                                        )}
                                      >
                                        {log.errorMessage}
                                      </p>
                                    }
                                  />
                                  <TooltipContent side="top" className="max-w-xs break-all">
                                    {log.errorMessage}
                                  </TooltipContent>
                                </TooltipRoot>
                              </TooltipProvider>
                            )}
                          </div>
                        </div>
                        <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                          <span>{formatTokenPair(log.inputTokens, log.outputTokens)}</span>
                          <span>I: {formatDurationMs(log.firstTokenLatencyMs)}</span>
                          {log.streamDurationMs > 0 && (
                            <span>O: {formatDurationMs(log.streamDurationMs)}</span>
                          )}
                          {cacheInfo && <span>{cacheInfo}</span>}
                          <span>{formatCost(log.cost, log.pricingCurrency)}</span>
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <span
                                    className="cursor-pointer font-mono underline-offset-2 hover:underline"
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      handleCopyTrace(log.traceId);
                                    }}
                                  >
                                    {log.traceId.slice(-6) || "—"}
                                  </span>
                                }
                              />
                              <TooltipContent side="top">
                                {t("audit.copy_traceid_title")}
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        </div>
                        <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground/70">
                          <span>{formatDateTime(log.createdAt)}</span>
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
                              <Alert variant="destructive" className="mb-3">
                                <AlertTitle>{t("audit.error_label")}</AlertTitle>
                                <AlertDescription className="break-words">
                                  {log.errorMessage}
                                </AlertDescription>
                              </Alert>
                            )}
                            {!hasError && log.errorMessage && (
                              <p className="mb-3 break-words text-xs text-muted-foreground">
                                {log.errorMessage}
                              </p>
                            )}

                            <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.input_tokens_label")}
                                </span>
                                <p>{formatInteger(log.inputTokens)}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.output_tokens_label")}
                                </span>
                                <p>{formatInteger(log.outputTokens)}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.cache_read_colon")}
                                </span>
                                <p>{formatInteger(log.cacheReadInputTokens)}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.cache_creation")}
                                </span>
                                <p>{formatInteger(log.cacheCreationInputTokens)}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.first_token_label")}
                                </span>
                                <p>{formatDurationMs(log.firstTokenLatencyMs)}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.stream_label")}
                                </span>
                                <p>
                                  {log.streamDurationMs > 0
                                    ? formatDurationMs(log.streamDurationMs)
                                    : "—"}
                                </p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.cost.column")}
                                </span>
                                <p>{formatCost(log.cost, log.pricingCurrency)}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">{t("audit.upstream")}</span>
                                <p className="flex items-center gap-1.5">
                                  <ProviderIcon protocol={log.upstreamProtocol} size={14} />
                                  {formatProtocol(log.upstreamProtocol)}
                                </p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.endpoint_name")}
                                </span>
                                <p>{log.endpoint || "—"}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.user_name")}
                                </span>
                                <p>{log.userName || "—"}</p>
                              </div>
                              <div>
                                <span className="text-muted-foreground">
                                  {t("audit.api_protocol")}
                                </span>
                                <p className="flex items-center gap-1.5">
                                  <ProviderIcon protocol={log.apiProtocol} size={14} />
                                  {formatProtocol(log.apiProtocol)}
                                </p>
                              </div>
                            </div>

                            <div className="mt-3 border-t border-border pt-2 text-xs">
                              <span className="text-muted-foreground">{t("audit.ua_label")}</span>
                              <span className="break-all">{log.userAgent || "—"}</span>
                            </div>

                            <div className="mt-2 flex items-center justify-between border-t border-border pt-2 text-xs">
                              <span className="text-muted-foreground">
                                {formatDateTime(log.createdAt)}
                              </span>
                              <span
                                className="cursor-pointer font-mono text-muted-foreground underline-offset-2 hover:underline"
                                onClick={() => handleCopyTrace(log.traceId)}
                              >
                                {t("audit.copy_traceid")}
                              </span>
                            </div>
                          </div>
                        </div>
                      </div>
                    </Item>
                  );
                })}
              </ItemGroup>
            ) : (
              <Table fit>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[10.5rem]">{t("audit.time")}</TableHead>
                    {/* 主列不声明宽度：fixed 布局把剩余空间全给模型列（端点/协议并入次行） */}
                    <TableHead>{t("audit.filter_model")}</TableHead>
                    <TableHead className="w-[4rem]">{t("audit.user_name")}</TableHead>
                    <TableHead className="w-[4rem]">{t("audit.filter_status")}</TableHead>
                    {/* 8.5rem 是 token 列的下限：cache 行 "ca: 4.1K / 113.5K" 再窄就被裁掉 */}
                    <TableHead className="w-[8.5rem]">{t("audit.tokens")}</TableHead>
                    <TableHead className="w-[5rem]">{t("audit.latency")}</TableHead>
                    <TableHead className="w-[5rem]">{t("audit.cost.column")}</TableHead>
                    <TableHead className="w-[5rem]">{t("audit.useragent")}</TableHead>
                    <TableHead className="w-[4.5rem]">{t("audit.copy_traceid")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((log) => {
                    const ok = log.upstreamStatusCode === 200;
                    const hasError = log.isError;
                    const cacheInfo = formatCacheTokens(
                      log.cacheCreationInputTokens,
                      log.cacheReadInputTokens,
                    );
                    const uaShort = log.userAgent
                      ? log.userAgent.slice(0, 30) + (log.userAgent.length > 30 ? "…" : "")
                      : "—";
                    // 模型列次行：端点 + 协议；客户端协议与上游协议不同时用 → 表示协议转换
                    const protocolText =
                      log.apiProtocol &&
                      log.upstreamProtocol &&
                      log.apiProtocol !== log.upstreamProtocol
                        ? `${formatProtocol(log.apiProtocol)} → ${formatProtocol(log.upstreamProtocol)}`
                        : formatProtocol(log.apiProtocol || log.upstreamProtocol);
                    const metaText = [log.endpoint, protocolText]
                      .filter((v) => v && v !== "—")
                      .join(" · ");
                    return (
                      <TableRow key={log.id} className={ok ? "" : "bg-destructive/5"}>
                        <TableCell className="whitespace-nowrap text-xs tabular-nums text-muted-foreground">
                          {formatDateTime(log.createdAt)}
                        </TableCell>
                        <TableCell>
                          {/* 端点与协议并入模型列次行：原「端点」「协议」两列的信息一行说清，表格少两列 */}
                          <div className="flex min-w-0 flex-col gap-0.5">
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <button
                                      type="button"
                                      className="flex min-w-0 cursor-default items-center gap-1.5"
                                    >
                                      <ProviderIcon
                                        protocol={log.modelId}
                                        size={14}
                                        className="shrink-0"
                                      />
                                      <span className="min-w-0 truncate">{log.modelId || "—"}</span>
                                    </button>
                                  }
                                />
                                <TooltipContent side="top" className="max-w-xs">
                                  <span className="break-all">{log.modelId}</span>
                                </TooltipContent>
                              </TooltipRoot>
                            </TooltipProvider>
                            {metaText && (
                              <TooltipProvider>
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span className="min-w-0 truncate text-xs text-muted-foreground">
                                        {metaText}
                                      </span>
                                    }
                                  />
                                  <TooltipContent side="top" className="max-w-xs break-all">
                                    {metaText}
                                  </TooltipContent>
                                </TooltipRoot>
                              </TooltipProvider>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="text-sm">
                            {log.userName ? (
                              <TooltipProvider>
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span className="block max-w-[14ch] truncate">
                                        {log.userName}
                                      </span>
                                    }
                                  />
                                  <TooltipContent
                                    side="top"
                                    align="start"
                                    className="max-w-xs break-all"
                                  >
                                    {log.userName}
                                  </TooltipContent>
                                </TooltipRoot>
                              </TooltipProvider>
                            ) : (
                              "—"
                            )}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {log.apiKeyName ? (
                              <TooltipProvider>
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span className="block max-w-[14ch] truncate">
                                        {log.apiKeyName}
                                      </span>
                                    }
                                  />
                                  <TooltipContent
                                    side="top"
                                    align="start"
                                    className="max-w-xs break-all"
                                  >
                                    {log.apiKeyName}
                                  </TooltipContent>
                                </TooltipRoot>
                              </TooltipProvider>
                            ) : (
                              "—"
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          {log.errorMessage ? (
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <button type="button">
                                      <Badge
                                        variant={hasError ? "destructive" : "secondary"}
                                        className="text-xs"
                                      >
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
                            <Badge variant={ok ? "secondary" : "destructive"} className="text-xs">
                              {log.upstreamStatusCode}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="whitespace-nowrap">
                          <div className="text-xs">
                            {formatTokenPair(log.inputTokens, log.outputTokens)}
                          </div>
                          {cacheInfo && (
                            <div className="text-xs text-muted-foreground/70">{cacheInfo}</div>
                          )}
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-muted-foreground">
                          <div className="text-xs">
                            I: {formatDurationMs(log.firstTokenLatencyMs)}
                          </div>
                          {log.streamDurationMs > 0 && (
                            <div className="text-xs">
                              O: {formatDurationMs(log.streamDurationMs)}
                            </div>
                          )}
                        </TableCell>
                        <TableCell className="whitespace-nowrap text-xs tabular-nums">
                          {formatCost(log.cost, log.pricingCurrency)}
                        </TableCell>
                        <TableCell>
                          {log.userAgent ? (
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <button
                                      type="button"
                                      className="max-w-[80px] cursor-default truncate text-xs text-muted-foreground"
                                    >
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
                        >
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={<span>{log.traceId.slice(-6) || "—"}</span>}
                              />
                              <TooltipContent side="top">
                                {t("audit.copy_traceid_title")}
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
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
              totalLabel={t("pagination.logs")}
            />
          </CardContent>
        </Card>
      </div>
    </PermissionGuard>
  );
}
