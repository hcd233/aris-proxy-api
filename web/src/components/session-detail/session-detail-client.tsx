"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Braces,
  ChevronDown,
  ChevronRight,
  Clock,
  Copy,
  FileText,
  Hash,
  MessagesSquare,
  PanelRightClose,
  PanelRightOpen,
  Share2,
  Wrench,
} from "lucide-react";
import { toast } from "sonner";

import { api } from "@/lib/api-client";
import type { SessionDetail, ToolItem, UnifiedTool } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import {
  ChatMessage,
  buildToolResultsByID,
} from "@/components/chat/chat-message";
import { ShareDialog, buildShareURL } from "@/components/share/share-dialog";

function CollapsibleText({
  text,
  previewChars = 140,
  className,
}: {
  text: string;
  previewChars?: number;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const trimmed = text.trim();
  const isLong = trimmed.length > previewChars;
  const display = !isLong || open ? trimmed : `${trimmed.slice(0, previewChars).trimEnd()}…`;

  return (
    <div className={className}>
      <p className="whitespace-pre-wrap break-words">{display}</p>
      {isLong && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            setOpen((v) => !v);
          }}
          className="mt-1 inline-flex items-center gap-0.5 font-medium text-primary/90 transition-colors hover:text-primary"
        >
          {open ? "Show less" : "Show more"}
          {open ? (
            <ChevronDown className="size-3" />
          ) : (
            <ChevronRight className="size-3" />
          )}
        </button>
      )}
    </div>
  );
}

