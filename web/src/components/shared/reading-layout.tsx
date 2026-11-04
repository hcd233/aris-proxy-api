"use client";

import { type ReactNode, type Ref } from "react";
import { ArrowDown, Wrench, X } from "lucide-react";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  MessageScroller,
  MessageScrollerButton,
  MessageScrollerContent,
  MessageScrollerProvider,
  MessageScrollerViewport,
} from "@/components/ui/message-scroller";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";

export interface ReadingLayoutProps {
  header: ReactNode;
  /** 转录条目（MessageScrollerItem 序列），渲染在 MessageScrollerContent 内 */
  children: ReactNode;
  toolsPanel: ReactNode;
  toolsOpen: boolean;
  onToolsOpenChange: (open: boolean) => void;
  toolsCount: number;
  headerCompact: boolean;
  /** 移动端标题收缩哨兵：位于滚动内容顶部，滚出 viewport 即收缩 */
  headerSentinelRef?: Ref<HTMLDivElement>;
  /** 指向 MessageScrollerViewport（无限加载 / 标题收缩的 IntersectionObserver root） */
  messagesScrollRootRef?: Ref<HTMLDivElement>;
  /** 独立页面（分享页）：不在控制台内容区内，不抵消其 padding，高度占满视口 */
  standalone?: boolean;
}

/** 阅读区：MessageScroller 从头阅读（不自动跟随），提供跳到开头 / 结尾 */
function TranscriptScroller({
  children,
  contentClassName,
  viewportClassName,
  sentinel,
  scrollRootRef,
}: {
  children: ReactNode;
  contentClassName: string;
  viewportClassName?: string;
  sentinel?: ReactNode;
  scrollRootRef?: Ref<HTMLDivElement>;
}) {
  const t = useT();
  return (
    <MessageScrollerProvider autoScroll={false} defaultScrollPosition="start">
      <MessageScroller className="h-auto min-h-0 flex-1">
        <MessageScrollerViewport
          ref={scrollRootRef}
          className={cn("[-webkit-overflow-scrolling:touch]", viewportClassName)}
        >
          <MessageScrollerContent className={contentClassName}>
            {sentinel}
            {children}
          </MessageScrollerContent>
        </MessageScrollerViewport>
        {/* 瞬时跳转：Item 的 content-visibility:auto 按估高占位，平滑滚动途中内容增高会停在旧终点 */}
        <MessageScrollerButton direction="start" behavior="instant">
          <ArrowDown />
          <span className="sr-only">{t("reading_layout.jump_start")}</span>
        </MessageScrollerButton>
        <MessageScrollerButton direction="end" behavior="instant">
          <ArrowDown />
          <span className="sr-only">{t("reading_layout.jump_end")}</span>
        </MessageScrollerButton>
      </MessageScroller>
    </MessageScrollerProvider>
  );
}

export function ReadingLayout({
  header,
  children,
  toolsPanel,
  toolsOpen,
  onToolsOpenChange,
  toolsCount,
  headerCompact,
  headerSentinelRef,
  messagesScrollRootRef,
  standalone = false,
}: ReadingLayoutProps) {
  const t = useT();
  const isMobile = useIsMobile();

  if (isMobile) {
    return (
      <div
        className={cn(
          "page-surface flex flex-col overflow-hidden bg-background",
          standalone ? "h-[100dvh]" : "-mx-4 -mt-4 -mb-4 h-[calc(100dvh-3.5rem)]",
        )}
      >
        <header
          className={cn(
            "relative z-30 shrink-0",
            "transition-[border-color,background-color,box-shadow] duration-200 ease-out",
            headerCompact
              ? "border-b border-border bg-background shadow-[0_1px_0_rgba(0,0,0,0.04)]"
              : "border-b border-border/60 bg-background",
          )}
        >
          <div
            className={cn(
              "flex flex-wrap items-center gap-1 gap-y-1 px-2 pt-2",
              "transition-[padding] duration-200 ease-out",
              headerCompact ? "pb-1.5" : "pb-2",
            )}
          >
            {header}
          </div>
        </header>

        <TranscriptScroller
          scrollRootRef={messagesScrollRootRef}
          sentinel={<div ref={headerSentinelRef} aria-hidden className="h-px w-full shrink-0" />}
          contentClassName="gap-5 px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+2.5rem)]"
        >
          {children}
        </TranscriptScroller>

        {toolsCount > 0 && (
          <Drawer open={toolsOpen} onOpenChange={onToolsOpenChange} showSwipeHandle>
            <DrawerContent>
              <DrawerHeader className="flex-row items-center gap-2 pb-3">
                <Wrench className="size-4 text-muted-foreground" />
                <DrawerTitle>{t("reading_layout.available_tools")}</DrawerTitle>
                <Badge variant="secondary">{toolsCount}</Badge>
                <DrawerClose render={<Button variant="ghost" size="sm" className="ml-auto" />}>
                  {t("common.done")}
                </DrawerClose>
              </DrawerHeader>
              <div className="flex min-h-0 flex-1 flex-col">{toolsPanel}</div>
            </DrawerContent>
          </Drawer>
        )}
      </div>
    );
  }

  return (
    <div
      className={cn(
        "page-surface flex h-[100dvh] overflow-hidden bg-background",
        !standalone && "-mx-4 -mt-4 -mb-4 md:-mx-8 md:-mt-8 md:-mb-8 lg:-mx-10 lg:-mt-10 lg:-mb-10",
      )}
    >
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="relative z-30 shrink-0 border-b border-border/70 bg-background/95 supports-[backdrop-filter]:backdrop-blur">
          <div
            className={cn(
              "mx-auto flex max-w-[768px] items-center gap-3 px-4 pb-3",
              standalone
                ? "pt-4"
                : "pt-[calc(1rem+0.25rem)] md:pt-[calc(2rem+0.25rem)] lg:pt-[calc(2.5rem+0.25rem)]",
            )}
          >
            {header}
          </div>
        </header>

        <TranscriptScroller
          scrollRootRef={messagesScrollRootRef}
          // scrollbar-gutter：滚动条出现与否不改变内容列宽（语言切换布局不变约定）
          viewportClassName="overflow-x-hidden [scrollbar-gutter:stable]"
          contentClassName="mx-auto w-full max-w-[768px] min-w-0 gap-5 px-4 py-6 sm:px-6"
        >
          {children}
        </TranscriptScroller>
      </div>

      {toolsCount > 0 && (
        <aside
          className={cn(
            "h-full shrink-0 overflow-hidden border-l border-border/70 bg-card transition-[width] duration-200 ease-out",
            toolsOpen ? "w-[280px]" : "w-0",
          )}
        >
          <div
            className="flex h-full w-[280px] flex-col"
            style={{ visibility: toolsOpen ? "visible" : "hidden" }}
          >
            <div className="flex items-center justify-between border-b border-border/60 px-4 py-3">
              <div className="flex items-center gap-2">
                <Wrench className="size-4 text-muted-foreground" />
                <span className="font-display text-[14px] font-semibold text-foreground">
                  {t("reading_layout.available_tools")}
                </span>
                <Badge variant="secondary" className="text-[11px]">
                  {toolsCount}
                </Badge>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                onClick={() => onToolsOpenChange(false)}
                aria-label={t("reading_layout.close_tools")}
              >
                <X />
              </Button>
            </div>
            <div className="flex min-h-0 flex-1 flex-col">{toolsPanel}</div>
          </div>
        </aside>
      )}
    </div>
  );
}
