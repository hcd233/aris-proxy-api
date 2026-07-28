"use client";

import { useT } from "@/lib/i18n";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, Bot } from "lucide-react";
import { api } from "@/lib/api-client";
import type {
  MessageItem,
  PageInfo,
  TraceConversation,
  TraceConversationItem,
  TraceDetail,
  TraceEventItem,
} from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { PaginationBar } from "@/components/pagination-bar";
import { ChatMessage, buildToolResultsByID } from "@/components/chat/chat-message";
import { toast } from "sonner";

const EVENT_PAGE_SIZE = 50;

/**
 * 将 trace 对话项映射为 session 聊天消息结构，复用 ChatMessage 气泡渲染。
 * tool_call 拆成 assistant(tool_calls) + tool(output) 两条消息，
 * 由 buildToolResultsByID 配对后内联进 ToolCallCard。
 */
function itemToMessages(item: TraceConversationItem, model: string): MessageItem[] {
  const id = item.recordIds[0] ?? 0;
  if (item.kind === "tool_call") {
    const messages: MessageItem[] = [
      {
        id,
        model,
        createdAt: "",
        message: {
          role: "assistant",
          tool_calls: [
            {
              id: item.callId,
              name: item.toolName || "tool",
              arguments: item.arguments ?? "",
            },
          ],
        },
      },
    ];
    if (item.output) {
      messages.push({
        id: item.recordIds[1] ?? id,
        model,
        createdAt: "",
        message: {
          role: "tool",
          tool_call_id: item.callId,
          content: item.output,
        },
      });
    }
    return messages;
  }
  return [
    {
      id,
      model,
      createdAt: "",
      message: { role: item.role || "assistant", content: item.content },
    },
  ];
}

function statusBadge(status: string, t: (k: string, f?: string) => string) {
  if (status === "active") {
    return <Badge variant="secondary">{t("trace.status_active")}</Badge>;
  }
  if (status === "done") {
    return <Badge variant="outline">{t("trace.status_done")}</Badge>;
  }
  return <Badge variant="outline">{status}</Badge>;
}

function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return dateStr;
  const year = d.getFullYear();
  const month = d.getMonth() + 1;
  const day = d.getDate();
  const hours = String(d.getHours()).padStart(2, "0");
  const minutes = String(d.getMinutes()).padStart(2, "0");
  const seconds = String(d.getSeconds()).padStart(2, "0");
  return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
}

