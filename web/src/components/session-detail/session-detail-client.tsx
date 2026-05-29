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
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import type {
  SessionMetadata,
  MessageItem,
  ToolItem,
  UnifiedTool,
} from "@/lib/types";
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
import { useInfiniteList } from "@/hooks/use-infinite-list";
import {
  Sheet,
  SheetContent,
} from "@/components/ui/sheet";
import { SwipeDismissSheetBody } from "@/components/session-detail/swipe-dismiss-sheet-body";

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
  const [metadata, setMetadata] = useState<SessionMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  // Desktop: tools panel docked open by default. Mobile: closed; user opens
  // the bottom sheet on demand.
  const [toolsPanelOpen, setToolsPanelOpen] = useState(true);
  const [toolsSheetOpen, setToolsSheetOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  // Mobile only: tracks whether the sticky header has been "pushed" — i.e. the
  // top sentinel has scrolled out of view. Drives the compact header variant.
  const [headerCompact, setHeaderCompact] = useState(false);
  const headerSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsSentinelRef = useRef<HTMLDivElement | null>(null);

  /* eslint-disable react-hooks/set-state-in-effect -- IntersectionObserver callback inherently sets state on visibility changes */
  useEffect(() => {
    if (!isMobile) {
      setHeaderCompact(false);
      return;
    }
    const sentinel = headerSentinelRef.current;
    if (!sentinel) return;
    const io = new IntersectionObserver(
      ([entry]) => setHeaderCompact(!entry.isIntersecting),
      { threshold: 0, rootMargin: "0px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
  }, [isMobile, loading, metadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const fetchMetadata = useCallback(async () => {
    if (!sessionId || Number.isNaN(sessionId)) return;
    setLoading(true);
    try {
      const rsp = await api.getSessionMetadata(sessionId);
      if (rsp.session) setMetadata(rsp.session);
    } catch {
      // handled silently
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchMetadata();
  }, [fetchMetadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // 只有 metadata 加载完成（且权限通过）后才开始拉 messages，避免越权请求多打一次。
  const listEnabled =
    !!sessionId && !Number.isNaN(sessionId) && metadata !== null;
  const toolsListEnabled =
    listEnabled &&
    (metadata?.toolCount ?? 0) > 0 &&
    ((!isMobile && toolsPanelOpen) || (isMobile && toolsSheetOpen));

  const messagesList = useInfiniteList<MessageItem>({
    fetcher: useCallback(
      async (offset, limit) => {
        const rsp = await api.listSessionMessages(sessionId, offset, limit);
        return {
          items: rsp.messages ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [sessionId],
    ),
    pageSize: 20,
    enabled: listEnabled,
  });

  const toolsList = useInfiniteList<ToolItem>({
    fetcher: useCallback(
      async (offset, limit) => {
        const rsp = await api.listSessionTools(sessionId, offset, limit);
        return {
          items: rsp.tools ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [sessionId],
    ),
    pageSize: 50,
    enabled: toolsListEnabled,
  });

  // messages 滚动加载 sentinel
  useEffect(() => {
    const sentinel = messagesSentinelRef.current;
    if (!sentinel || !messagesList.hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void messagesList.loadMore();
        }
      },
      { rootMargin: "200px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-bind IO when hasMore/loadMore identity changes
  }, [messagesList.hasMore, messagesList.loadMore]);

  // tools 滚动加载 sentinel
  useEffect(() => {
    const sentinel = toolsSentinelRef.current;
    if (!sentinel || !toolsList.hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void toolsList.loadMore();
        }
      },
      { rootMargin: "200px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-bind IO when hasMore/loadMore identity changes
  }, [toolsList.hasMore, toolsList.loadMore]);

  const messages = messagesList.items;
  const tools = toolsList.items;
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
    if (isMobile) {
      // Mobile skeleton mirrors the real iOS layout: chrome bar with title
      // placeholder, then alternating user-bubble + assistant-prose blocks.
      return (
        <div className="-mx-4 -mt-4 flex min-h-[calc(100dvh-3.5rem)] flex-col bg-background pb-[calc(env(safe-area-inset-bottom)+1rem)]">
          <div className="border-b border-border/60 bg-background/85 supports-[backdrop-filter]:bg-background/70 supports-[backdrop-filter]:backdrop-blur">
            <div className="flex items-center gap-2 px-2 pt-[calc(1rem+0.25rem)] pb-2">
              <Skeleton className="size-10 rounded-full" />
              <div className="flex flex-1 flex-col items-center gap-1">
                <Skeleton className="h-3.5 w-24" />
                <Skeleton className="h-2.5 w-32" />
              </div>
              <Skeleton className="size-10 rounded-full" />
              <Skeleton className="size-10 rounded-full" />
            </div>
          </div>
          <div className="space-y-6 px-4 pt-5">
            <div className="flex justify-end">
              <Skeleton className="h-16 w-3/4 rounded-3xl" />
            </div>
            <div className="space-y-2">
              <Skeleton className="h-3 w-16" />
              <Skeleton className="h-4 w-11/12" />
              <Skeleton className="h-4 w-10/12" />
              <Skeleton className="h-4 w-8/12" />
            </div>
            <div className="flex justify-end">
              <Skeleton className="h-12 w-2/3 rounded-3xl" />
            </div>
            <div className="space-y-2">
              <Skeleton className="h-4 w-11/12" />
              <Skeleton className="h-4 w-9/12" />
            </div>
          </div>
        </div>
      );
    }
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

  if (!metadata) {
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

  // metadata.messageCount 直接来自后端（含 tool messages），与 SessionSummary 字段一致。
  // 若产品对"非 tool 消息数"敏感可后续扩字段。
  const messageCount = metadata.messageCount;

  // ── Mobile layout (claude.ai iOS-style) ─────────────────────────────────
  // Escape the dashboard's outer padding so the column itself owns the
  // gutters. We bleed horizontally (-mx-4) and pull up vertically (-mt-4)
  // so the sticky header can dock against the dashboard's mobile top bar
  // without a 16px gap from the parent <main>'s padding-top.
  if (isMobile) {
    return (
      <div className="-mx-4 -mt-4 flex min-h-[calc(100dvh-3.5rem)] flex-col bg-background pb-[calc(env(safe-area-inset-bottom)+1rem)]">
        {/*
          Sentinel sits at the very top of the bled-out container. When it
          scrolls out of view (stuck = true), we switch the header to its
          compact variant.
        */}
        <div ref={headerSentinelRef} aria-hidden className="h-px w-full" />

        {/* iOS-style sticky chrome — collapses on scroll.
            `top: -1rem` cancels the parent <main>'s 16px padding-top so the
            header docks flush against the dashboard's mobile top bar; the
            inner div carries the visible chrome and absorbs that 1rem with
            its own padding so visuals don't shift. */}
        <header
          className={[
            "sticky top-[-1rem] z-30 -mt-px",
            "transition-[border-color,background-color,box-shadow] duration-200 ease-out",
            "supports-[backdrop-filter]:backdrop-blur",
            headerCompact
              ? "border-b border-border bg-background/92 supports-[backdrop-filter]:bg-background/75 shadow-[0_1px_0_rgba(0,0,0,0.04)]"
              : "border-b border-border/60 bg-background/85 supports-[backdrop-filter]:bg-background/70",
          ].join(" ")}
        >
          {/* 1rem top spacer absorbs the negative top so content stays in place
              while the chrome's background extends up under the dashboard bar. */}
          <div
            className={[
              "flex items-center gap-1 px-2 pt-[calc(1rem+0.25rem)]",
              "transition-[padding] duration-200 ease-out",
              headerCompact ? "pb-1.5" : "pb-2",
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
              <h1
                className={[
                  "truncate font-display font-semibold tracking-tight text-foreground",
                  "transition-[font-size] duration-200 ease-out",
                  headerCompact ? "text-[14px]" : "text-[15px]",
                ].join(" ")}
              >
                Session #{metadata.id}
              </h1>
              <p
                className={[
                  "truncate text-[11px] text-muted-foreground",
                  "transition-[max-height,opacity] duration-200 ease-out overflow-hidden",
                  headerCompact ? "max-h-0 opacity-0" : "max-h-4 opacity-100",
                ].join(" ")}
              >
                {messageCount} message{messageCount === 1 ? "" : "s"}
                {metadata.apiKeyName ? ` · ${metadata.apiKeyName}` : ""}
              </p>
            </div>

            <Button
              variant="ghost"
              size="icon-sm"
              onClick={() => setShareOpen(true)}
              className={[
                "size-10",
                metadata.shareID
                  ? "text-primary"
                  : "text-foreground/70 hover:text-foreground",
              ].join(" ")}
              aria-label={metadata.shareID ? "Manage share link" : "Share session"}
              title={metadata.shareID ? "Shared" : "Share"}
            >
              <Share2 className="size-5" />
            </Button>

            {metadata.toolCount > 0 && (
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
                  {metadata.toolCount}
                </span>
              </Button>
            )}
          </div>
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
              {messagesList.hasMore && (
                <div
                  ref={messagesSentinelRef}
                  className="flex justify-center py-3"
                >
                  <Skeleton className="h-4 w-32" />
                </div>
              )}
              {!messagesList.hasMore && messages.length > 0 && (
                <div className="pt-3 pb-1 text-center">
                  <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground/50">
                    end of conversation
                  </span>
                </div>
              )}
            </div>
          )}
        </div>

        {/* iOS-style bottom sheet for available tools */}
        {metadata.toolCount > 0 && (
          <Sheet open={toolsSheetOpen} onOpenChange={setToolsSheetOpen}>
            <SheetContent
              side="bottom"
              showCloseButton={false}
              className={[
                "h-[88dvh] max-h-[88dvh] rounded-t-[20px] border-border/70 p-0",
                "shadow-[0_-8px_32px_rgba(0,0,0,0.16)]",
                "flex flex-col",
                // touch-action: pan-y so vertical drags reach our handlers
                // before the browser starts its own scroll on parent layers.
                "touch-pan-y",
                // Override base-ui's default 200ms ease-in-out enter/exit:
                // a slightly longer duration with iOS spring-style easing
                // makes the sheet feel native instead of snapping into view.
                "!duration-[320ms] !ease-[cubic-bezier(0.32,0.72,0,1)]",
                // Slide all the way from below the viewport edge for a more
                // pronounced (and softer-looking) entrance than the default
                // 2.5rem offset. The arbitrary 100% bypasses the library's
                // built-in `data-starting-style:translate-y-[2.5rem]`.
                "data-[side=bottom]:data-starting-style:!translate-y-[100%]",
                "data-[side=bottom]:data-ending-style:!translate-y-[100%]",
              ].join(" ")}
            >
              <SwipeDismissSheetBody
                onDismiss={() => setToolsSheetOpen(false)}
                title="Available Tools"
                count={metadata.toolCount}
              >
                {tools.map((t) => (
                  <ToolSidebarItem key={t.id} tool={t} />
                ))}
                {toolsList.hasMore && (
                  <div
                    ref={toolsSentinelRef}
                    className="flex justify-center py-3"
                  >
                    <Skeleton className="h-4 w-24" />
                  </div>
                )}
              </SwipeDismissSheetBody>
            </SheetContent>
          </Sheet>
        )}

        <ShareDialog
          sessionId={metadata.id}
          existingShareID={metadata.shareID}
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
              Session #{metadata.id}
            </h1>
            {metadata.apiKeyName && (
              <Badge variant="secondary" className="text-xs">
                {metadata.apiKeyName}
              </Badge>
            )}
            <span className="hidden items-center gap-1 text-xs text-muted-foreground sm:flex">
              <MessagesSquare className="size-3.5" />
              {messageCount} message{messageCount === 1 ? "" : "s"}
            </span>
          </div>
          <div className="ml-auto flex items-center gap-2">
            <div className="hidden items-center gap-1.5 text-xs text-muted-foreground md:flex">
              <span>{new Date(metadata.createdAt).toLocaleString()}</span>
            </div>
            <Button
              variant={metadata.shareID ? "secondary" : "outline"}
              size="sm"
              onClick={() => setShareOpen(true)}
              className="gap-1.5"
              title={metadata.shareID ? "Manage share link" : "Create a public share link"}
            >
              <Share2 className="size-3.5" />
              <span className="hidden sm:inline">{metadata.shareID ? "Shared" : "Share"}</span>
            </Button>
            {metadata.toolCount > 0 && (
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
                  />
                ))}
                {messagesList.hasMore && (
                  <div
                    ref={messagesSentinelRef}
                    className="flex justify-center py-4"
                  >
                    <Skeleton className="h-4 w-32" />
                  </div>
                )}
                {!messagesList.hasMore && messages.length > 0 && (
                  <div className="pt-4 pb-2 text-center">
                    <span className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground/50">
                      end of conversation
                    </span>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {toolsPanelOpen && metadata.toolCount > 0 && (
        <>
          <Separator orientation="vertical" className="mx-0 h-auto" />
          <aside className="flex w-80 shrink-0 flex-col overflow-hidden bg-sidebar/40">
            <div className="flex items-center gap-2 border-b border-border/70 px-4 py-3.5">
              <Wrench className="size-4 text-muted-foreground" />
              <h2 className="font-display text-sm font-semibold text-foreground">
                Available Tools
              </h2>
              <Badge variant="secondary" className="ml-auto text-[10px]">
                {metadata.toolCount}
              </Badge>
            </div>
            <div className="flex-1 space-y-2 overflow-y-auto p-3">
              {tools.map((t) => (
                <ToolSidebarItem key={t.id} tool={t} />
              ))}
              {toolsList.hasMore && (
                <div
                  ref={toolsSentinelRef}
                  className="flex justify-center py-3"
                >
                  <Skeleton className="h-4 w-24" />
                </div>
              )}
            </div>
          </aside>
        </>
      )}

      <ShareDialog
        sessionId={metadata.id}
        existingShareID={metadata.shareID}
        open={shareOpen}
        onOpenChange={setShareOpen}
      />
    </div>
  );
}
