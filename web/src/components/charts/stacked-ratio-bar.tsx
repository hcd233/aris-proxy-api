"use client";

import { useState } from "react";
import {
  TooltipContent,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn, formatPct } from "@/lib/utils";

export interface RatioSegment {
  key: string;
  label: string;
  value: number;
  color: string;
}

/**
 * StackedRatioBar 多段占比条：各段宽度 = 段值 / 段值总和。
 * 悬停某段时聚焦该段（其余段淡出），并在该段上方弹出 Tooltip：列出全部段的具体数值与占比，
 * 聚焦段高亮。Tooltip 经 Portal 挂到 body，不受卡片祖先 backdrop-filter / overflow-hidden 影响。
 */
export function StackedRatioBar({
  segments,
  formatValue,
}: {
  segments: RatioSegment[];
  formatValue: (v: number) => string;
}) {
  const [focusedKey, setFocusedKey] = useState<string | null>(null);

  const sum = segments.reduce((acc, s) => acc + Math.max(s.value, 0), 0);
  const visible = sum > 0 ? segments.filter((s) => s.value > 0) : [];

  return (
    <TooltipProvider>
      <div className="py-1" onMouseLeave={() => setFocusedKey(null)}>
        <div className="ratio-track flex h-3 gap-px overflow-hidden rounded-md bg-muted">
          {visible.map((s) => (
            <TooltipRoot key={s.key}>
              <TooltipTrigger
                render={
                  <div
                    // 段宽与段色由数据驱动，只能内联
                    style={{ width: `${(s.value / sum) * 100}%`, backgroundColor: s.color }}
                    className={cn(
                      "cursor-default transition-opacity duration-150",
                      focusedKey !== null && focusedKey !== s.key && "opacity-30",
                    )}
                    onMouseEnter={() => setFocusedKey(s.key)}
                  />
                }
              />
              <TooltipContent className="grid min-w-52 gap-0.5 px-2 py-1.5">
                {segments.map((seg) => (
                  <div
                    key={seg.key}
                    className={cn(
                      "flex items-center gap-2 rounded px-1.5 py-1",
                      seg.key === s.key && "bg-muted font-medium",
                    )}
                  >
                    <div
                      className="size-2.5 shrink-0 rounded-[2px]"
                      style={{ backgroundColor: seg.color }}
                    />
                    <span
                      className={cn(
                        "flex-1",
                        seg.key === s.key ? "text-foreground" : "text-muted-foreground",
                      )}
                    >
                      {seg.label}
                    </span>
                    <span className="font-mono text-foreground tabular-nums">
                      {formatValue(seg.value)}
                    </span>
                    <span className="w-12 text-right font-mono text-muted-foreground tabular-nums">
                      {formatPct(seg.value, sum)}
                    </span>
                  </div>
                ))}
              </TooltipContent>
            </TooltipRoot>
          ))}
        </div>
      </div>
    </TooltipProvider>
  );
}

/** RatioLegend 表头图例：色块 + 段名。 */
export function RatioLegend({
  segments,
}: {
  segments: Pick<RatioSegment, "key" | "label" | "color">[];
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
      {segments.map((s) => (
        <span key={s.key} className="inline-flex items-center gap-1">
          <span className="size-2 shrink-0 rounded-[2px]" style={{ backgroundColor: s.color }} />
          {s.label}
        </span>
      ))}
    </div>
  );
}
