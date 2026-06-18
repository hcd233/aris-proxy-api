"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  AlertTriangle,
  ArrowLeft,
  History,
  MessagesSquare,
  Share2,
  Trash2,
  Wrench,
} from "lucide-react";
import { api } from "@/lib/api-client";
import type { SessionMetadata, MessageItem, ToolItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
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
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { SessionHistoryList } from "./session-history-list";
import { ScoreDots } from "./score-dots";
import { ToolsRail } from "./tools-rail";
import { ReadingLayout } from "@/components/shared/reading-layout";
import { toast } from "sonner";

export default function SessionDetailClient({
  sessionId,
}: {
  sessionId: number;
}) {
  const router = useRouter();
  const isMobile = useIsMobile();
  const [metadata, setMetadata] = useState<SessionMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [toolsOpen, setToolsOpen] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [score, setScore] = useState<number | undefined>(undefined);
  const [scoring, setScoring] = useState(false);
  const [headerCompact, setHeaderCompact] = useState(false);
  const headerSentinelRef = useRef<HTMLDivElement | null>(null);
  const messagesScrollRootRef = useRef<HTMLDivElement | null>(null);
  const messagesSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsSentinelRef = useRef<HTMLDivElement | null>(null);
  const toolsScrollRootRef = useRef<HTMLDivElement | null>(null);

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
      if (rsp.session) {
        setMetadata(rsp.session);
        setScore(rsp.session.score);
      }
    } catch {
      // handled silently
    } finally {
      setLoading(false);
    }
  }, [sessionId]);

  const handleDelete = useCallback(async () => {
    setDeleting(true);
    try {
      await api.deleteSession(sessionId);
      toast.success("Session deleted");
      router.push("/sessions/");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete session");
    } finally {
      setDeleting(false);
      setDeleteConfirmOpen(false);
    }
  }, [sessionId, router]);

  const handleScore = useCallback(
    async (value: number) => {
      if (!sessionId || scoring) return;
      setScoring(true);
      try {
        await api.scoreSession({ sessionId, score: value });
        setScore(value);
        toast.success("Scored");
      } catch {
        toast.error("Failed to score");
      } finally {
        setScoring(false);
      }
    },
    [sessionId, scoring],
  );

  const handleDeleteScore = useCallback(async () => {
    if (!sessionId || scoring) return;
    setScoring(true);
    try {
      await api.deleteScoreSession(sessionId);
      setScore(undefined);
      toast.success("Score removed");
    } catch {
      toast.error("Failed to remove score");
    } finally {
      setScoring(false);
    }
  }, [sessionId, scoring]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    void fetchMetadata();
  }, [fetchMetadata]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const listEnabled =
    !!sessionId && !Number.isNaN(sessionId) && metadata !== null;
  const toolsListEnabled =
    listEnabled &&
    (metadata?.toolCount ?? 0) > 0 &&
    toolsOpen;

  const messagesList = useInfiniteList<MessageItem>({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listSessionMessages(sessionId, page, limit);
        return {
          items: rsp.messages ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [sessionId],
    ),
    pageSize: 50,
    enabled: listEnabled,
  });

  const toolsList = useInfiniteList<ToolItem>({
    fetcher: useCallback(
      async (offset, limit) => {
        const page = Math.floor(offset / limit) + 1;
        const rsp = await api.listSessionTools(sessionId, page, limit);
        return {
          items: rsp.tools ?? [],
          total: Number(rsp.pageInfo?.total ?? 0),
        };
      },
      [sessionId],
    ),
    pageSize: 20,
    enabled: toolsListEnabled,
  });

  useEffect(() => {
    const root = messagesScrollRootRef.current;
    const sentinel = messagesSentinelRef.current;
    if (!sentinel || !messagesList.hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void messagesList.loadMore();
        }
      },
      { root, rootMargin: "200px" },
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
      { root: toolsScrollRootRef.current, rootMargin: "200px" },
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
      <div className="mx-auto w-full max-w-[768px] space-y-5 py-6">
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

  const messageCount = metadata.messageCount;

  const headerContent = (
    <>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => router.push("/sessions/")}
        className="size-10 text-foreground/70 hover:text-foreground"
        aria-label="Back to sessions"
      >
        <ArrowLeft className="size-5" />
      </Button>

      {isMobile && (
        <Sheet>
          <SheetTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                className="size-10 text-foreground/70 hover:text-foreground"
                aria-label="Session history"
              />
            }
          >
            <History className="size-5" />
          </SheetTrigger>
          <SheetContent
            side="bottom"
            showCloseButton={false}
            className="h-[80dvh] max-h-[80dvh] rounded-t-[20px] border-border/70 p-0"
          >
            <div className="flex h-full flex-col">
              <SheetHeader className="border-b border-border/60 px-4 py-3 text-left">
                <SheetTitle>History</SheetTitle>
              </SheetHeader>
              <div className="min-h-0 flex-1">
                <SessionHistoryList
                  activeSessionId={sessionId}
                  onSelect={(id) => router.push(`/sessions/detail?id=${id}`)}
                />
              </div>
            </div>
          </SheetContent>
        </Sheet>
      )}

      <div className="flex min-w-0 flex-1 flex-col items-center px-1 leading-tight">
        <h1
          className={[
            "truncate font-display font-semibold tracking-tight text-foreground",
            "transition-[font-size] duration-200 ease-out",
            isMobile && headerCompact ? "text-[14px]" : "text-[15px]",
          ].filter(Boolean).join(" ")}
        >
          Session #{metadata.id}
        </h1>
        <p
          className={[
            "truncate text-[11px] text-muted-foreground",
            "transition-[max-height,opacity] duration-200 ease-out overflow-hidden",
            isMobile && headerCompact ? "max-h-0 opacity-0" : "max-h-4 opacity-100",
          ].filter(Boolean).join(" ")}
        >
          {messageCount} message{messageCount === 1 ? "" : "s"}
          {metadata.apiKeyName ? ` · ${metadata.apiKeyName}` : ""}
        </p>
      </div>

      <ScoreDots
        score={score}
        scoring={scoring}
        onScore={handleScore}
        onClear={handleDeleteScore}
        size={isMobile ? 20 : 16}
      />

      <Button
        variant={metadata.shareID ? "secondary" : "ghost"}
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

      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => setDeleteConfirmOpen(true)}
        className="size-10 text-foreground/70 hover:text-destructive"
        aria-label="Delete session"
        title="Delete session"
      >
        <Trash2 className="size-5" />
      </Button>

      {metadata.toolCount > 0 && (
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => setToolsOpen((v) => !v)}
          className={[
            "relative size-10",
            toolsOpen
              ? "bg-secondary text-foreground"
              : "text-foreground/70 hover:text-foreground",
          ].join(" ")}
          aria-label="Toggle available tools"
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
    </>
  );

  const readingContent = (
    <>
      {messages.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <MessagesSquare className="mb-3 size-10 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">
            No messages in this session
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
                end of conversation
              </span>
            </div>
          )}
        </div>
      )}
    </>
  );

  const toolsPanelContent = (
    <ToolsRail
      tools={tools}
      hasMore={toolsList.hasMore}
      sentinelRef={toolsSentinelRef}
    />
  );

  return (
    <>
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

      <ShareDialog
        sessionId={metadata.id}
        existingShareID={metadata.shareID}
        open={shareOpen}
        onOpenChange={setShareOpen}
      />

      <AlertDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="size-5 text-destructive" />
              Delete session?
            </AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete{" "}
              <strong>Session #{metadata.id}</strong> and all its messages. This
              action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? "Deleting..." : "Delete"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

    </>
  );
}
