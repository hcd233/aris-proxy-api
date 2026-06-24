"use client";

import { type ReactNode, type Ref, type UIEvent } from "react";
import { Wrench } from "lucide-react";
import { Sheet, SheetContent } from "@/components/ui/sheet";
import { Badge } from "@/components/ui/badge";
import { SwipeDismissSheetBody } from "@/components/session-detail/swipe-dismiss-sheet-body";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";

export interface ReadingLayoutProps {
  header: ReactNode;
  children: ReactNode;
  toolsPanel: ReactNode;
  toolsOpen: boolean;
  onToolsOpenChange: (open: boolean) => void;
  toolsCount: number;
  headerCompact: boolean;
  headerSentinelRef?: Ref<HTMLDivElement>;
  messagesScrollRootRef?: Ref<HTMLDivElement>;
  onMessagesScroll?: (e: UIEvent<HTMLDivElement>) => void;
  onToolsScroll?: (e: UIEvent<HTMLDivElement>) => void;
  onToolsScrollRootChange?: (node: HTMLDivElement | null) => void;
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
  onMessagesScroll,
  onToolsScroll,
  onToolsScrollRootChange,
}: ReadingLayoutProps) {
  const t = useT();
  const isMobile = useIsMobile();

  if (isMobile) {
    return (
      <div className="-mx-4 -mt-4 flex min-h-[calc(100dvh-3.5rem)] flex-col bg-background pb-[calc(env(safe-area-inset-bottom)+1rem)]">
        <div ref={headerSentinelRef} aria-hidden className="h-px w-full" />

        <header
          className={cn(
            "sticky top-[-1rem] z-30 -mt-px",
            "transition-[border-color,background-color,box-shadow] duration-200 ease-out",
            "supports-[backdrop-filter]:backdrop-blur",
            headerCompact
              ? "border-b border-border bg-background/92 supports-[backdrop-filter]:bg-background/75 shadow-[0_1px_0_rgba(0,0,0,0.04)]"
              : "border-b border-border/60 bg-background/85 supports-[backdrop-filter]:bg-background/70",
          )}
        >
          <div
            className={cn(
              "flex items-center gap-1 px-2 pt-[calc(1rem+0.25rem)]",
              "transition-[padding] duration-200 ease-out",
              headerCompact ? "pb-1.5" : "pb-2",
            )}
          >
            {header}
          </div>
        </header>

        <div
          ref={messagesScrollRootRef}
          onScroll={onMessagesScroll}
          className={cn(
            "flex-1 overflow-y-auto px-4 pt-5 pb-[calc(env(safe-area-inset-bottom)+2.5rem)]",
            "[-webkit-overflow-scrolling:touch] overscroll-contain",
          )}
        >
          {children}
        </div>

        {toolsCount > 0 && (
          <Sheet open={toolsOpen} onOpenChange={onToolsOpenChange}>
            <SheetContent
              side="bottom"
              showCloseButton={false}
              className={cn(
                "!h-[88dvh] max-h-[88dvh] rounded-t-[20px] border-border/70 p-0",
                "shadow-[0_-8px_32px_rgba(0,0,0,0.16)]",
                "flex flex-col",
                "!duration-[320ms] !ease-[cubic-bezier(0.32,0.72,0,1)]",
                "data-[side=bottom]:data-starting-style:!translate-y-[100%]",
                "data-[side=bottom]:data-ending-style:!translate-y-[100%]",
              )}
            >
              <SwipeDismissSheetBody
                onDismiss={() => onToolsOpenChange(false)}
                title={t("reading_layout.available_tools")}
                count={toolsCount}
                onScroll={onToolsScroll}
                onScrollRootChange={onToolsScrollRootChange}
              >
                {toolsPanel}
              </SwipeDismissSheetBody>
            </SheetContent>
          </Sheet>
        )}
      </div>
    );
  }

  return (
    <div className="-mx-4 -mt-4 -mb-4 flex h-[100dvh] overflow-hidden bg-background md:-mx-8 md:-mt-8 md:-mb-8 lg:-mx-10 lg:-mt-10 lg:-mb-10">
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="relative z-30 shrink-0 border-b border-border/70 bg-background/95 supports-[backdrop-filter]:backdrop-blur">
          <div className="mx-auto flex max-w-[768px] items-center gap-3 px-4 pt-[calc(1rem+0.25rem)] pb-3 md:pt-[calc(2rem+0.25rem)] lg:pt-[calc(2.5rem+0.25rem)]">
            {header}
          </div>
        </header>

        <div
          ref={messagesScrollRootRef}
          onScroll={onMessagesScroll}
          className="mx-auto w-full max-w-[768px] min-w-0 flex-1 overflow-y-auto overflow-x-hidden px-4 py-6 sm:px-6"
        >
          {children}
        </div>
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
                <Badge variant="secondary" className="text-[10px]">
                  {toolsCount}
                </Badge>
              </div>
              <button
                type="button"
                onClick={() => onToolsOpenChange(false)}
                className="text-[14px] text-muted-foreground transition-colors hover:text-foreground"
                aria-label={t("reading_layout.close_tools")}
              >
                ✕
              </button>
            </div>
            <div
              ref={(node) => onToolsScrollRootChange?.(node)}
              onScroll={onToolsScroll}
              className="min-h-0 flex-1 space-y-2 overflow-y-auto p-4"
            >
              {toolsPanel}
            </div>
          </div>
        </aside>
      )}
    </div>
  );
}
