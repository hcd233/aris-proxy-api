"use client";

import { useCallback, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api-client";
import type { SessionDetail, MessageItem, ToolItem, UnifiedToolCall, UnifiedTool } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import {
  ArrowLeft,
  ChevronDown,
  ChevronRight,
  Wrench,
  User,
  Bot,
  Clock,
  PanelRightOpen,
  PanelRightClose,
  Hash,
  Braces,
  FileText,
} from "lucide-react";

// ─── Inline Tool Call (inside assistant message) ────────────────────────────────

function ToolCallInline({ call }: { call: UnifiedToolCall }) {
  const [expanded, setExpanded] = useState(false);

  let argsDisplay: string;
  try {
    argsDisplay = JSON.stringify(JSON.parse(call.arguments), null, 2);
  } catch {
    argsDisplay = call.arguments;
  }

  return (
    <div className="mt-2 rounded-lg border border-amber-200/60 bg-amber-50/40 dark:border-amber-800/40 dark:bg-amber-950/20">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs transition-colors hover:bg-amber-100/50 dark:hover:bg-amber-900/20"
      >
        <div className="flex size-5 items-center justify-center rounded bg-amber-200/80 dark:bg-amber-800/40">
          <Wrench className="size-3 text-amber-700 dark:text-amber-400" />
        </div>
        <span className="font-mono font-medium text-amber-800 dark:text-amber-300">
          {call.name}
        </span>
        {call.id && (
          <span className="ml-1 font-mono text-[10px] text-muted-foreground/50">
            {call.id}
          </span>
        )}
        <span className="ml-auto text-muted-foreground/60">
          {expanded ? <ChevronDown className="size-3" /> : <ChevronRight className="size-3" />}
        </span>
      </button>
      {expanded && (
        <div className="border-t border-amber-200/60 dark:border-amber-800/40 px-3 py-2.5">
          <p className="mb-1.5 flex items-center gap-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            <Braces className="size-3" />
            Arguments
          </p>
          <pre className="overflow-x-auto rounded-md bg-background/60 p-2.5 text-[11px] leading-relaxed font-mono">
            {argsDisplay}
          </pre>
        </div>
      )}
    </div>
  );
}

// ─── Chat Bubble ────────────────────────────────────────────────────────────────

function ChatBubble({ message }: { message: MessageItem }) {
  const { role, content, tool_calls } = message.message;
  const isUser = role === "user";
  const isAssistant = role === "assistant";

  let textContent = "";
  if (typeof content === "string") {
    textContent = content;
  } else if (Array.isArray(content)) {
    textContent = content
      .filter((part: Record<string, unknown>) => part.type === "text")
      .map((part: Record<string, unknown>) => part.text as string)
      .join("\n");
  }

  const time = message.createdAt
    ? new Date(message.createdAt).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : "";

  const hasToolCalls = isAssistant && tool_calls && tool_calls.length > 0;

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"} group`}>
      <div className={`flex max-w-[85%] flex-col ${isUser ? "items-end" : "items-start"}`}>
        <div className="mb-1 flex items-center gap-1.5 px-1">
          {isUser ? (
            <User className="size-3 text-muted-foreground/60" />
          ) : isAssistant ? (
            <Bot className="size-3 text-muted-foreground/60" />
          ) : (
            <Badge variant="outline" className="px-1 py-0 text-[10px]">
              {role}
            </Badge>
          )}
          {message.model && (
            <span className="text-[10px] text-muted-foreground/60">{message.model}</span>
          )}
        </div>
        <div
          className={`rounded-2xl px-5 py-3.5 text-sm leading-relaxed shadow-sm ${
            isUser
              ? "rounded-br-md bg-primary text-primary-foreground"
              : isAssistant
                ? "rounded-bl-md border bg-card text-foreground"
                : "bg-secondary text-secondary-foreground"
          }`}
        >
          <div className="whitespace-pre-wrap break-words">{textContent || "\u2014"}</div>
          {hasToolCalls && (
            <div className="mt-3 space-y-1.5">
              {tool_calls!.map((call, i) => (
                <ToolCallInline key={call.id ?? i} call={call} />
              ))}
            </div>
          )}
        </div>
        {time && (
          <div className="mt-1 flex items-center gap-0.5 px-1 text-[10px] text-muted-foreground/50 opacity-0 transition-opacity group-hover:opacity-100">
            <Clock className="size-2.5" />
            {time}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Tool Sidebar Item ──────────────────────────────────────────────────────────

function ToolSidebarItem({ tool }: { tool: ToolItem }) {
  const [expanded, setExpanded] = useState(false);
  const toolData: UnifiedTool = tool.tool;

  const params = toolData.parameters;
  const paramProperties = (params?.properties as Record<string, Record<string, unknown>>) ?? {};
  const requiredParams = (params?.required as string[]) ?? [];

  return (
    <div className="rounded-lg border bg-card/50 shadow-xs">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm transition-colors hover:bg-accent/50"
      >
        <div className="flex size-6 shrink-0 items-center justify-center rounded-md bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400">
          <Wrench className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-foreground">{toolData.name}</p>
          <p className="truncate text-[11px] text-muted-foreground/70">
            {toolData.description || "No description"}
          </p>
        </div>
        {expanded ? (
          <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-3.5 shrink-0 text-muted-foreground" />
        )}
      </button>
      {expanded && (
        <div className="space-y-3 border-t px-3 py-3">
          {toolData.description && (
            <div>
              <p className="mb-1 flex items-center gap-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                <FileText className="size-3" />
                Description
              </p>
              <p className="text-xs leading-relaxed text-foreground/80">
                {toolData.description}
              </p>
            </div>
          )}
          {Object.keys(paramProperties).length > 0 && (
            <div>
              <p className="mb-1.5 flex items-center gap-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                <Braces className="size-3" />
                Parameters
              </p>
              <div className="space-y-1.5">
                {Object.entries(paramProperties).map(([name, schema]) => (
                  <div
                    key={name}
                    className="rounded-md bg-muted/40 px-2.5 py-1.5 text-xs"
                  >
                    <div className="flex items-center gap-1.5">
                      <span className="font-mono font-medium text-foreground">{name}</span>
                      {requiredParams.includes(name) && (
                        <span className="text-[9px] text-rose-500">required</span>
                      )}
                      {schema.type && (
                        <Badge variant="secondary" className="ml-auto px-1 py-0 text-[9px]">
                          {schema.type as string}
                        </Badge>
                      )}
                    </div>
                    {schema.description && (
                      <p className="mt-0.5 text-[11px] text-muted-foreground/70">
                        {schema.description as string}
                      </p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
          {params?.type && (
            <div className="flex items-center gap-1.5 text-[10px] text-muted-foreground/50">
              <Hash className="size-3" />
              Schema type: {params.type as string}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Main Page ──────────────────────────────────────────────────────────────────

export default function SessionDetailPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const sessionId = Number(searchParams.get("id"));

  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(true);

  const fetchSession = useCallback(async () => {
    if (!sessionId || Number.isNaN(sessionId)) return;
    setLoading(true);
    try {
      const rsp = await api.getSession(sessionId);
      if (rsp.session) setSession(rsp.session);
    } catch {
      // handled silently
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchSession();
  }, [fetchSession]);
  /* eslint-enable react-hooks/set-state-in-effect */

  if (!sessionId || Number.isNaN(sessionId)) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <p className="text-muted-foreground">Invalid session ID</p>
        <Button variant="outline" className="mt-4" onClick={() => router.push("/sessions/")}>
          Back to Sessions
        </Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-48" />
        <div className="space-y-6">
          <Skeleton className="h-24 w-3/4 ml-auto rounded-2xl" />
          <Skeleton className="h-32 w-3/4 rounded-2xl" />
          <Skeleton className="h-24 w-3/4 ml-auto rounded-2xl" />
        </div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <p className="text-muted-foreground">Session not found</p>
        <Button variant="outline" className="mt-4" onClick={() => router.push("/sessions/")}>
          Back to Sessions
        </Button>
      </div>
    );
  }

  const messages = session.messages ?? [];
  const tools = session.tools ?? [];

  return (
    <div className="flex h-[calc(100vh-6rem)] gap-0 overflow-hidden">
      {/* ── Main content area ── */}
      <div className="flex min-w-0 flex-1 flex-col">
        {/* Header */}
        <div className="flex items-center gap-3 border-b pb-4">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => router.push("/sessions/")}
            className="text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="size-4" />
          </Button>
          <div className="flex items-center gap-3">
            <h1 className="font-display text-xl font-semibold tracking-tight text-foreground">
              Session #{session.id}
            </h1>
            {session.apiKeyName && (
              <Badge variant="secondary" className="text-xs">
                {session.apiKeyName}
              </Badge>
            )}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Clock className="size-3.5" />
              <span>{new Date(session.createdAt).toLocaleString()}</span>
            </div>
            {tools.length > 0 && (
              <Button
                variant={sidebarOpen ? "secondary" : "ghost"}
                size="icon-sm"
                onClick={() => setSidebarOpen(!sidebarOpen)}
                title={sidebarOpen ? "Hide tools panel" : "Show tools panel"}
              >
                {sidebarOpen ? (
                  <PanelRightClose className="size-4" />
                ) : (
                  <PanelRightOpen className="size-4" />
                )}
              </Button>
            )}
          </div>
        </div>

        {/* Messages stream */}
        <div className="flex-1 space-y-4 overflow-y-auto py-4 pr-2">
          {messages.length === 0 ? (
            <p className="py-12 text-center text-sm text-muted-foreground">
              No messages in this session
            </p>
          ) : (
            messages.map((msg) => (
              <ChatBubble key={msg.id} message={msg} />
            ))
          )}
        </div>
      </div>

      {/* ── Right sidebar: Tools panel ── */}
      {sidebarOpen && tools.length > 0 && (
        <>
          <Separator orientation="vertical" className="mx-0 h-auto" />
          <aside className="flex w-80 shrink-0 flex-col overflow-hidden">
            <div className="flex items-center gap-2 border-b px-4 py-3">
              <Wrench className="size-4 text-muted-foreground" />
              <h2 className="text-sm font-semibold">Tools</h2>
              <Badge variant="secondary" className="ml-auto text-[10px]">
                {tools.length}
              </Badge>
            </div>
            <div className="flex-1 space-y-2 overflow-y-auto p-3">
              {tools.map((t) => (
                <ToolSidebarItem key={t.id} tool={t} />
              ))}
            </div>
          </aside>
        </>
      )}
    </div>
  );
}
