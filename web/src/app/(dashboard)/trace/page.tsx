"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type {
  PageInfo,
  TraceDetail,
  TraceEventItem,
  TraceSummary,
  TraceConversation,
} from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Radar, Search, Eye, ChevronDown, Bot, Wrench, UserRound } from "lucide-react";
import { Codex } from "@lobehub/icons";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import TraceInstallDialog from "@/components/trace-install-dialog";

const EVENT_PAGE_SIZE = 50;

function statusBadge(status: string, t: (k: string, f?: string) => string) {
  if (status === "active") {
    return <Badge variant="secondary">{t("trace.status_active")}</Badge>;
  }
  if (status === "done") {
    return <Badge variant="outline">{t("trace.status_done")}</Badge>;
  }
  return <Badge variant="outline">{status}</Badge>;
}

function formatTime(s: string): string {
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  return d.toLocaleString();
}

export default function TracePage() {
  const [traces, setTraces] = useState<TraceSummary[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.trace.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.trace.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [installOpen, setInstallOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<TraceDetail | null>(null);
  const [events, setEvents] = useState<TraceEventItem[]>([]);
  const [eventPageInfo, setEventPageInfo] = useState<PageInfo>({
    page: 1,
    pageSize: EVENT_PAGE_SIZE,
    total: 0,
  });
  const [eventsLoading, setEventsLoading] = useState(false);
  const [conversation, setConversation] = useState<TraceConversation | null>(null);
  const [conversationLoading, setConversationLoading] = useState(false);
  const [detailTab, setDetailTab] = useState<"conversation" | "raw">("conversation");
  const t = useT();
  const isMobile = useIsMobile();

  const fetchTraces = useCallback(
    async (page: number, pageSize: number, query?: string) => {
      setLoading(true);
      try {
        const safeSize = pageSize > 0 ? pageSize : 20;
        const rsp = await api.listTraces(page, safeSize);
        void query;
        setTraces(rsp.traces ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch {
        toast.error(t("trace.load_error"));
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t]
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch list when the persisted page or size changes */
  useEffect(() => {
    fetchTraces(persistedPage, persistedPageSize);
  }, [fetchTraces, persistedPage, persistedPageSize]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const fetchEvents = useCallback(
    async (traceId: number, page: number, pageSize: number) => {
      setEventsLoading(true);
      try {
        const rsp = await api.listTraceEvents(traceId, page, pageSize);
        setEvents(rsp.events ?? []);
        if (rsp.pageInfo) setEventPageInfo(rsp.pageInfo);
      } catch {
        toast.error(t("trace.load_error"));
      } finally {
        setEventsLoading(false);
      }
    },
    [t]
  );

  const fetchConversation = useCallback(async (traceId: number) => {
    setConversationLoading(true);
    try {
      const rsp = await api.getTraceConversation(traceId);
      setConversation(rsp.conversation ?? null);
    } catch {
      toast.error(t("trace.load_error"));
    } finally {
      setConversationLoading(false);
    }
  }, [t]);

  const openDetail = useCallback(
    async (trace: TraceSummary) => {
      setDetailOpen(true);
      setDetail(null);
      setEvents([]);
      setConversation(null);
      setDetailTab("conversation");
      setEventPageInfo({ page: 1, pageSize: EVENT_PAGE_SIZE, total: 0 });
      try {
        const rsp = await api.getTrace(trace.id);
        setDetail(rsp.trace ?? null);
        if (rsp.trace) {
          await Promise.all([
            fetchEvents(trace.id, 1, EVENT_PAGE_SIZE),
            fetchConversation(trace.id),
          ]);
        }
      } catch {
        toast.error(t("trace.load_error"));
      }
    },
    [fetchConversation, fetchEvents, t]
  );

  const handleSearch = useCallback(() => {
    setPersistedPage(1);
    fetchTraces(1, persistedPageSize);
  }, [fetchTraces, persistedPageSize, setPersistedPage]);

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            {t("trace.title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{t("trace.subtitle")}</p>
        </div>
        <div className="flex items-center gap-2">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant="outline" className="gap-1.5" />}
            >
              <Radar className="size-4" />
              {t("trace.install")}
              <ChevronDown className="size-3.5 opacity-50 transition-transform duration-150 group-aria-expanded/button:rotate-180" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64 p-1.5">
              <DropdownMenuGroup>
                <DropdownMenuLabel className="px-2 pb-1.5 pt-1 text-[11px] uppercase tracking-[0.08em] text-muted-foreground/70">
                  {t("trace.install_target")}
                </DropdownMenuLabel>
                <DropdownMenuItem
                  onClick={() => setInstallOpen(true)}
                  className="items-start gap-2.5 rounded-lg px-2 py-2"
                >
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-gradient-to-b from-secondary to-muted">
                    <Codex.Color size={17} />
                  </span>
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="text-sm font-medium leading-none">
                      {t("trace.install_codex")}
                    </span>
                    <span className="truncate text-xs text-muted-foreground">
                      {t("trace.install_codex_hint")}
                    </span>
                  </span>
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("trace.all_traces")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="mb-4">
            <div className="relative w-full md:max-w-sm">
              <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={t("trace.search_placeholder")}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleSearch();
                }}
                className="pl-9"
              />
            </div>
          </div>

          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : traces.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Radar className="mb-3 size-10 text-muted-foreground/40" />
              <p className="text-sm text-muted-foreground">{t("trace.no_traces")}</p>
            </div>
          ) : (
            <>
              {isMobile ? (
                <div className="space-y-3">
                  {traces.map((tr) => (
                    <div
                      key={tr.id}
                      className="rounded-lg border border-border bg-card p-4"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <p className="truncate font-mono text-sm font-medium">{tr.sessionId}</p>
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            {tr.agent} · {tr.model} · {statusBadge(tr.status, t)}
                          </p>
                        </div>
                        <Button variant="outline" size="sm" onClick={() => openDetail(tr)}>
                          <Eye />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-16">{t("common.id")}</TableHead>
                      <TableHead>{t("trace.session_id")}</TableHead>
                      <TableHead>{t("trace.agent")}</TableHead>
                      <TableHead>{t("trace.api_key")}</TableHead>
                      <TableHead>{t("trace.model")}</TableHead>
                      <TableHead>{t("trace.source")}</TableHead>
                      <TableHead className="w-24">{t("trace.status")}</TableHead>
                      <TableHead className="w-32">{t("trace.created_at")}</TableHead>
                      <TableHead className="w-20">{t("common.actions")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {traces.map((tr) => (
                      <TableRow key={tr.id}>
                        <TableCell className="text-muted-foreground">{tr.id}</TableCell>
                        <TableCell className="font-mono">{tr.sessionId}</TableCell>
                        <TableCell>{tr.agent}</TableCell>
                        <TableCell>{tr.apiKeyName}</TableCell>
                        <TableCell>{tr.model}</TableCell>
                        <TableCell>{tr.source}</TableCell>
                        <TableCell>{statusBadge(tr.status, t)}</TableCell>
                        <TableCell className="text-muted-foreground">{formatTime(tr.createdAt)}</TableCell>
                        <TableCell>
                          <Button variant="outline" size="sm" onClick={() => openDetail(tr)}>
                            <Eye /> {t("trace.view_detail")}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
              <PaginationBar
                pageInfo={pageInfo}
                onChange={(page, pageSize) => fetchTraces(page, pageSize)}
                totalLabel={t("trace.event_count")}
              />
            </>
          )}
        </CardContent>
      </Card>

      <TraceInstallDialog open={installOpen} onOpenChange={setInstallOpen} />

      <Dialog open={detailOpen} onOpenChange={setDetailOpen}>
        <DialogContent className="sm:max-w-3xl h-[min(86vh,760px)] p-0 gap-0 overflow-hidden flex flex-col">
          <DialogHeader className="shrink-0 flex-row items-center gap-3 px-6 py-4 border-b border-border">
            <span className="flex size-9 items-center justify-center rounded-xl border border-border bg-gradient-to-b from-secondary to-muted shadow-sm">
              <Radar className="size-4.5" />
            </span>
            <div className="flex flex-col gap-0.5 min-w-0">
              <DialogTitle className="font-display text-base leading-tight">
                {t("trace.detail_title")}
              </DialogTitle>
              <DialogDescription className="font-mono text-xs leading-snug truncate">
                {detail ? detail.sessionId : ""}
              </DialogDescription>
            </div>
          </DialogHeader>

          <div className="flex-1 min-h-0 overflow-y-auto px-6 py-5 space-y-6">
            {!detail ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Radar className="mb-3 size-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">{t("trace.load_error")}</p>
              </div>
            ) : (
              <>
                <section className="grid grid-cols-2 gap-4 md:grid-cols-3">
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.agent")}
                    </p>
                    <p className="mt-1 text-sm">{detail.agent}</p>
                  </div>
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.model")}
                    </p>
                    <p className="mt-1 text-sm">{detail.model}</p>
                  </div>
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.source")}
                    </p>
                    <p className="mt-1 text-sm">{detail.source}</p>
                  </div>
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.api_key")}
                    </p>
                    <p className="mt-1 text-sm">{detail.apiKeyName}</p>
                  </div>
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.status")}
                    </p>
                    <p className="mt-1 text-sm">{statusBadge(detail.status, t)}</p>
                  </div>
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.cwd")}
                    </p>
                    <p className="mt-1 truncate font-mono text-xs" title={detail.cwd}>
                      {detail.cwd || "—"}
                    </p>
                  </div>
                </section>

                {detail.metadata && Object.keys(detail.metadata).length > 0 && (
                  <section className="space-y-2">
                    <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.metadata")}
                    </h3>
                    <div className="space-y-1 rounded-lg border border-border bg-secondary/40 p-3">
                      {Object.entries(detail.metadata).map(([k, v]) => (
                        <div key={k} className="flex gap-2 text-xs">
                          <span className="shrink-0 font-mono text-muted-foreground">{k}:</span>
                          <span className="break-all font-mono">{v}</span>
                        </div>
                      ))}
                    </div>
                  </section>
                )}

                <section className="space-y-3">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex rounded-lg border border-border bg-secondary/40 p-1">
                      <Button
                        variant={detailTab === "conversation" ? "secondary" : "ghost"}
                        size="sm"
                        onClick={() => setDetailTab("conversation")}
                      >
                        <Bot /> {t("trace.conversation")}
                      </Button>
                      <Button
                        variant={detailTab === "raw" ? "secondary" : "ghost"}
                        size="sm"
                        onClick={() => setDetailTab("raw")}
                      >
                        {t("trace.raw_records")}
                      </Button>
                    </div>
                    <span className="text-xs text-muted-foreground">
                      {detailTab === "raw"
                        ? `${eventPageInfo.total} ${t("trace.event_count")}`
                        : `${conversation?.turns.length ?? 0} ${t("trace.turns")}`}
                    </span>
                  </div>

                  {detailTab === "conversation" ? (
                    conversationLoading ? (
                      <div className="space-y-3">
                        {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-20 w-full" />)}
                      </div>
                    ) : !conversation || conversation.turns.length === 0 ? (
                      <p className="text-sm text-muted-foreground">{t("trace.no_events")}</p>
                    ) : (
                      <div className="space-y-5">
                        {conversation.turns.map((turn) => (
                          <div key={turn.turnId || "default"} className="space-y-3">
                            <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                              <span className="h-px flex-1 bg-border" />
                              <span>{turn.turnId || "session"}</span>
                              <span className="h-px flex-1 bg-border" />
                            </div>
                            {turn.items.map((item, index) => (
                              <div key={`${item.recordIds.join("-")}-${index}`} className="rounded-xl border border-border bg-card p-3 shadow-sm">
                                <div className="flex items-center gap-2 text-xs font-medium">
                                  {item.kind === "tool_call" ? <Wrench className="size-3.5 text-amber-500" /> : item.role === "user" ? <UserRound className="size-3.5 text-sky-500" /> : <Bot className="size-3.5 text-emerald-500" />}
                                  <span className="capitalize">{item.kind === "tool_call" ? item.toolName || "tool" : item.role || "message"}</span>
                                  <Badge variant="outline" className="ml-auto text-[10px]">{item.source}</Badge>
                                </div>
                                {item.content ? <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed">{item.content}</p> : null}
                                {item.arguments ? <pre className="mt-2 overflow-auto rounded-md bg-[#262624] p-3 font-mono text-[11px] text-[#E5E0D6]">{item.arguments}</pre> : null}
                                {item.output ? <pre className="mt-2 overflow-auto rounded-md bg-secondary/50 p-3 font-mono text-[11px]">{item.output}</pre> : null}
                              </div>
                            ))}
                          </div>
                        ))}
                      </div>
                    )
                  ) : (
                    <>
                      {eventsLoading ? (
                        <div className="space-y-3">
                          {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-16 w-full" />)}
                        </div>
                      ) : events.length === 0 ? (
                        <p className="text-sm text-muted-foreground">{t("trace.no_events")}</p>
                      ) : (
                        <div className="space-y-2">
                          {events.map((ev) => (
                            <div key={ev.id} className="rounded-lg border border-border bg-card p-3">
                              <div className="flex items-center justify-between gap-3">
                                <div className="flex items-center gap-2">
                                  <span className="rounded-md bg-secondary px-2 py-0.5 font-mono text-xs">{ev.event}</span>
                                  <Badge variant="outline" className="text-[10px]">{ev.source}</Badge>
                                </div>
                                <span className="text-xs text-muted-foreground">{formatTime(ev.createdAt)}</span>
                              </div>
                              <p className="mt-1 text-[11px] text-muted-foreground">
                                {ev.turnId ? `${t("trace.turn_id")}: ${ev.turnId} · ` : ""}{ev.callId ? `call: ${ev.callId} · ` : ""}record #{ev.id}
                              </p>
                              <pre className="mt-2 max-h-60 overflow-auto rounded-md bg-[#262624] p-3 font-mono text-[11px] leading-relaxed text-[#E5E0D6]">{ev.payload != null ? JSON.stringify(ev.payload, null, 2) : t("trace.empty_payload")}</pre>
                            </div>
                          ))}
                        </div>
                      )}
                      <PaginationBar pageInfo={eventPageInfo} onChange={(page, pageSize) => detail && fetchEvents(detail.id, page, pageSize)} totalLabel={t("trace.event_count")} />
                    </>
                  )}
                </section>
              </>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
