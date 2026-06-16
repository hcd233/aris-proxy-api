"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { api } from "@/lib/api-client";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import type { SessionSummary } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn, formatRelativeTime } from "@/lib/utils";

const HISTORY_PAGE_SIZE = 20;

export interface SessionHistoryListProps {
  activeSessionId: number;
  onSelect: (sessionId: number) => void;
}

export function SessionHistoryList({
  activeSessionId,
  onSelect,
}: SessionHistoryListProps) {
  const [keyword, setKeyword] = useState("");
  const [debouncedKeyword, setDebouncedKeyword] = useState("");
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  const fetcher = useCallback(
    async (offset: number, limit: number) => {
      const page = Math.floor(offset / limit) + 1;
      const rsp = await api.listSessions({
        page,
        pageSize: limit,
        keyword: debouncedKeyword || undefined,
        sortField: "updated_at",
        sort: "desc",
      });
      const items = rsp.sessions ?? [];
      return { items, total: rsp.pageInfo?.total ?? 0 };
    },
    [debouncedKeyword],
  );

  const { items, loading, hasMore, loadMore, reset } = useInfiniteList<SessionSummary>({
    fetcher,
    pageSize: HISTORY_PAGE_SIZE,
    enabled: true,
  });

  useEffect(() => {
    if (searchDebounceRef.current) {
      clearTimeout(searchDebounceRef.current);
    }
    searchDebounceRef.current = setTimeout(() => {
      setDebouncedKeyword(keyword);
    }, 250);
    return () => {
      if (searchDebounceRef.current) {
        clearTimeout(searchDebounceRef.current);
      }
    };
  }, [keyword]);

  useEffect(() => {
    reset();
  }, [debouncedKeyword, reset]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || !hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          void loadMore();
        }
      },
      { rootMargin: "200px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
  }, [hasMore, loadMore]);

  return (
    <div className="flex h-full flex-col">
      <div className="shrink-0 px-3 pb-3 pt-2">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search history"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="h-9 bg-background pl-9 text-sm"
          />
        </div>
      </div>
      <div className="flex-1 overflow-y-auto">
        <div className="px-2 pb-4">
          {items.length === 0 && !loading && (
            <p className="px-3 py-4 text-center text-sm text-muted-foreground">
              No history found
            </p>
          )}
          <ul className="space-y-0.5">
            {items.map((session) => (
              <li key={session.id}>
                <button
                  type="button"
                  onClick={() => onSelect(session.id)}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-left transition-colors",
                    "hover:bg-accent hover:text-accent-foreground",
                    session.id === activeSessionId &&
                      "bg-accent text-accent-foreground",
                  )}
                >
                  <p className="line-clamp-1 text-sm font-medium">
                    {session.summary || `Session #${session.id}`}
                  </p>
                  <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">
                    {session.messageCount} message{session.messageCount === 1 ? "" : "s"} ·{" "}
                    {formatRelativeTime(session.updatedAt)}
                  </p>
                </button>
              </li>
            ))}
          </ul>
          {hasMore && (
            <div
              ref={sentinelRef}
              className="flex justify-center py-3"
            >
              <Skeleton className="h-4 w-24" />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
