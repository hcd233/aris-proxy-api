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
  "h-7 px-1 text-[11px] font-medium tracking-wide text-muted-foreground uppercase";
export const DENSE_CELL = "h-7 px-1 py-0 text-[11px] tabular-nums";

/**
 * 面板主体声明为 container（见两张表的 bodyClassName）后，列按面板宽度渐进显示：
 * 窄面板只留主名列 + 关键指标，宽面板再逐列展开。
 *
 * 为什么不是「所有列常驻 + 主名列吃剩余宽度」：1280 视口下面板表格只有约 310px，
 * 六列的声明宽度就占了 280px，主名列被挤到 17px（Top 排行只剩 5px），名字完全看不见。
 * 阈值按「主名列至少 ~130px（够显示 claude-code-oauth 这类名字）」反推，数值是表格可用宽度。
 */
export const SHOW_FROM_310 = "hidden @min-[310px]:table-cell";
export const SHOW_FROM_360 = "hidden @min-[360px]:table-cell";
export const SHOW_FROM_440 = "hidden @min-[440px]:table-cell";
export const SHOW_FROM_520 = "hidden @min-[520px]:table-cell";
