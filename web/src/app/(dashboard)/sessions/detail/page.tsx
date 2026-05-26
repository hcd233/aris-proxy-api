"use client";

import { useCallback, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api-client";
import type { SessionDetail, MessageItem, ToolItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowLeft, ChevronDown, ChevronRight, Wrench, User, Bot, Clock } from "lucide-react";

function ChatBubble({ message }: { message: MessageItem }) {
  const role = (message.message as Record<string, unknown>)?.role as string ?? "unknown";
  const content = (message.message as Record<string, unknown>)?.content;
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
    ? new Date(message.createdAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : "";

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"} group`}>
      <div className={`flex max-w-[85%] flex-col ${isUser ? "items-end" : "items-start"}`}>
        <div className="mb-1 flex items-center gap-1.5 px-1">
          {isUser ? (
            <User className="size-3 text-muted-foreground/60" />
          ) : isAssistant ? (
            <Bot className="size-3 text-muted-foreground/60" />
          ) : (
            <Badge variant="outline" className="text-[10px] px-1 py-0">{role}</Badge>
          )}
          {message.model && (
            <span className="text-[10px] text-muted-foreground/60">{message.model}</span>
          )}
        </div>
        <div
          className={`rounded-2xl px-5 py-3.5 text-sm leading-relaxed shadow-sm ${
            isUser
              ? "bg-primary text-primary-foreground rounded-br-md"
              : isAssistant
                ? "border bg-card text-foreground rounded-bl-md"
                : "bg-secondary text-secondary-foreground"
          }`}
        >
          <div className="whitespace-pre-wrap break-words">{textContent || "—"}</div>
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

function ToolCallBlock({ tool }: { tool: ToolItem }) {
  const [expanded, setExpanded] = useState(false);
  const toolData = tool.tool as Record<string, unknown>;
  const name = (toolData?.name as string) ?? (toolData?.type as string) ?? "Tool";
  const input = toolData?.input ?? toolData?.arguments ?? toolData?.params;
  const output = toolData?.output ?? toolData?.result;

  const renderValue = (val: unknown): string => {
    if (typeof val === "string") return val;
    try {
      return JSON.stringify(val, null, 2);
    } catch {
      return String(val);
    }
  };

  return (
    <div className="mx-auto max-w-[85%] rounded-xl border border-border bg-card/50 shadow-sm">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-2.5 rounded-xl px-4 py-3 text-left text-sm transition-colors hover:bg-accent/50"
      >
        <div className="flex size-6 items-center justify-center rounded-md bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
          <Wrench className="size-3.5" />
        </div>
        <span className="font-medium">{name}</span>
        <span className="ml-auto text-xs text-muted-foreground">
          {expanded ? "Hide details" : "Show details"}
        </span>
        {expanded ? (
          <ChevronDown className="size-3.5 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-3.5 text-muted-foreground" />
        )}
      </button>
      {expanded && (
        <div className="border-t px-4 py-3 space-y-3">
          {input != null && (
            <div>
              <p className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                <span className="inline-block h-1.5 w-1.5 rounded-full bg-blue-500" />
                Input
              </p>
              <pre className="overflow-x-auto rounded-lg border border-blue-100 bg-blue-50/50 p-3 text-xs dark:border-blue-900/30 dark:bg-blue-950/20">
                {renderValue(input)}
              </pre>
            </div>
          )}
          {output != null && (
            <div>
              <p className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                Output
              </p>
              <pre className="overflow-x-auto rounded-lg border border-emerald-100 bg-emerald-50/50 p-3 text-xs dark:border-emerald-900/30 dark:bg-emerald-950/20">
                {renderValue(output)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default function SessionDetailPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const sessionId = Number(searchParams.get("id"));

  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);

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

  const allItems: Array<{ type: "message" | "tool"; data: MessageItem | ToolItem; createdAt: string }> = [
    ...(session.messages ?? []).map((m) => ({ type: "message" as const, data: m, createdAt: m.createdAt })),
    ...(session.tools ?? []).map((t) => ({ type: "tool" as const, data: t, createdAt: t.createdAt })),
  ].sort((a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime());

  return (
    <div className="space-y-6">
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
      </div>

      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-muted-foreground">
        <div className="flex items-center gap-1.5">
          <Clock className="size-3.5" />
          <span>Created {new Date(session.createdAt).toLocaleString()}</span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-muted-foreground/40">·</span>
          <span>Updated {new Date(session.updatedAt).toLocaleString()}</span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-muted-foreground/40">·</span>
          <span>{allItems.length} item{allItems.length !== 1 ? "s" : ""}</span>
        </div>
      </div>

      <div className="space-y-6 pb-8">
        {allItems.length === 0 ? (
          <p className="py-12 text-center text-sm text-muted-foreground">
            No messages in this session
          </p>
        ) : (
          allItems.map((item, idx) =>
            item.type === "message" ? (
              <ChatBubble key={`msg-${idx}`} message={item.data as MessageItem} />
            ) : (
              <ToolCallBlock key={`tool-${idx}`} tool={item.data as ToolItem} />
            )
          )
        )}
      </div>
    </div>
  );
}
