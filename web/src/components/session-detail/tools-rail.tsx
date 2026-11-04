"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, ChevronRight, Search, Wrench, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { useInfiniteList } from "@/hooks/use-infinite-list";
import type { ToolItem } from "@/lib/types";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { ToolDetail } from "./tool-detail";

export type FetchToolsPage = (
  page: number,
  pageSize: number,
  query: string,
) => Promise<{ items: ToolItem[]; total: number }>;

export interface ToolsRailProps {
  /** 按页拉取工具；query 非空时由后端按工具名过滤 */
  fetchPage: FetchToolsPage;
  /** 抽屉打开后才开始加载 */
  enabled: boolean;
}

const PAGE_SIZE = 20;
const SEARCH_DEBOUNCE_MS = 250;
const BOTTOM_SAFE = "pb-[calc(env(safe-area-inset-bottom)+0.75rem)]";

/**
 * 工具抽屉（会话详情与分享页共用）：列表只列工具名，点击进入详情页；
 * 搜索交给后端，输入停顿后从第一页重新拉取。
 * 详情页叠在列表之上，列表保持挂载（visibility 隐藏），返回时滚动位置不丢。
 */
export function ToolsRail({ fetchPage, enabled }: ToolsRailProps) {
  const t = useT();
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [selected, setSelected] = useState<ToolItem | null>(null);
  const [viewport, setViewport] = useState<HTMLDivElement | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  const fetcher = useCallback(
    (offset: number, limit: number) =>
      fetchPage(Math.floor(offset / limit) + 1, limit, debouncedQuery),
    [fetchPage, debouncedQuery],
  );
  const { items, loading, hasMore, loadMore, reset } = useInfiniteList<ToolItem>({
    fetcher,
    pageSize: PAGE_SIZE,
    enabled,
  });

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [query]);

  // 只在查询真正变化时重拉；挂载时不 reset，避免首屏请求被作废重发
  const appliedQueryRef = useRef(debouncedQuery);
  useEffect(() => {
    if (appliedQueryRef.current === debouncedQuery) return;
    appliedQueryRef.current = debouncedQuery;
    reset();
  }, [debouncedQuery, reset]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || !hasMore) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) void loadMore();
      },
      { root: viewport, rootMargin: "200px" },
    );
    io.observe(sentinel);
    return () => io.disconnect();
  }, [viewport, hasMore, loadMore]);

  const searching = debouncedQuery !== "";

  return (
    <div className="relative flex min-h-0 flex-1 flex-col">
      <div
        className={cn("flex min-h-0 flex-1 flex-col", selected && "invisible")}
        inert={!!selected}
      >
        <div className="shrink-0 px-3 pt-3 pb-2">
          <InputGroup>
            <InputGroupAddon>
              <Search />
            </InputGroupAddon>
            <InputGroupInput
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              maxLength={100}
              placeholder={t("tool_sidebar.search_placeholder")}
              aria-label={t("tool_sidebar.search_placeholder")}
            />
            {query && (
              <InputGroupAddon align="inline-end">
                <InputGroupButton
                  size="icon-xs"
                  onClick={() => setQuery("")}
                  aria-label={t("common.clear")}
                >
                  <X />
                </InputGroupButton>
              </InputGroupAddon>
            )}
          </InputGroup>
        </div>
        <ScrollArea className="min-h-0 flex-1" viewportRef={setViewport}>
          <div className={cn("px-1.5", BOTTOM_SAFE)}>
            <ul className="divide-y divide-border/40">
              {items.map((item) => (
                <li key={item.id}>
                  <Button
                    variant="ghost"
                    onClick={() => setSelected(item)}
                    className="group/tool h-auto w-full justify-start gap-2 px-2 py-2 text-left font-normal whitespace-normal active:not-aria-[haspopup]:scale-100"
                  >
                    <Wrench className="size-3.5 text-muted-foreground" />
                    <span className="min-w-0 flex-1 font-mono text-[13px] break-all text-foreground">
                      {item.tool.name || "tool"}
                    </span>
                    <ChevronRight className="size-3.5 text-muted-foreground/50 transition-colors group-hover/tool:text-muted-foreground" />
                  </Button>
                </li>
              ))}
            </ul>
            {searching && items.length === 0 && !hasMore && !loading && (
              <p className="px-2 py-6 text-center text-[12px] text-muted-foreground">
                {t("tool_sidebar.no_match")}
              </p>
            )}
            {hasMore && (
              <div ref={sentinelRef} className="flex justify-center py-3">
                <Skeleton className="h-4 w-24" />
              </div>
            )}
          </div>
        </ScrollArea>
      </div>

      {selected && (
        <div className="absolute inset-0 flex flex-col">
          <div className="flex shrink-0 items-start gap-1.5 border-b border-border/60 px-2 py-2">
            <Button
              variant="ghost"
              size="icon-sm"
              className="shrink-0"
              onClick={() => setSelected(null)}
              aria-label={t("tool_sidebar.back_to_list")}
            >
              <ArrowLeft />
            </Button>
            <h3 className="min-w-0 flex-1 py-1.5 font-mono text-[13.5px] font-medium break-all text-foreground">
              {selected.tool.name || "tool"}
            </h3>
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <div className={cn("px-4 pt-4", BOTTOM_SAFE)}>
              <ToolDetail tool={selected.tool} />
            </div>
          </ScrollArea>
        </div>
      )}
    </div>
  );
}
