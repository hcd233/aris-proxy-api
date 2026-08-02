"use client";

import { useT } from "@/lib/i18n";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowLeft, Bot, Braces, ChevronDown, ChevronRight, Wrench } from "lucide-react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { PageInfo, TraceDetail, TraceEventItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { TooltipProvider, TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { PaginationBar } from "@/components/pagination-bar";
import { DeleteIconButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { toast } from "sonner";
import { formatDateTime } from "@/lib/utils";

const EVENT_PAGE_SIZE = 50;

// ─── payload 解析辅助 ────────────────────────────────────────────────────────

function payloadRoot(payload: unknown): Record<string, unknown> {
  if (payload && typeof payload === "object" && !Array.isArray(payload)) {
    return payload as Record<string, unknown>;
  }
  return {};
}

/** 提取 rollout 记录的 payload.payload 内层；hook 等扁平记录回退整个 payload。 */
function payloadInner(payload: unknown): Record<string, unknown> {
  const root = payloadRoot(payload);
  const inner = root.payload;
  if (inner && typeof inner === "object" && !Array.isArray(inner)) {
    return inner as Record<string, unknown>;
  }
  return root;
}

/** 拼接消息 content 数组（input_text/output_text 等含 text 字段的分块）。 */
function messageText(inner: Record<string, unknown>): string {
  const content = inner.content;
  if (Array.isArray(content)) {
    return content
      .map((part) => {
        if (part && typeof part === "object" && typeof (part as { text?: unknown }).text === "string") {
          return (part as { text: string }).text;
        }
        return "";
      })
      .filter((text) => text.length > 0)
      .join("\n");
  }
  if (typeof inner.message === "string") return inner.message;
  if (typeof content === "string") return content;
  return "";
}

/** 对象/JSON 字符串 → 格式化 JSON 文本；其余原样字符串化。 */
function jsonText(value: unknown): string {
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  if (value === undefined) return "";
  return JSON.stringify(value, null, 2);
}

// ─── 事件可读化分类 ──────────────────────────────────────────────────────────

type SessionMetaTool = {
  namespace: string;
  name: string;
  description?: string;
  parameters?: string;
};

type EventView =
  | { kind: "message"; role: string; text: string }
  | { kind: "tool_call"; name: string; argumentsText: string }
  | { kind: "tool_output"; outputText: string }
  | { kind: "session_meta"; systemPrompt: string; tools: SessionMetaTool[] }
  | { kind: "plain" };

function classifyEvent(ev: TraceEventItem): EventView {
  const inner = payloadInner(ev.payload);
  if (ev.recordType === "session_meta") {
    const baseInstructions = inner.base_instructions;
    const systemPrompt =
      baseInstructions && typeof baseInstructions === "object"
        ? String((baseInstructions as { text?: unknown }).text ?? "")
        : "";
    const tools: SessionMetaTool[] = [];
    const dynamicTools = inner.dynamic_tools;
    if (Array.isArray(dynamicTools)) {
      for (const ns of dynamicTools) {
        if (!ns || typeof ns !== "object") continue;
        const namespace = String((ns as { name?: unknown }).name ?? "");
        const list = (ns as { tools?: unknown }).tools;
        if (!Array.isArray(list)) continue;
        for (const tool of list) {
          if (!tool || typeof tool !== "object") continue;
          const name = String((tool as { name?: unknown }).name ?? "");
          if (!name) continue;
          const description = String((tool as { description?: unknown }).description ?? "");
          const parameters = (tool as { parameters?: unknown }).parameters;
          tools.push({
            namespace,
            name,
            description: description || undefined,
            parameters: parameters != null ? jsonText(parameters) : undefined,
          });
        }
      }
    }
    return { kind: "session_meta", systemPrompt, tools };
  }
  switch (ev.event) {
    case "user_message":
    case "agent_message":
      return {
        kind: "message",
        role: ev.event === "user_message" ? "user" : "assistant",
        text: messageText(inner),
      };
    case "message":
      return { kind: "message", role: String(inner.role ?? "assistant"), text: messageText(inner) };
    case "function_call":
      return { kind: "tool_call", name: String(inner.name ?? ""), argumentsText: jsonText(inner.arguments) };
    case "function_call_output":
      return { kind: "tool_output", outputText: jsonText(inner.output) };
    default:
      return { kind: "plain" };
  }
}

// ─── 单条事件卡片 ────────────────────────────────────────────────────────────

function EventCard({
  ev,
  t,
}: {
  ev: TraceEventItem;
  t: (k: string, f?: string) => string;
}) {
  const [showJson, setShowJson] = useState(false);
  const view = useMemo(() => classifyEvent(ev), [ev]);
  const rawJson = useMemo(
    () => JSON.stringify(ev.payload ?? ev, null, 2),
    [ev]
  );

  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="rounded-md bg-secondary px-2 py-0.5 font-mono text-xs">
          {ev.event || ev.recordType || "—"}
        </span>
        <Badge variant="outline" className="text-[10px]">{ev.source}</Badge>
        {view.kind === "message" && (
          <Badge variant="secondary" className="text-[10px]">{view.role}</Badge>
        )}
        {view.kind === "tool_call" && view.name && (
          <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-2 py-0.5 font-mono text-xs">
            <Wrench className="size-3" />
            {view.name}
          </span>
        )}
        <span className="ml-auto text-xs text-muted-foreground">
          {formatDateTime(ev.createdAt)}
        </span>
      </div>
      <p className="mt-1 text-[11px] text-muted-foreground">
        record #{ev.id}
        {ev.turnId ? ` · ${t("trace.turn_id")}: ${ev.turnId}` : ""}
        {ev.callId ? ` · call: ${ev.callId}` : ""}
      </p>

      {view.kind === "message" && view.text && (
        <pre className="mt-2 max-h-96 overflow-auto whitespace-pre-wrap break-words rounded-md bg-(--code-bg) p-3 font-mono text-[12px] leading-relaxed text-(--code-text)">
          {view.text}
        </pre>
      )}
      {view.kind === "tool_call" && (
        <pre className="mt-2 max-h-60 overflow-auto rounded-md bg-(--code-bg) p-3 font-mono text-[11px] leading-relaxed text-(--code-text)">
          {view.argumentsText || t("trace.empty_payload")}
        </pre>
      )}
      {view.kind === "tool_output" && (
        <pre className="mt-2 max-h-60 overflow-auto rounded-md bg-(--code-bg) p-3 font-mono text-[11px] leading-relaxed text-(--code-text)">
          {view.outputText || t("trace.empty_payload")}
        </pre>
      )}
      {view.kind === "session_meta" && (
        <div className="mt-2 space-y-3">
          {view.systemPrompt && (
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("trace.system_prompt")}
              </p>
              <pre className="mt-1 max-h-60 overflow-auto whitespace-pre-wrap break-words rounded-md bg-(--code-bg) p-3 font-mono text-[11px] leading-relaxed text-(--code-text)">
                {view.systemPrompt}
              </pre>
            </div>
          )}
          {view.tools.length > 0 && (
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("trace.tools")} ({view.tools.length})
              </p>
              <div className="mt-1 space-y-2">
                {view.tools.map((tool, i) => (
                  <div
                    key={`${tool.namespace}-${tool.name}-${i}`}
                    className="rounded-lg border border-border bg-secondary/30 p-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      {tool.namespace && (
                        <Badge variant="secondary" className="font-mono text-[10px]">
                          {tool.namespace}
                        </Badge>
                      )}
                      <span className="font-mono text-sm font-medium">{tool.name}</span>
                    </div>
                    {tool.description && (
                      <p className="mt-1.5 text-[13px] leading-relaxed text-muted-foreground">
                        {tool.description}
                      </p>
                    )}
                    {tool.parameters && (
                      <pre className="mt-2 max-h-60 overflow-auto rounded-md bg-(--code-bg) p-3 font-mono text-[11px] leading-relaxed text-(--code-text)">
                        {tool.parameters}
                      </pre>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      <button
        type="button"
        onClick={() => setShowJson((v) => !v)}
        className="mt-2 inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
      >
        <Braces className="size-3.5" />
        {showJson ? t("trace.collapse_json") : t("trace.expand_json")}
        {showJson ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
      </button>
      {showJson && (
        <pre className="mt-2 max-h-96 overflow-auto rounded-md bg-(--code-bg) p-3 font-mono text-[11px] leading-relaxed text-(--code-text)">
          {rawJson}
        </pre>
      )}
    </div>
  );
}

// ─── 页面主体 ────────────────────────────────────────────────────────────────

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
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

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

  const fetchDetail = useCallback(async () => {
    if (!traceId || Number.isNaN(traceId)) return;
    setLoading(true);
    try {
      const rsp = await api.getTrace(traceId);
      setDetail(rsp.trace ?? null);
      if (rsp.trace) {
        await fetchEvents(traceId, 1, EVENT_PAGE_SIZE);
      }
    } catch (err) {
      showErrorToast(err, { title: t("trace.load_error") });
    } finally {
      setLoading(false);
    }
  }, [traceId, fetchEvents, t]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchDetail();
  }, [fetchDetail]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await api.deleteTrace(traceId);
      toast.success(t("trace.delete_success"));
      router.push("/trace/");
    } catch (err) {
      showErrorToast(err, { title: t("trace.delete_error") });
      setDeleting(false);
      setDeleteConfirmOpen(false);
    }
  };

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
        <TooltipProvider>
          <TooltipRoot>
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => router.push("/trace/")}
                  className="mt-1 size-9 shrink-0 text-foreground/70 hover:text-foreground"
                  aria-label={t("trace.back_to_traces")}
                >
                  <ArrowLeft className="size-5" />
                </Button>
              }
            />
            <TooltipContent side="top">{t("trace.back_to_traces")}</TooltipContent>
          </TooltipRoot>
        </TooltipProvider>
        <div className="mt-1 shrink-0">
          <TooltipProvider>
            <TooltipRoot>
              <TooltipTrigger
                render={
                  <DeleteIconButton
                    aria-label={t("trace.delete_aria")}
                    disabled={deleting}
                    onClick={() => setDeleteConfirmOpen(true)}
                  />
                }
              />
              <TooltipContent side="top">{t("trace.delete_aria")}</TooltipContent>
            </TooltipRoot>
          </TooltipProvider>
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
              {t("trace.detail_title")}
            </h1>
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
              {detail.cwd ? (
                <TooltipProvider>
                  <TooltipRoot>
                    <TooltipTrigger
                      render={
                        <p className="mt-1 truncate font-mono text-xs">
                          {detail.cwd}
                        </p>
                      }
                    />
                    <TooltipContent side="top" align="start" className="max-w-xs break-all">
                      {detail.cwd}
                    </TooltipContent>
                  </TooltipRoot>
                </TooltipProvider>
              ) : (
                <p className="mt-1 truncate font-mono text-xs">—</p>
              )}
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

      {/* 事件时间线 */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-3">
          <CardTitle className="flex items-center gap-2 font-display">
            <Bot className="size-4 text-muted-foreground" />
            {t("trace.timeline")}
          </CardTitle>
          <span className="text-xs text-muted-foreground">
            {t("pagination.total_format")
              .replace("{count}", String(eventPageInfo.total))
              .replace("{label}", t("trace.event_count"))}
          </span>
        </CardHeader>
        <CardContent className="space-y-4">
          {eventsLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-16 w-full" />)}
            </div>
          ) : events.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("trace.no_events")}</p>
          ) : (
            <div className="space-y-2">
              {events.map((ev) => (
                <EventCard key={ev.id} ev={ev} t={t} />
              ))}
            </div>
          )}
          <PaginationBar
            pageInfo={eventPageInfo}
            onChange={(page, pageSize) => fetchEvents(detail.id, page, pageSize)}
            totalLabel={t("trace.event_count")}
          />
        </CardContent>
      </Card>

      <DeleteConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={t("trace.delete_dialog_title")}
        description={t("trace.delete_dialog_desc").replace("{name}", String(traceId))}
        confirmLabel={t("common.delete")}
        loadingLabel={t("common.deleting")}
        loading={deleting}
        onConfirm={handleDelete}
      />
    </div>
  );
}
