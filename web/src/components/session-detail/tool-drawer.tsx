"use client";

import { Wrench } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface ToolDrawerProps {
  children: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  toolCount?: number;
  onScroll?: (e: React.UIEvent<HTMLDivElement>) => void;
  scrollRootRef?: React.Ref<HTMLDivElement>;
}

export function ToolDrawer({
  children,
  open,
  onOpenChange,
  toolCount,
  onScroll,
  scrollRootRef,
}: ToolDrawerProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Available tools"
            title="Available tools"
            className="relative"
          />
        }
      >
        <Wrench className="size-5" />
        {toolCount ? (
          <span
            className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold tabular-nums text-primary-foreground"
            aria-hidden
          >
            {toolCount}
          </span>
        ) : null}
      </SheetTrigger>
      <SheetContent
        side="right"
        className={cn(
          "w-full sm:w-[420px] sm:max-w-[420px]",
          "border-l border-border/70 p-0",
        )}
      >
        <div className="flex h-full flex-col">
          <SheetHeader className="border-b border-border/60 px-4 py-3 text-left">
            <SheetTitle>Available Tools</SheetTitle>
          </SheetHeader>
          <div
            ref={scrollRootRef}
            onScroll={onScroll}
            className="flex-1 space-y-2 overflow-y-auto p-4"
          >
            {children}
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
