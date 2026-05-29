"use client";

/**
 * Session detail view.
 *
 * Mobile layout is tuned to match the claude.ai iOS reading experience:
 *  - A slim sticky chrome bar (back / centered title / actions) with hairline
 *    border + backdrop blur, instead of a horizontally crowded toolbar.
 *  - The conversation column escapes the dashboard's outer padding via
 *    negative margins so the column itself sets the comfortable reading
 *    gutters (16px on phones, expanding on tablets).
 *  - Vertical sizing uses 100dvh and respects the iOS safe-area inset so
 *    the last assistant turn is never trapped under the home indicator.
 *  - The available-tools panel becomes a tall iOS-style bottom sheet with a
 *    grabber, sticky header, and safe-area aware scroll region.
 */
import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Braces,
  ChevronDown,
  ChevronRight,
  FileText,
  Hash,
  MessagesSquare,
  Share2,
  Wrench,
} from "lucide-react";
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
import { ShareDialog } from "@/components/share/share-dialog";
import { useIsMobile } from "@/hooks/use-mobile";
import {
  Sheet,
  SheetContent,
} from "@/components/ui/sheet";

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
    <div className="rounded-xl border border-border/70 bg-card/60">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex min-h-[52px] w-full items-center gap-3 px-3.5 py-2.5 text-left transition-colors active:bg-accent/50 md:hover:bg-accent/40"
      >
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/15 text-primary">
          <Wrench className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-mono text-[13.5px] font-medium text-foreground">
            {toolData.name}
          </p>
          <p className="truncate text-[12px] leading-snug text-muted-foreground">
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
        <div className="space-y-3 border-t border-border/60 px-3.5 py-3">
          {toolData.description && (
            <div>
              <p className="mb-1 flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                <FileText className="size-3" />
                Description
              </p>
              <CollapsibleText
                text={toolData.description}
                previewChars={140}
                className="text-[13px] leading-relaxed text-foreground/85"
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
                        className="mt-1 text-[11.5px] leading-relaxed text-muted-foreground"
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
  const isMobile = useIsMobile();
  const [session, setSession] = useState<SessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  // Desktop: tools panel docked open by default. Mobile: closed; user opens
  // the bottom sheet on demand.
  const [toolsPanelOpen, setToolsPanelOpen] = useState(true);
  const [toolsSheetOpen, setToolsSheetOpen] = useState(false);
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

  // ── Mobile layout (claude.ai iOS-style) ─────────────────────────────────
  // Escape the dashboard's outer padding so the column itself owns the
  // gutters. Use 100dvh minus the dashboard mobile top bar (h-14 = 3.5rem)
  // so the conversation reaches the home indicator with a safe-area pad.
  if (isMobile) {
    return (
      <div className="-mx-4 -my-4 flex min-h-[calc(100dvh-3.5rem)] flex-col bg-background">
        {/* iOS-style sticky chrome */}
        <header
          className={[
            "sticky top-0 z-30 flex items-center gap-1 px-2 py-2",
            "border-b border-border/60",
            "bg-background/85 backdrop-blur supports-[backdrop-filter]:bg-background/70",
          ].join(" ")}
        >
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => router.push("/sessions/")}
            className="size-10 text-foreground/70 hover:text-foreground"
            aria-label="Back to sessions"
          >
            <ArrowLeft className="size-5" />
          </Button>

          <div className="flex min-w-0 flex-1 flex-col items-center px-1 leading-tight">
            <h1 className="truncate font-display text-[15px] font-semibold tracking-tight text-foreground">
              Session #{session.id}
            </h1>
            <p className="truncate text-[11px] text-muted-foreground">
              {messageCount} message{messageCount === 1 ? "" : "s"}
              {session.apiKeyName ? ` · ${session.apiKeyName}` : ""}
            </p>
          </div>

          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setShareOpen(true)}
            className={[
              "size-10",
              session.shareID
                ? "text-primary"
                : "text-foreground/70 hover:text-foreground",
            ].join(" ")}
            aria-label={session.shareID ? "Manage share link" : "Share session"}
            title={session.shareID ? "Shared" : "Share"}
          >
            <Share2 className="size-5" />
          </Button>

          {tools.length > 0 && (
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => setToolsSheetOpen(true)}
              className="relative size-10 text-foreground/70 hover:text-foreground"
              aria-label="Show available tools"
              title="Available tools"
            >
              <Wrench className="size-5" />
              <span
                className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold tabular-nums text-primary-foreground"
                aria-hidden
              >
                {tools.length}
              </span>
            </Button>
          )}
        </header>

        {/* Conversation column */}
        <div
          className={[
            "flex-1",
            "px-4",
            // Generous bottom padding + iOS home-indicator safe area so the
            // last assistant message never gets trapped under the OS chrome.
            "pt-5 pb-[calc(env(safe-area-inset-bottom)+2.5rem)]",
            // Smoother native scroll on iOS, no rubber-band at edges.
            "[-webkit-overflow-scrolling:touch] overscroll-contain",
          ].join(" ")}
        >
          {messages.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <MessagesSquare className="mb-3 size-10 text-muted-foreground/40" />
              <p className="text-sm text-muted-foreground">
                No messages in this session
              </p>
            </div>
          ) : (
            <div className="space-y-6">
              {messages.map((msg, idx) => (
                <ChatMessage
                  key={msg.id}
                  message={msg}
                  index={idx}
                  toolResultsByID={toolResultsByID}
                />
              ))}
              <div className="pt-3 pb-1 text-center">
                <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground/50">
                  end of conversation
                </span>
              </div>
            </div>
          )}
        </div>

        {/* iOS-style bottom sheet for available tools */}
        {tools.length > 0 && (
          <Sheet open={toolsSheetOpen} onOpenChange={setToolsSheetOpen}>
            <SheetContent
              side="bottom"
              showCloseButton={false}
              className={[
                "h-[88dvh] max-h-[88dvh] rounded-t-[20px] border-border/70 p-0",
                "shadow-[0_-8px_24px_rgba(0,0,0,0.10)]",
                "flex flex-col",
              ].join(" ")}
            >
              {/* grabber */}
              <div className="flex justify-center pt-2.5 pb-1">
                <span
                  className="block h-1 w-9 rounded-full bg-foreground/20"
                  aria-hidden
                />
              </div>

              {/* sticky title row */}
              <div className="flex items-center gap-2 border-b border-border/60 px-4 pb-3">
                <Wrench className="size-4 text-muted-foreground" />
                <h2 className="font-display text-[15px] font-semibold text-foreground">
                  Available Tools
                </h2>
                <Badge variant="secondary" className="ml-1 text-[10px]">
                  {tools.length}
                </Badge>
                <button
                  type="button"
                  onClick={() => setToolsSheetOpen(false)}
                  className="ml-auto -mr-1 inline-flex h-9 items-center px-2 text-[14px] font-medium text-primary"
                >
                  Done
                </button>
              </div>

              <div
                className={[
                  "flex-1 space-y-2 overflow-y-auto px-3 pt-3",
                  "pb-[calc(env(safe-area-inset-bottom)+0.75rem)]",
                  "[-webkit-overflow-scrolling:touch] overscroll-contain",
                ].join(" ")}
              >
                {tools.map((t) => (
                  <ToolSidebarItem key={t.id} tool={t} />
                ))}
              </div>
            </SheetContent>
          </Sheet>
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

  // ── Desktop layout (unchanged behaviour) ────────────────────────────────
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
            <h1 className="font-display text-lg md:text-xl font-semibold tracking-tight text-foreground">
              Session #{session.id}
            </h1>
            {session.apiKeyName && (
              <Badge variant="secondary" className="text-xs">
                {session.apiKeyName}
              </Badge>
            )}
            <span className="hidden items-center gap-1 text-xs text-muted-foreground sm:flex">
              <MessagesSquare className="size-3.5" />
              {messageCount} message{messageCount === 1 ? "" : "s"}
            </span>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <div className="hidden items-center gap-1.5 text-xs text-muted-foreground md:flex">
              <span>{new Date(session.createdAt).toLocaleString()}</span>
            </div>
            <Button
              variant={session.shareID ? "secondary" : "outline"}
              size="sm"
              onClick={() => setShareOpen(true)}
              className="gap-1.5"
              title={session.shareID ? "Manage share link" : "Create a public share link"}
            >
              <Share2 className="size-3.5" />
              <span className="hidden sm:inline">{session.shareID ? "Shared" : "Share"}</span>
            </Button>
            {tools.length > 0 && (
              <Button
                variant={toolsPanelOpen ? "secondary" : "ghost"}
                size="icon-sm"
                onClick={() => setToolsPanelOpen(!toolsPanelOpen)}
                title={toolsPanelOpen ? "Hide tools panel" : "Show tools panel"}
                aria-label={toolsPanelOpen ? "Hide tools panel" : "Show tools panel"}
              >
                <Wrench className="size-4" />
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

      {toolsPanelOpen && tools.length > 0 && (
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
