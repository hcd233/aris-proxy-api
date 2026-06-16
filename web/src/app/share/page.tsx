"use client";

/**
 * Public session share view — accessed via `/web/share/?id={uuid}`.
 *
 * Lives outside the `(dashboard)` route group so the `PermissionGuard` does
 * not redirect anonymous viewers to /login. Uses three public, IP-rate-limited
 * endpoints (metadata + paginated messages + paginated tools) with incremental
 * loading, matching the pattern used by the authenticated session detail page.
 *
 * Layout has two variants:
 *  - Mobile: claude.ai iOS-style sticky header + bottom-sheet tools panel,
 *    safe-area aware, full-bleed.
 *  - Desktop: original docked layout with a right-side tools sidebar.
 */

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type UIEvent,
} from "react";
import { useSearchParams } from "next/navigation";
import {
  Clock,
  MessagesSquare,
  Share2,
  Wrench,
} from "lucide-react";

import { api, ApiError } from "@/lib/api-client";
import type { ShareSessionMetadata } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import {
  ChatMessage,
  buildToolResultsByID,
} from "@/components/chat/chat-message";
import { ToolSidebarItem } from "@/components/session-detail/session-detail-client";
import { ToolDrawer } from "@/components/session-detail/tool-drawer";
import { SwipeDismissSheetBody } from "@/components/session-detail/swipe-dismiss-sheet-body";
import { useIsMobile } from "@/hooks/use-mobile";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import { formatRelativeTime } from "@/lib/utils";

// ─── Empty / error states ──────────────────────────────────────────────────

type ShareError =
  | { kind: "missing-id" }
  | { kind: "rate-limited" }
  | { kind: "not-found" }
  | { kind: "unknown"; message: string };

function ShareErrorView({ error }: { error: ShareError }) {
  const { title, description } = (() => {
    switch (error.kind) {
      case "missing-id":
        return {
          title: "Invalid share link",
          description:
            "This share link is missing a required identifier. Please ask the sender for a fresh link.",
        };
      case "rate-limited":
        return {
          title: "Too many requests",
          description:
            "You've opened this link too frequently. Please wait a moment and try again.",
        };
      case "not-found":
        return {
          title: "Link expired or unavailable",
          description:
            "This share link is no longer valid. It may have expired (after 24 hours) or been revoked by the owner.",
        };
      default:
        return {
          title: "Unable to load shared session",
          description: error.message,
        };
    }
  })();

  return (
    <div className="flex min-h-[100dvh] items-center justify-center bg-background px-4">
      <div className="w-full max-w-md rounded-3xl border bg-card p-8 text-center shadow-[0_24px_70px_rgba(92,62,29,0.14)]">
        <div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-muted">
          <Share2 className="size-5 text-muted-foreground" />
        </div>
        <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground">
          {title}
        </h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      </div>
    </div>
  );
}

// ─── Loading skeleton ──────────────────────────────────────────────────────

