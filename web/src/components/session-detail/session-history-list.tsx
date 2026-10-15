"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { api } from "@/lib/api-client";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import type { SessionSummary } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Item, ItemContent, ItemDescription, ItemGroup, ItemTitle } from "@/components/ui/item";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { formatRelativeTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";

const HISTORY_PAGE_SIZE = 20;

export interface SessionHistoryListProps {
  activeSessionId: number;
  onSelect: (sessionId: number) => void;
}

export function SessionHistoryList({ activeSessionId, onSelect }: SessionHistoryListProps) {
  const { t, locale } = useI18n();
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
            placeholder={t("session_detail.search_history")}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="h-9 bg-background pl-9 text-sm"
          />
        </div>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div className="px-2 pb-4">
          {items.length === 0 && !loading && (
            <p className="px-3 py-4 text-center text-sm text-muted-foreground">
              {t("session_detail.no_history")}
            </p>
          )}
          <ItemGroup className="gap-0.5">
            {items.map((session) => (
              <Item
                key={session.id}
                size="sm"
                variant={session.id === activeSessionId ? "muted" : "default"}
                render={<button type="button" onClick={() => onSelect(session.id)} />}
                className="text-left hover:bg-muted"
              >
                <ItemContent className="min-w-0 gap-0.5">
                  <TooltipRoot>
                    <TooltipTrigger
                      render={
                        <ItemTitle className="w-full">
                          {session.summary ||
                            t("session_history.session_label").replace("{id}", String(session.id))}
                        </ItemTitle>
                      }
                    />
                    <TooltipContent side="top" className="max-w-xs break-all">
                      {session.summary ||
                        t("session_history.session_label").replace("{id}", String(session.id))}
                    </TooltipContent>
                  </TooltipRoot>
                  <TooltipRoot>
                    <TooltipTrigger
                      render={
                        <ItemDescription className="line-clamp-1 text-xs">
                          {t("session_history.message_count")
                            .replace("{count}", String(session.messageCount))
                            .replace("{s}", session.messageCount === 1 ? "" : "s")}{" "}
                          · {formatRelativeTime(session.updatedAt, locale)}
                        </ItemDescription>
                      }
                    />
                    <TooltipContent side="top" className="max-w-xs break-all">
                      {t("session_history.message_count")
                        .replace("{count}", String(session.messageCount))
                        .replace("{s}", session.messageCount === 1 ? "" : "s")}{" "}
                      · {formatRelativeTime(session.updatedAt, locale)}
                    </TooltipContent>
                  </TooltipRoot>
                </ItemContent>
              </Item>
            ))}
          </ItemGroup>
          {hasMore && (
            <div ref={sentinelRef} className="flex justify-center py-3">
              <Skeleton className="h-4 w-24" />
            </div>
          )}
        </div>
      </ScrollArea>
    </div>
  );
}