function ToolSidebarItem({ tool }: { tool: ToolItem }) {
  const [expanded, setExpanded] = useState(false);
  const toolData: UnifiedTool = tool.tool;

  const params = toolData.parameters;
  const paramProperties =
    (params?.properties as Record<string, Record<string, unknown>>) ?? {};
  const requiredParams = (params?.required as string[]) ?? [];

  return (
    <div className="rounded-lg border border-border/70 bg-card/60">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-2.5 px-3 py-2.5 text-left transition-colors hover:bg-accent/40"
      >
        <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary/15 text-primary">
          <Wrench className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-mono text-[13px] font-medium text-foreground">
            {toolData.name}
          </p>
          <p className="truncate text-[11px] leading-snug text-muted-foreground">
            {toolData.description || "No description"}
          </p>
        </div>
        {expanded ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {expanded && (
        <div className="space-y-3 border-t border-border/60 px-3 py-3">
          {toolData.description && (
            <div>
              <p className="mb-1 flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                <FileText className="size-3" />
                Description
              </p>
              <CollapsibleText
                text={toolData.description}
                previewChars={140}
                className="text-[12.5px] leading-relaxed text-foreground/85"
              />
            </div>
          )}
          {Object.keys(paramProperties).length > 0 && (
            <div>
              <p className="mb-1.5 flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                <Braces className="size-3" />
                Parameters
              </p>
              <div className="space-y-1.5">
                {Object.entries(paramProperties).map(([name, schema]) => (
                  <div
                    key={name}
                    className="rounded-md bg-muted/50 px-2.5 py-1.5"
                  >
                    <div className="flex items-center gap-1.5">
                      <span className="font-mono text-[12px] font-medium text-foreground">
                        {name}
                      </span>
                      {requiredParams.includes(name) && (
                        <span className="text-[9px] font-medium uppercase tracking-wider text-rose-500">
                          required
                        </span>
                      )}
                      {schema.type !== undefined && (
                        <Badge
                          variant="secondary"
                          className="ml-auto px-1.5 py-0 font-mono text-[9px]"
                        >
                          {schema.type as string}
                        </Badge>
                      )}
                    </div>
                    {schema.description !== undefined && (
                      <CollapsibleText
                        text={schema.description as string}
                        previewChars={100}
                        className="mt-1 text-[11px] leading-relaxed text-muted-foreground"
                      />
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
          {params?.type !== undefined && (
            <div className="flex items-center gap-1.5 text-[10px] text-muted-foreground/60">
              <Hash className="size-3" />
              Schema type: {params.type as string}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default function SessionDetailClient({ sessionId }: { sessionId: number }) {
  const router = useRouter();
  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [shareOpen, setShareOpen] = useState(false);

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

  const messages = useMemo(() => session?.messages ?? [], [session]);
  const tools = useMemo(() => session?.tools ?? [], [session]);
  const toolResultsByID = useMemo(
    () => buildToolResultsByID(messages),
    [messages],
  );

  if (!sessionId || Number.isNaN(sessionId)) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-muted-foreground">Invalid session ID</p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={() => router.push("/sessions/")}
        >
          Back to Sessions
        </Button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="mx-auto w-full max-w-3xl space-y-6 py-6">
        <Skeleton className="h-8 w-48" />
        <div className="space-y-6">
          <Skeleton className="ml-auto h-20 w-3/4 rounded-2xl" />
          <Skeleton className="h-32 w-full rounded-xl" />
          <Skeleton className="ml-auto h-16 w-2/3 rounded-2xl" />
          <Skeleton className="h-24 w-full rounded-xl" />
        </div>
      </div>
    );
  }

  if (!session) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <p className="text-muted-foreground">Session not found</p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={() => router.push("/sessions/")}
        >
          Back to Sessions
        </Button>
      </div>
    );
  }

  const messageCount = messages.filter(
    (m) => m.message.role !== "tool" && !m.message.tool_call_id,
  ).length;

  return (
    <div className="flex h-[calc(100vh-6rem)] gap-0 overflow-hidden">
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-3 border-b border-border/70 pb-4">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => router.push("/sessions/")}
            className="text-muted-foreground hover:text-foreground"
            aria-label="Back to sessions"
          >
            <ArrowLeft className="size-4" />
          </Button>
          <div className="flex min-w-0 items-center gap-3">
            <h1 className="font-display text-xl font-semibold tracking-tight text-foreground">
              Session #{session.id}
            </h1>
            {session.apiKeyName && (
              <Badge variant="secondary" className="text-xs">
                {session.apiKeyName}
              </Badge>
            )}
            {session.isShared && (
              <Badge variant="outline" className="gap-1 text-xs">
                Shared
                {session.shareID && (
                  <button
                    type="button"
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(buildShareURL(session.shareID!));
                        toast.success("Share link copied");
                      } catch {
                        toast.error("Failed to copy link");
                      }
                    }}
                    className="ml-0.5 text-primary hover:text-primary/80"
                    title="Copy share link"
                  >
                    <Copy className="size-3" />
                  </button>
                )}
              </Badge>
            )}
            <span className="hidden items-center gap-1 text-xs text-muted-foreground sm:flex">
              <MessagesSquare className="size-3.5" />
              {messageCount} message{messageCount === 1 ? "" : "s"}
            </span>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <div className="hidden items-center gap-1.5 text-xs text-muted-foreground md:flex">
              <Clock className="size-3.5" />
              <span>{new Date(session.createdAt).toLocaleString()}</span>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShareOpen(true)}
              className="gap-1.5"
              title="Create a public share link"
            >
              <Share2 className="size-3.5" />
              <span className="hidden sm:inline">Share</span>
            </Button>
            {tools.length > 0 && (
              <Button
                variant={sidebarOpen ? "secondary" : "ghost"}
                size="icon-sm"
                onClick={() => setSidebarOpen(!sidebarOpen)}
                title={sidebarOpen ? "Hide tools panel" : "Show tools panel"}
                aria-label={sidebarOpen ? "Hide tools panel" : "Show tools panel"}
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

        <div className="flex-1 overflow-y-auto">
          <div className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6">
            {messages.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-20 text-center">
                <MessagesSquare className="mb-3 size-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">
                  No messages in this session
                </p>
              </div>
            ) : (
              <div className="space-y-7">
                {messages.map((msg, idx) => (
                  <ChatMessage
                    key={msg.id}
                    message={msg}
                    index={idx}
                    toolResultsByID={toolResultsByID}
                    messages={messages}
                  />
                ))}
                <div className="pt-4 pb-2 text-center">
                  <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground/50">
                    end of conversation
                  </span>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {sidebarOpen && tools.length > 0 && (
        <>
          <Separator orientation="vertical" className="mx-0 h-auto" />
          <aside className="flex w-80 shrink-0 flex-col overflow-hidden bg-sidebar/40">
            <div className="flex items-center gap-2 border-b border-border/70 px-4 py-3.5">
              <Wrench className="size-4 text-muted-foreground" />
              <h2 className="font-display text-sm font-semibold text-foreground">
                Available Tools
              </h2>
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

      <ShareDialog
        sessionId={session.id}
        existingShareID={session.shareID}
        open={shareOpen}
        onOpenChange={setShareOpen}
      />
    </div>
  );
}
