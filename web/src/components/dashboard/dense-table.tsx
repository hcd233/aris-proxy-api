"use client";

import { TooltipContent, TooltipRoot, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

/** 表格主名列：超长截断 + 悬停看全名（truncate-requires-tooltip 约定） */
export function TruncatedName({ text, className }: { text: string; className?: string }) {
  return (
    <TooltipRoot>
      <TooltipTrigger
        render={<span className={cn("block min-w-0 truncate", className)}>{text}</span>}
      />
      <TooltipContent className="max-w-xs break-all">{text}</TooltipContent>
    </TooltipRoot>
  );
}

/** 紧凑表格的通用 class（行高 28px、11px 字号、数字等宽） */
export const DENSE_HEAD =
  "h-7 px-1.5 text-[10px] font-medium tracking-wide text-muted-foreground uppercase";
export const DENSE_CELL = "h-7 px-1.5 py-0 text-[11px] tabular-nums";