function ShareLoading({ mobile }: { mobile?: boolean }) {
  if (mobile) {
    return (
      <div className="flex min-h-[100dvh] flex-col bg-background pb-[calc(env(safe-area-inset-bottom)+1rem)]">
        <div className="border-b border-border/60 bg-background/85 supports-[backdrop-filter]:bg-background/70 supports-[backdrop-filter]:backdrop-blur">
          <div className="flex items-center gap-2 px-3 pt-[calc(env(safe-area-inset-top)+0.5rem)] pb-2">
            <Skeleton className="size-9 rounded-lg" />
            <div className="flex flex-1 flex-col gap-1">
              <Skeleton className="h-3.5 w-40" />
              <Skeleton className="h-2.5 w-28" />
            </div>
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
    <div className="mx-auto w-full max-w-3xl space-y-6 px-4 py-10 sm:px-6">
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

// ─── Main view ─────────────────────────────────────────────────────────────

function SharedSessionView() {
  const searchParams = useSearchParams();
  const shareID = searchParams.get("id") ?? "";
  const isMobile = useIsMobile();

  const [metadata, setMetadata] = useState<ShareSessionMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ShareError | null>(null);
  const [toolsDrawerOpen, setToolsDrawerOpen] = useState(false);
  const [toolsSheetOpen, setToolsSheetOpen] = useState(false);
  const [headerCompact, setHeaderCompact] = useState(false);
  const headerSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesScrollRootRef = useRef<HTMLDivElement | null>(null);
  const toolsScrollRootRef = useRef<HTMLDivElement | null>(null);
  const setToolsScrollRoot = useCallback((node: HTMLDivElement | null) => {
    toolsScrollRootRef.current = node;
  }, []);

  const fetchMetadata = useCallback(async () => {
    if (!shareID) {
      setError({ kind: "missing-id" });
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const rsp = await api.getShareMetadata(shareID);
      if (rsp.error) {
        setError({ kind: "not-found" });
        return;
      }
      if (!rsp.session) {
        setError({ kind: "not-found" });
        return;
      }
      setMetadata(rsp.session);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 404) {
          setError({ kind: "not-found" });
        } else if (err.status === 429) {
          setError({ kind: "rate-limited" });
        } else {
          setError({
            kind: "unknown",
            message: `Request failed (${err.status})`,
          });
        }
      } else {
        setError({
          kind: "unknown",
          message:
            err instanceof Error ? err.message : "Unexpected network error",
        });
      }
    } finally {
      setLoading(false);
    }
  }, [shareID]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchMetadata();
  }, [fetchMetadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // 只有 metadata 加载完成（shareID 有效）后才开始拉 messages/tools
  const listEnabled =
    !!shareID && metadata !== null;
  const toolsListEnabled =
    listEnabled &&
    (metadata?.toolCount ?? 0) > 0 &&
    ((!isMobile && toolsDrawerOpen) || (isMobile && toolsSheetOpen));

  const messagesList = useInfiniteList({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listShareMessages(shareID, page, limit);
        return {
          items: rsp.messages ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [shareID],
    ),
    pageSize: 50,
    enabled: listEnabled,
  });

  const toolsList = useInfiniteList({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listShareTools(shareID, page, limit);
        return {
          items: rsp.tools ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [shareID],
    ),
    pageSize: 20,
    enabled: toolsListEnabled,
  });

  const messagesHasMore = messagesList.hasMore;
  const messagesLoading = messagesList.loading;
  const loadMoreMessages = messagesList.loadMore;
  const toolsHasMore = toolsList.hasMore;
  const toolsLoading = toolsList.loading;
  const loadMoreTools = toolsList.loadMore;

  const isNearScrollBottom = useCallback((el: HTMLDivElement) => {
    return el.scrollHeight - el.scrollTop - el.clientHeight <= 240;
  }, []);

  const handleMessagesScroll = useCallback(
    (e: UIEvent<HTMLDivElement>) => {
      if (!messagesHasMore || messagesLoading) return;
      if (isNearScrollBottom(e.currentTarget)) {
        void loadMoreMessages();
      }
    },
    [isNearScrollBottom, loadMoreMessages, messagesHasMore, messagesLoading],
  );

  const handleToolsScroll = useCallback(
    (e: UIEvent<HTMLDivElement>) => {
      if (!toolsHasMore || toolsLoading) return;
      if (isNearScrollBottom(e.currentTarget)) {
        void loadMoreTools();
      }
    },
    [isNearScrollBottom, loadMoreTools, toolsHasMore, toolsLoading],
  );

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
      {
        root: messagesScrollRootRef.current,
        rootMargin: "200px",
      },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- refs are read at bind time; re-bind when layout mode or loader changes
  }, [isMobile, messagesList.hasMore, messagesList.loadMore]);

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
      {
        root: toolsScrollRootRef.current,
        rootMargin: "200px",
      },
    );
    io.observe(sentinel);
    return () => io.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- refs are read at bind time; re-bind when panel/sheet or loader changes
  }, [isMobile, toolsDrawerOpen, toolsSheetOpen, toolsList.hasMore, toolsList.loadMore]);

  const messages = messagesList.items;
  const tools = toolsList.items;
  const toolResultsByID = useMemo(
    () => buildToolResultsByID(messages),
    [messages],
  );

  if (error) return <ShareErrorView error={error} />;

  if (loading) {
    return <ShareLoading mobile={isMobile} />;
  }

  if (!metadata) return <ShareErrorView error={{ kind: "not-found" }} />;

  // ── Mobile layout ──────────────────────────────────────────────────────
  if (isMobile) {
    return (
      <div className="flex h-dvh flex-col bg-background overflow-hidden">
        <div ref={headerSentinelRef} aria-hidden className="h-px w-full shrink-0" />

        <header
          className={[
            "sticky top-0 z-30",
            "transition-[border-color,background-color,box-shadow] duration-200 ease-out",
            "supports-[backdrop-filter]:backdrop-blur",
            headerCompact
              ? "border-b border-border bg-background/92 supports-[backdrop-filter]:bg-background/75 shadow-[0_1px_0_rgba(0,0,0,0.04)]"
              : "border-b border-border/60 bg-background/85 supports-[backdrop-filter]:bg-background/70",
          ].join(" ")}
        >
          <div
            className={[
              "flex items-center gap-2.5 px-3",
              "pt-[calc(env(safe-area-inset-top)+0.5rem)]",
              "transition-[padding] duration-200 ease-out",
              headerCompact ? "pb-1.5" : "pb-2",
            ].join(" ")}
          >
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/15 text-primary">
              <Share2 className="size-[18px]" />
            </div>
            <div className="min-w-0 flex-1 leading-tight">
              <h1
                className={[
                  "truncate font-display font-semibold tracking-tight text-foreground",
                  "transition-[font-size] duration-200 ease-out",
                  headerCompact ? "text-[14px]" : "text-[15px]",
                ].join(" ")}
              >
                Shared session #{metadata.id}
              </h1>
              <p
                className={[
                  "truncate text-[11px] text-muted-foreground",
                  "transition-[max-height,opacity] duration-200 ease-out overflow-hidden",
                  headerCompact ? "max-h-0 opacity-0" : "max-h-4 opacity-100",
                ].join(" ")}
              >
                Read-only · {metadata.messageCount} message{metadata.messageCount === 1 ? "" : "s"}
              </p>
            </div>
            {metadata.toolCount > 0 && (
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => setToolsSheetOpen(true)}
                className="relative size-10 shrink-0 text-foreground/70 hover:text-foreground"
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
          ref={messagesScrollRootRef}
          onScroll={handleMessagesScroll}
          className={[
            "flex-1 overflow-y-auto px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+2.5rem)]",
            "overscroll-contain",
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
                "!h-[88dvh] max-h-[88dvh] rounded-t-[20px] border-border/70 p-0",
                "shadow-[0_-8px_32px_rgba(0,0,0,0.16)]",
                "flex flex-col",
                "!duration-[320ms] !ease-[cubic-bezier(0.32,0.72,0,1)]",
                "data-[side=bottom]:data-starting-style:!translate-y-[100%]",
                "data-[side=bottom]:data-ending-style:!translate-y-[100%]",
              ].join(" ")}
            >
              <SwipeDismissSheetBody
                onDismiss={() => setToolsSheetOpen(false)}
                title="Available Tools"
                count={metadata.toolCount}
                onScroll={handleToolsScroll}
                onScrollRootChange={setToolsScrollRoot}
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
      </div>
    );
  }

  // ── Desktop layout ─────────────────────────────────────────────────────
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-background text-foreground">
      <div className="flex min-h-0 flex-1">
        {/* Left metadata sidebar */}
        <aside className="hidden w-64 shrink-0 flex-col border-r border-border/70 bg-muted/30 px-4 py-5 md:flex">
          <h2 className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
            Shared session
          </h2>
          <h1 className="mt-3 font-display text-base font-semibold tracking-tight text-foreground">
            Session #{metadata.id}
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            {metadata.messageCount} message{metadata.messageCount === 1 ? "" : "s"}
          </p>
          <Separator className="my-4" />
          <div className="space-y-3 text-xs text-muted-foreground">
            <div className="flex items-center gap-2">
              <Clock className="size-3.5" />
              <span>{formatRelativeTime(metadata.createdAt)}</span>
            </div>
            <div className="flex items-center gap-2">
              <MessagesSquare className="size-3.5" />
              <span>
                {metadata.messageCount} message{metadata.messageCount === 1 ? "" : "s"}
              </span>
            </div>
          </div>
        </aside>

        {/* Main reading column */}
        <main className="flex min-w-0 flex-1 flex-col">
          <header className="sticky top-0 z-30 border-b border-border/70 bg-background/95 px-4 py-3 supports-[backdrop-filter]:backdrop-blur">
            <div className="mx-auto flex max-w-3xl items-center justify-between gap-3">
              <h1 className="flex-1 truncate text-center text-sm font-semibold text-foreground">
                Shared session #{metadata.id}
              </h1>
              <div className="flex items-center gap-1">
                {metadata.toolCount > 0 && (
                  <ToolDrawer
                    open={toolsDrawerOpen}
                    onOpenChange={setToolsDrawerOpen}
                    toolCount={metadata.toolCount}
                    onScroll={handleToolsScroll}
                    scrollRootRef={toolsScrollRootRef}
                  >
                    <div className="space-y-2">
                      {tools.map((t) => (
                        <ToolSidebarItem key={t.id} tool={t} />
                      ))}
                      {toolsList.hasMore && (
                        <div className="flex justify-center py-3">
                          <Skeleton className="h-4 w-24" />
                        </div>
                      )}
                    </div>
                  </ToolDrawer>
                )}
              </div>
            </div>
          </header>

          <div
            ref={messagesScrollRootRef}
            onScroll={handleMessagesScroll}
            className="flex-1 overflow-y-auto"
          >
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
        </main>
      </div>
    </div>
  );
}

// `useSearchParams()` requires a Suspense boundary when the page is statically
// rendered (`output: "export"`); wrap the view component here so the build
// succeeds without bailing out to client-side rendering at the page root.
export default function SharedSessionPage() {
  return (
    <Suspense fallback={<ShareLoading />}>
      <SharedSessionView />
    </Suspense>
  );
}
