"use client";

import type { ReactElement, ReactNode } from "react";
import { TooltipContent, TooltipRoot, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

interface TruncatedTextProps {
  children: ReactNode;
  /** 悬停展示的完整内容，默认同 children */
  tooltip?: ReactNode;
  /** 承载截断的元素（如 `<p />`、`<ItemTitle />`），className / children 由 Base UI 合并进去；默认 `<span />` */
  render?: ReactElement;
  className?: string;
  side?: "top" | "right" | "bottom" | "left";
  align?: "start" | "center" | "end";
}

/** 单行截断 + 悬停 Tooltip 看全文（truncate-requires-tooltip 约定的标准写法）。 */
export function TruncatedText({
  children,
  tooltip,
  render = <span />,
  className,
  side,
  align,
}: TruncatedTextProps) {
  return (
    <TooltipRoot>
      <TooltipTrigger render={render} className={cn("truncate", className)}>
        {children}
      </TooltipTrigger>
      <TooltipContent side={side} align={align} className="break-all">
        {tooltip ?? children}
      </TooltipContent>
    </TooltipRoot>
  );
}
