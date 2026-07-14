"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import type {
  PageInfo,
  TraceDetail,
  TraceEventItem,
  TraceSummary,
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
import { Radar, RefreshCw, Search, Eye } from "lucide-react";
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

  const openDetail = useCallback(
    async (trace: TraceSummary) => {
      setDetailOpen(true);
      setDetail(null);
      setEvents([]);
      setEventPageInfo({ page: 1, pageSize: EVENT_PAGE_SIZE, total: 0 });
      try {
        const rsp = await api.getTrace(trace.id);
        setDetail(rsp.trace ?? null);
        if (rsp.trace) {
          await fetchEvents(trace.id, 1, EVENT_PAGE_SIZE);
        }
      } catch {
        toast.error(t("trace.load_error"));
      }
    },
    [fetchEvents, t]
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
          <Button variant="outline" size="sm" onClick={() => fetchTraces(persistedPage, persistedPageSize)}>
            <RefreshCw /> {t("trace.refresh")}
          </Button>
          <Button size="sm" onClick={() => setInstallOpen(true)}>
            <Radar /> {t("trace.install")}
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("trace.title")}</CardTitle>
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
                  <div className="flex items-center justify-between">
                    <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      {t("trace.events")}
                    </h3>
                    <span className="text-xs text-muted-foreground">
                      {eventPageInfo.total} {t("trace.event_count")}
                    </span>
                  </div>

                  {eventsLoading ? (
                    <div className="space-y-3">
                      {Array.from({ length: 3 }).map((_, i) => (
                        <Skeleton key={i} className="h-16 w-full" />
                      ))}
                    </div>
                  ) : events.length === 0 ? (
                    <p className="text-sm text-muted-foreground">{t("trace.no_events")}</p>
                  ) : (
                    <div className="space-y-2">
                      {events.map((ev) => (
                        <div key={ev.id} className="rounded-lg border border-border bg-card p-3">
                          <div className="flex items-center justify-between gap-3">
                            <span className="rounded-md bg-secondary px-2 py-0.5 font-mono text-xs">
                              {ev.event}
                            </span>
                            <span className="text-xs text-muted-foreground">{formatTime(ev.createdAt)}</span>
                          </div>
                          {ev.turnId ? (
                            <p className="mt-1 text-[11px] text-muted-foreground">
                              {t("trace.turn_id")}: {ev.turnId}
                            </p>
                          ) : null}
                          <pre className="mt-2 max-h-60 overflow-auto rounded-md bg-[#262624] p-3 font-mono text-[11px] leading-relaxed text-[#E5E0D6]">
                            {ev.payload != null
                              ? JSON.stringify(ev.payload, null, 2)
                              : t("trace.empty_payload")}
                          </pre>
                        </div>
                      ))}
                    </div>
                  )}

                  <PaginationBar
                    pageInfo={eventPageInfo}
                    onChange={(page, pageSize) =>
                      detail && fetchEvents(detail.id, page, pageSize)
                    }
                    totalLabel={t("trace.event_count")}
                  />
                </section>
              </>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
