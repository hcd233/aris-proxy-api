"use client";

import { useCallback, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api-client";
import type { SessionDetail, MessageItem, ToolItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowLeft, ChevronDown, ChevronRight, Wrench } from "lucide-react";

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

  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <div
        className={`max-w-[80%] rounded-xl px-4 py-2.5 text-sm ${
          isUser
            ? "bg-primary text-primary-foreground"
            : isAssistant
              ? "bg-muted text-foreground"
              : "bg-secondary text-secondary-foreground"
        }`}
      >
        <div className="mb-1 flex items-center gap-1.5">
          <Badge variant="outline" className="text-[10px] px-1 py-0">
            {role}
          </Badge>
          {message.model && (
            <span className="text-[10px] opacity-60">{message.model}</span>
          )}
        </div>
        <div className="whitespace-pre-wrap break-words">{textContent || "—"}</div>
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
    <div className="rounded-lg border border-border bg-card">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-accent"
      >
        <Wrench className="size-3.5 text-muted-foreground" />
        <span className="font-medium">{name}</span>
        {expanded ? (
          <ChevronDown className="ml-auto size-3.5 text-muted-foreground" />
        ) : (
          <ChevronRight className="ml-auto size-3.5 text-muted-foreground" />
        )}
      </button>
      {expanded && (
        <div className="border-t px-3 py-2">
          {input != null && (
            <div className="mb-2">
              <p className="mb-1 text-xs font-medium text-muted-foreground">Input</p>
              <pre className="overflow-x-auto rounded bg-muted p-2 text-xs">
                {renderValue(input)}
              </pre>
            </div>
          )}
          {output != null && (
            <div>
              <p className="mb-1 text-xs font-medium text-muted-foreground">Output</p>
              <pre className="overflow-x-auto rounded bg-muted p-2 text-xs">
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
        <Button variant="outline" className="mt-4" onClick={() => router.push("/web/sessions/")}>
          Back to Sessions
        </Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-32" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (!session) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <p className="text-muted-foreground">Session not found</p>
        <Button variant="outline" className="mt-4" onClick={() => router.push("/web/sessions/")}>
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
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon-sm" onClick={() => router.push("/web/sessions/")}>
          <ArrowLeft className="size-4" />
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Session #{session.id}
          </h1>
          <p className="text-sm text-muted-foreground">
            {session.apiKeyName && `Key: ${session.apiKeyName}`}
          </p>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Session Info</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <p className="text-xs text-muted-foreground">ID</p>
              <p className="font-mono text-sm">{session.id}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">API Key</p>
              <p className="text-sm">{session.apiKeyName || "—"}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Created</p>
              <p className="text-sm">{new Date(session.createdAt).toLocaleString()}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">Updated</p>
              <p className="text-sm">{new Date(session.updatedAt).toLocaleString()}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <Separator />

      <div>
        <h2 className="mb-4 text-lg font-semibold">Conversation</h2>
        <div className="space-y-3">
          {allItems.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
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
    </div>
  );
}