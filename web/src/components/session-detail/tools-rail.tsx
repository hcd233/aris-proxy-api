"use client";

import { useEffect, useMemo, useState, type Ref } from "react";
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
import type { ToolItem } from "@/lib/types";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { filterToolsByName } from "./tool-filter";
import { ToolDetail } from "./tool-detail";

export interface ToolsRailProps {
  tools: ToolItem[];
  hasMore: boolean;
  loading: boolean;
  loadMore: () => Promise<void>;
  /** 无限加载哨兵（IntersectionObserver 的 root 是 scrollRootRef 指向的列表 viewport） */
  sentinelRef?: Ref<HTMLDivElement>;
  scrollRootRef?: (node: HTMLDivElement | null) => void;
}

const BOTTOM_SAFE = "pb-[calc(env(safe-area-inset-bottom)+0.75rem)]";

/**
 * 工具抽屉（会话详情与分享页共用）：列表只列工具名，点击进入详情页。
 * 详情页叠在列表之上，列表保持挂载（visibility 隐藏），返回时滚动位置不丢。
 */
export function ToolsRail({
  tools,
  hasMore,
  loading,
  loadMore,
  sentinelRef,
  scrollRootRef,
}: ToolsRailProps) {
  const t = useT();
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<number | null>(null);

  const searching = query.trim() !== "";
  const filtered = useMemo(() => filterToolsByName(tools, query), [tools, query]);
  const selected = selectedId === null ? undefined : tools.find((x) => x.id === selectedId);

  // 搜索要覆盖全部工具：有查询时把剩余分页一次拉完（单会话工具数量级为百）
  useEffect(() => {
    if (searching && hasMore && !loading) void loadMore();
  }, [searching, hasMore, loading, loadMore]);

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
        <ScrollArea className="min-h-0 flex-1" viewportRef={scrollRootRef}>
          <div className={cn("px-1.5", BOTTOM_SAFE)}>
            <ul className="divide-y divide-border/40">
              {filtered.map((item) => (
                <li key={item.id}>
                  <Button
                    variant="ghost"
                    onClick={() => setSelectedId(item.id)}
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
            {searching && filtered.length === 0 && !hasMore && (
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
              onClick={() => setSelectedId(null)}
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
