"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export interface RatioSegment {
  key: string;
  label: string;
  value: number;
  color: string;
}

function formatPct(value: number, sum: number): string {
  if (sum <= 0) return "0%";
  return `${((value / sum) * 100).toFixed(1)}%`;
}

/**
 * StackedRatioBar 多段占比条：各段宽度 = 段值 / 段值总和。
 * 悬停某段时聚焦该段（其余段淡出），并在该段上方弹出提示：列出全部段的具体数值与占比，聚焦段高亮。
 * 提示经 portal 挂到 body：卡片祖先可能带 backdrop-filter（moonshot 主题），
 * 会让 fixed 定位相对卡片计算并被 overflow-hidden 裁掉。
 */
export function StackedRatioBar({
  segments,
  formatValue,
}: {
  segments: RatioSegment[];
  formatValue: (v: number) => string;
}) {
  const [focusedKey, setFocusedKey] = useState<string | null>(null);
  const [tooltipPos, setTooltipPos] = useState<{ left: number; top: number } | null>(null);
  const segmentRefs = useRef(new Map<string, HTMLDivElement>());
  const tooltipRef = useRef<HTMLDivElement>(null);

  const sum = segments.reduce((acc, s) => acc + Math.max(s.value, 0), 0);
  const visible = sum > 0 ? segments.filter((s) => s.value > 0) : [];

  useLayoutEffect(() => {
    if (focusedKey === null) return;
    const anchor = segmentRefs.current.get(focusedKey);
    if (!anchor || !tooltipRef.current) return;
    const segRect = anchor.getBoundingClientRect();
    const ttRect = tooltipRef.current.getBoundingClientRect();
    const left = segRect.left + segRect.width / 2 - ttRect.width / 2;
    const clampedLeft = Math.max(8, Math.min(left, window.innerWidth - ttRect.width - 8));
    const above = segRect.top - ttRect.height - 8;
    const top = above >= 8 ? above : segRect.bottom + 8;
    setTooltipPos({ left: clampedLeft, top });
  }, [focusedKey]);

  return (
    <div
      className="py-1"
      onMouseLeave={() => {
        setFocusedKey(null);
        setTooltipPos(null);
      }}
    >
      <div className="flex h-3 gap-px overflow-hidden rounded-md bg-muted">
        {visible.map((s) => (
          <div
            key={s.key}
            ref={(el) => {
              if (el) segmentRefs.current.set(s.key, el);
              else segmentRefs.current.delete(s.key);
            }}
            style={{ width: `${(s.value / sum) * 100}%`, backgroundColor: s.color }}
            className={`cursor-default transition-opacity duration-150 ${
              focusedKey !== null && focusedKey !== s.key ? "opacity-30" : ""
            }`}
            onMouseEnter={() => setFocusedKey(s.key)}
          />
        ))}
      </div>

      {focusedKey !== null &&
        createPortal(
          <div
            ref={tooltipRef}
            className="pointer-events-none fixed z-50"
            style={{
              left: tooltipPos?.left ?? 0,
              top: tooltipPos?.top ?? 0,
              visibility: tooltipPos ? "visible" : "hidden",
            }}
          >
            <div className="grid min-w-52 items-start gap-0.5 rounded-lg border border-border/50 bg-popover px-2 py-1.5 text-xs text-popover-foreground shadow-xl">
              {segments.map((s) => (
                <div
                  key={s.key}
                  className={`flex items-center gap-2 rounded px-1.5 py-1 ${
                    focusedKey === s.key ? "bg-muted font-medium" : ""
                  }`}
                >
                  <div
                    className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                    style={{ backgroundColor: s.color }}
                  />
                  <span
                    className={`flex-1 ${focusedKey === s.key ? "text-foreground" : "text-muted-foreground"}`}
                  >
                    {s.label}
                  </span>
                  <span className="font-mono text-foreground tabular-nums">
                    {formatValue(s.value)}
                  </span>
                  <span className="w-12 text-right font-mono text-muted-foreground tabular-nums">
                    {formatPct(s.value, sum)}
                  </span>
                </div>
              ))}
            </div>
          </div>,
          document.body,
        )}
    </div>
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
          <span className="h-2 w-2 shrink-0 rounded-[2px]" style={{ backgroundColor: s.color }} />
          {s.label}
        </span>
      ))}
    </div>
  );
}