export default function TraceDetailClient({
  traceId,
}: {
  traceId: number;
}) {
  const router = useRouter();
  const t = useT();
  const [detail, setDetail] = useState<TraceDetail | null>(null);
  const [loading, setLoading] = useState(true);
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

  const chatMessages = useMemo(
    () =>
      (conversation?.turns ?? []).flatMap((turn) =>
        turn.items.flatMap((item) => itemToMessages(item, detail?.model ?? ""))
      ),
    [conversation, detail?.model]
  );
  const toolResultsByID = useMemo(() => buildToolResultsByID(chatMessages), [chatMessages]);

  const fetchEvents = useCallback(
    async (id: number, page: number, pageSize: number) => {
      setEventsLoading(true);
      try {
        const rsp = await api.listTraceEvents(id, page, pageSize);
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

  const fetchConversation = useCallback(
    async (id: number) => {
      setConversationLoading(true);
      try {
        const rsp = await api.getTraceConversation(id);
        setConversation(rsp.conversation ?? null);
      } catch {
        toast.error(t("trace.load_error"));
      } finally {
        setConversationLoading(false);
      }
    },
    [t]
  );

  const fetchDetail = useCallback(async () => {
    if (!traceId || Number.isNaN(traceId)) return;
    setLoading(true);
    try {
      const rsp = await api.getTrace(traceId);
      setDetail(rsp.trace ?? null);
      if (rsp.trace) {
        await Promise.all([
          fetchEvents(traceId, 1, EVENT_PAGE_SIZE),
          fetchConversation(traceId),
        ]);
      }
    } catch {
      toast.error(t("trace.load_error"));
    } finally {
      setLoading(false);
    }
  }, [traceId, fetchEvents, fetchConversation, t]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchDetail();
  }, [fetchDetail]);
  /* eslint-enable react-hooks/set-state-in-effect */

  if (!traceId || Number.isNaN(traceId)) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-muted-foreground">{t("trace.invalid_id")}</p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={() => router.push("/trace/")}
        >
          {t("trace.back_to_traces")}
        </Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-3">
          <Skeleton className="size-9 rounded-md" />
          <div className="space-y-2">
            <Skeleton className="h-7 w-40" />
            <Skeleton className="h-4 w-64" />
          </div>
        </div>
        <Skeleton className="h-44 w-full rounded-xl" />
        <Skeleton className="h-72 w-full rounded-xl" />
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-muted-foreground">{t("trace.not_found")}</p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={() => router.push("/trace/")}
        >
          {t("trace.back_to_traces")}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header — mirrors session detail: back + title + meta */}
      <div className="flex items-start gap-3">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => router.push("/trace/")}
          className="mt-1 size-9 shrink-0 text-foreground/70 hover:text-foreground"
          aria-label={t("trace.back_to_traces")}
          title={t("trace.back_to_traces")}
        >
          <ArrowLeft className="size-5" />
        </Button>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
              {t("trace.detail_title")}
            </h1>
            {statusBadge(detail.status, t)}
          </div>
          <p className="mt-1.5 truncate font-mono text-sm text-muted-foreground">
            {detail.sessionId}
          </p>
        </div>
      </div>

      {/* Overview */}
      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("trace.overview")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-6">
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
                {t("trace.event_count")}
              </p>
              <p className="mt-1 text-sm tabular-nums">{detail.eventCount}</p>
            </div>
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("trace.cwd")}
              </p>
              <p className="mt-1 truncate font-mono text-xs" title={detail.cwd}>
                {detail.cwd || "—"}
              </p>
            </div>
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("trace.created_at")}
              </p>
              <p className="mt-1 text-sm text-muted-foreground">{formatDateTime(detail.createdAt)}</p>
            </div>
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("trace.updated_at")}
              </p>
              <p className="mt-1 text-sm text-muted-foreground">{formatDateTime(detail.updatedAt)}</p>
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
        </CardContent>
      </Card>

      {/* Records */}
      <Card>
        <CardContent className="space-y-4">
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
              <div className="mx-auto w-full max-w-[768px] space-y-6">
                {conversation.turns.map((turn) => (
                  <div key={turn.turnId || "default"} className="space-y-4">
                    <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                      <span className="h-px flex-1 bg-border" />
                      <span>{turn.turnId || "session"}</span>
                      <span className="h-px flex-1 bg-border" />
                    </div>
                    <div className="space-y-5">
                      {turn.items
                        .flatMap((item) => itemToMessages(item, detail.model))
                        .map((m, i) => (
                          <ChatMessage
                            key={`${turn.turnId || "default"}-${i}`}
                            message={m}
                            index={i}
                            toolResultsByID={toolResultsByID}
                          />
                        ))}
                    </div>
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
                        <span className="text-xs text-muted-foreground">{formatDateTime(ev.createdAt)}</span>
                      </div>
                      <p className="mt-1 text-[11px] text-muted-foreground">
                        {ev.turnId ? `${t("trace.turn_id")}: ${ev.turnId} · ` : ""}{ev.callId ? `call: ${ev.callId} · ` : ""}record #{ev.id}
                      </p>
                      <pre className="mt-2 max-h-60 overflow-auto rounded-md bg-(--code-bg) p-3 font-mono text-[11px] leading-relaxed text-(--code-text)">{ev.payload != null ? JSON.stringify(ev.payload, null, 2) : t("trace.empty_payload")}</pre>
                    </div>
                  ))}
                </div>
              )}
              <PaginationBar
                pageInfo={eventPageInfo}
                onChange={(page, pageSize) => fetchEvents(detail.id, page, pageSize)}
                totalLabel={t("trace.event_count")}
              />
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
