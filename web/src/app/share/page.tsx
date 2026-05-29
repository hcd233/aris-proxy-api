"use client";

/**
 * Public session share view — accessed via `/web/share/?id={uuid}`.
 *
 * Lives outside the `(dashboard)` route group so the `PermissionGuard` does
 * not redirect anonymous viewers to /login. Calls the public, IP-rate-limited
 * `GET /api/v1/session/share/?id=xxx` endpoint and renders the conversation with
 * the same `ChatMessage` component used by the authenticated detail page.
 *
 * Layout has two variants:
 *  - Mobile: claude.ai iOS-style sticky header + bottom-sheet tools panel,
 *    safe-area aware, full-bleed. Mirrors the authenticated session detail
 *    page so shared links feel like the real product on phones.
 *  - Desktop: original docked layout with a right-side tools sidebar.
 */

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import {
  Braces,
  ChevronDown,
  ChevronRight,
  Clock,
  FileText,
  Hash,
  MessagesSquare,
  PanelRightClose,
  PanelRightOpen,
  Share2,
  Wrench,
} from "lucide-react";

import { api, ApiError } from "@/lib/api-client";
import type {
  ShareContentSessionDetail,
  ToolItem,
  UnifiedTool,
} from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import {
  ChatMessage,
  buildToolResultsByID,
} from "@/components/chat/chat-message";
import { SwipeDismissSheetBody } from "@/components/session-detail/swipe-dismiss-sheet-body";
import { useIsMobile } from "@/hooks/use-mobile";

// ─── Tool sidebar item (kept local to avoid import cycles) ──────────────────

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
  const display =
    !isLong || open ? trimmed : `${trimmed.slice(0, previewChars).trimEnd()}…`;

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

// ─── Page ──────────────────────────────────────────────────────────────────

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

function SharedSessionView() {
  const searchParams = useSearchParams();
  const shareID = searchParams.get("id") ?? "";
  const isMobile = useIsMobile();

  const [session, setSession] = useState<ShareContentSessionDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ShareError | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [toolsSheetOpen, setToolsSheetOpen] = useState(false);
  const [headerCompact, setHeaderCompact] = useState(false);
  const headerSentinelRef = useRef<HTMLDivElement | null>(null);

  const fetchSession = useCallback(async () => {
    if (!shareID) {
      setError({ kind: "missing-id" });
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const rsp = await api.getShareContent(shareID);
      if (rsp.error) {
        setError({ kind: "not-found" });
        return;
      }
      if (!rsp.session) {
        setError({ kind: "not-found" });
        return;
      }
      setSession(rsp.session);
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
    fetchSession();
  }, [fetchSession]);
  /* eslint-enable react-hooks/set-state-in-effect */

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
  }, [isMobile, loading, session]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const messages = useMemo(() => session?.messages ?? [], [session]);
  const tools = useMemo(() => session?.tools ?? [], [session]);
  const toolResultsByID = useMemo(
    () => buildToolResultsByID(messages),
    [messages],
  );

  if (error) return <ShareErrorView error={error} />;

  if (loading) {
    return <ShareLoading mobile={isMobile} />;
  }

  if (!session) return <ShareErrorView error={{ kind: "not-found" }} />;

  const messageCount = messages.filter(
    (m) => m.message.role !== "tool" && !m.message.tool_call_id,
  ).length;

  // ── Mobile layout ──────────────────────────────────────────────────────
  if (isMobile) {
    return (
      <div className="flex min-h-[100dvh] flex-col bg-background pb-[calc(env(safe-area-inset-bottom)+1rem)]">
        <div ref={headerSentinelRef} aria-hidden className="h-px w-full" />

        {/* Sticky chrome — uses iOS-style status-bar safe-area top padding so
            the share badge docks against the device bezel. */}
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
                Shared session #{session.id}
              </h1>
              <p
                className={[
                  "truncate text-[11px] text-muted-foreground",
                  "transition-[max-height,opacity] duration-200 ease-out overflow-hidden",
                  headerCompact ? "max-h-0 opacity-0" : "max-h-4 opacity-100",
                ].join(" ")}
              >
                Read-only · {messageCount} message{messageCount === 1 ? "" : "s"}
              </p>
            </div>
            {tools.length > 0 && (
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
                  {tools.length}
                </span>
              </Button>
            )}
          </div>
        </header>

        {/* Conversation column */}
        <div
          className={[
            "flex-1 px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+2.5rem)]",
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
                "shadow-[0_-8px_32px_rgba(0,0,0,0.16)]",
                "flex flex-col",
                "touch-pan-y",
                "!duration-[320ms] !ease-[cubic-bezier(0.32,0.72,0,1)]",
                "data-[side=bottom]:data-starting-style:!translate-y-[100%]",
                "data-[side=bottom]:data-ending-style:!translate-y-[100%]",
              ].join(" ")}
            >
              <SwipeDismissSheetBody
                onDismiss={() => setToolsSheetOpen(false)}
                title="Available Tools"
                count={tools.length}
              >
                {tools.map((t) => (
                  <ToolSidebarItem key={t.id} tool={t} />
                ))}
              </SwipeDismissSheetBody>
            </SheetContent>
          </Sheet>
        )}
      </div>
    );
  }

  // ── Desktop layout (unchanged) ─────────────────────────────────────────
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-background text-foreground">
      {/* ── Top banner: shared session badge ── */}
      <div className="border-b border-border/70 bg-sidebar/40">
        <div className="mx-auto flex max-w-6xl items-center gap-3 px-4 py-3 sm:px-6">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary/15 text-primary">
            <Share2 className="size-4" />
          </div>
          <div className="min-w-0 flex-1">
            <h1 className="font-display text-base font-semibold tracking-tight text-foreground sm:text-lg">
              Shared session #{session.id}
            </h1>
            <p className="text-[11px] text-muted-foreground">
              Read-only view · Generated by Aris Proxy
            </p>
          </div>
          <div className="hidden items-center gap-1.5 text-xs text-muted-foreground md:flex">
            <Clock className="size-3.5" />
            <span>{new Date(session.createdAt).toLocaleString()}</span>
          </div>
          <span className="hidden items-center gap-1 text-xs text-muted-foreground sm:flex">
            <MessagesSquare className="size-3.5" />
            {messageCount} message{messageCount === 1 ? "" : "s"}
          </span>
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

      {/* ── Main content ── */}
      <div className="flex min-h-0 flex-1 gap-0 overflow-hidden">
        <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
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
