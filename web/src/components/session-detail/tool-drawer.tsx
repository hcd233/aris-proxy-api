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
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";

export interface ToolDrawerProps {
  children: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  toolCount?: number;
}

export function ToolDrawer({
  children,
  open,
  onOpenChange,
  toolCount,
}: ToolDrawerProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Available tools"
          title="Available tools"
          className="relative"
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
        </Button>
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
          <ScrollArea className="flex-1 p-4">{children}</ScrollArea>
        </div>
      </SheetContent>
    </Sheet>
  );
}
