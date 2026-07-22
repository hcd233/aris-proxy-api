"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchParams } from "next/navigation";
import { MessagesSquare, Share2, Wrench } from "lucide-react";

import { api, ApiError } from "@/lib/api-client";
import type { ShareSessionMetadata, MessageItem, ToolItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ChatMessage,
  buildToolResultsByID,
} from "@/components/chat/chat-message";
import { ToolSidebarItem } from "@/components/session-detail/tool-sidebar-item";
import { useIsMobile } from "@/hooks/use-mobile";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import { formatRelativeTime } from "@/lib/utils";
import { ReadingLayout } from "@/components/shared/reading-layout";
import { LocaleFade } from "@/components/locale-fade";
import { useT } from "@/lib/i18n";

type ShareError =
  | { kind: "missing-id" }
  | { kind: "rate-limited" }
  | { kind: "not-found" }
  | { kind: "unknown"; message: string };

function ShareErrorView({ error }: { error: ShareError }) {
  const t = useT();
  const { title, description } = (() => {
    switch (error.kind) {
      case "missing-id":
        return {
          title: t("share.invalid_link"),
          description: t("share.invalid_link_desc"),
        };
      case "rate-limited":
        return {
          title: t("error.too_many_requests"),
          description: t("share.rate_limited_desc"),
        };
      case "not-found":
        return {
          title: t("share.expired"),
          description: t("share.expired_desc"),
        };
      default:
        return {
          title: t("share.load_error"),
          description: error.message,
        };
    }
  })();

  return (
    <div className="flex min-h-[100dvh] items-center justify-center bg-background px-4">
      <div className="w-full max-w-md rounded-3xl border border-border/70 bg-card p-8 text-center shadow-xl">
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

function ShareLoading() {
  return (
    <div className="mx-auto w-full max-w-[768px] space-y-5 px-4 py-10 sm:px-6">
      <Skeleton className="h-8 w-48" />
      <div className="space-y-5">
        <Skeleton className="ml-auto h-20 w-3/4 rounded-[20px]" />
        <Skeleton className="h-32 w-full rounded-xl" />
        <Skeleton className="ml-auto h-16 w-2/3 rounded-[20px]" />
        <Skeleton className="h-24 w-full rounded-xl" />
      </div>
    </div>
  );
}

function SharedSessionView() {
  const searchParams = useSearchParams();
  const shareID = searchParams.get("id") ?? "";
  const isMobile = useIsMobile();
  const t = useT();

  const [metadata, setMetadata] = useState<ShareSessionMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ShareError | null>(null);
  const [toolsOpen, setToolsOpen] = useState(false);
  const [headerCompact, setHeaderCompact] = useState(false);
  const headerSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesScrollRootRef = useRef<HTMLDivElement | null>(null);
  const toolsScrollRootRef = useRef<HTMLDivElement | null>(null);

  const fetchMetadata = useCallback(async () => {
    if (!shareID) {
      setError({ kind: "missing-id" });
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const rsp = await api.getShareMetadata(shareID);
      if (rsp.error || !rsp.session) {
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
            message: t("share_page.request_failed").replace("{status}", String(err.status)),
          });
        }
      } else {
        setError({
          kind: "unknown",
          message:
            err instanceof Error ? err.message : t("share_page.network_error"),
        });
      }
    } finally {
      setLoading(false);
    }
  }, [shareID, t]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchMetadata();
  }, [fetchMetadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const listEnabled = !!shareID && metadata !== null;
  const toolsListEnabled =
    listEnabled &&
    (metadata?.toolCount ?? 0) > 0 &&
    toolsOpen;

  const messagesList = useInfiniteList<MessageItem>({
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

  const toolsList = useInfiniteList<ToolItem>({
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isMobile, messagesList.hasMore, messagesList.loadMore]);

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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [toolsOpen, isMobile, toolsList.hasMore, toolsList.loadMore]);

  const messages = messagesList.items;
  const tools = toolsList.items;
  const toolResultsByID = useMemo(
    () => buildToolResultsByID(messages),
    [messages],
  );

  const setToolsScrollRoot = useCallback((node: HTMLDivElement | null) => {
    toolsScrollRootRef.current = node;
  }, []);

  if (error) return <ShareErrorView error={error} />;
  if (loading) return <ShareLoading />;
  if (!metadata) return <ShareErrorView error={{ kind: "not-found" }} />;

  const headerContent = (
    <>
      <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/15 text-primary">
        <Share2 className="size-[18px]" />
      </div>
      <div className="flex min-w-0 flex-1 flex-col items-center leading-tight">
        <h1
          className={[
            "truncate font-display font-semibold tracking-tight text-foreground",
            "transition-[font-size] duration-200 ease-out",
            isMobile && headerCompact ? "text-[14px]" : "text-[15px]",
          ].filter(Boolean).join(" ")}
        >
          {t("share.session_title").replace("{id}", String(metadata.id))}
        </h1>
        <p
          className={[
            "truncate text-[11px] text-muted-foreground",
            "transition-[max-height,opacity] duration-200 ease-out overflow-hidden",
            isMobile && headerCompact ? "max-h-0 opacity-0" : "max-h-4 opacity-100",
          ].filter(Boolean).join(" ")}
        >
          {formatRelativeTime(metadata.createdAt)} · {metadata.messageCount} {t("sessions.messages").toLowerCase()}
        </p>
      </div>
      {metadata.toolCount > 0 && (
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => setToolsOpen((v) => !v)}
          className={[
            "relative size-10 shrink-0",
            toolsOpen
              ? "bg-secondary text-foreground"
              : "text-foreground/70 hover:text-foreground",
          ].join(" ")}
          aria-label={t("share.toggle_tools")}
          title={t("share.available_tools")}
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
    </>
  );

  const readingContent = (
    <>
      {messages.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <MessagesSquare className="mb-3 size-10 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">
            {t("share.no_messages")}
          </p>
        </div>
      ) : (
        <div className="space-y-5">
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
                {t("share.end_of_conversation")}
              </span>
            </div>
          )}
        </div>
      )}
    </>
  );

  const toolsPanelContent = (
    <div className="space-y-2">
      {tools.map((t) => (
        <ToolSidebarItem key={t.id} tool={t} />
      ))}
      {toolsList.hasMore && (
        <div ref={toolsSentinelRef} className="flex justify-center py-3">
          <Skeleton className="h-4 w-24" />
        </div>
      )}
    </div>
  );

  return (
    <LocaleFade>
      <ReadingLayout
        header={headerContent}
        toolsPanel={toolsPanelContent}
        toolsOpen={toolsOpen}
        onToolsOpenChange={setToolsOpen}
        toolsCount={metadata.toolCount}
        headerCompact={isMobile && headerCompact}
        headerSentinelRef={headerSentinelRef}
        messagesScrollRootRef={messagesScrollRootRef}
        onToolsScrollRootChange={setToolsScrollRoot}
      >
        {readingContent}
      </ReadingLayout>
    </LocaleFade>
  );
}

export default function SharedSessionPage() {
  return (
    <Suspense fallback={<ShareLoading />}>
      <SharedSessionView />
    </Suspense>
  );
}
