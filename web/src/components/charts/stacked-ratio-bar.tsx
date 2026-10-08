"use client";

import { useLayoutEffect, useRef, useState } from "react";

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
 * 悬停展示全部段的具体数值与占比，悬停到某段时聚焦该段（其余段淡出、提示中对应行高亮）。
 */
export function StackedRatioBar({
  segments,
  formatValue,
}: {
  segments: RatioSegment[];
  formatValue: (v: number) => string;
}) {
  const [hovered, setHovered] = useState(false);
  const [focusedKey, setFocusedKey] = useState<string | null>(null);
  const [tooltipPos, setTooltipPos] = useState({ left: 0, top: 0 });
  const barRef = useRef<HTMLDivElement>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);

  const sum = segments.reduce((acc, s) => acc + Math.max(s.value, 0), 0);

  useLayoutEffect(() => {
    if (!hovered || !barRef.current || !tooltipRef.current) return;
    const barRect = barRef.current.getBoundingClientRect();
    const ttRect = tooltipRef.current.getBoundingClientRect();
    const left = barRect.left + barRect.width / 2 - ttRect.width / 2;
    const top = barRect.top - ttRect.height - 8;
    const clampedLeft = Math.max(8, Math.min(left, window.innerWidth - ttRect.width - 8));
    setTooltipPos({ left: clampedLeft, top });
  }, [hovered]);

  return (
    <div
      ref={barRef}
      className="relative py-1"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => {
        setHovered(false);
        setFocusedKey(null);
      }}
    >
      <div className="flex h-3 gap-px overflow-hidden rounded-md bg-muted">
        {sum > 0 &&
          segments
            .filter((s) => s.value > 0)
            .map((s) => (
              <div
                key={s.key}
                style={{ width: `${(s.value / sum) * 100}%`, backgroundColor: s.color }}
                className={`transition-opacity duration-200 ${
                  focusedKey !== null && focusedKey !== s.key ? "opacity-35" : ""
                }`}
                onMouseEnter={() => setFocusedKey(s.key)}
              />
            ))}
      </div>

      {hovered && (
        <div
          ref={tooltipRef}
          className="pointer-events-none fixed z-50"
          style={{ left: tooltipPos.left, top: tooltipPos.top }}
        >
          <div className="grid min-w-52 items-start gap-1 rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl">
            {segments.map((s) => (
              <div
                key={s.key}
                className={`flex items-center gap-2 rounded px-1 py-0.5 ${
                  focusedKey === s.key ? "bg-muted" : ""
                }`}
              >
                <div
                  className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                  style={{ backgroundColor: s.color }}
                />
                <span className="flex-1 text-muted-foreground">{s.label}</span>
                <span className="font-mono font-medium text-foreground tabular-nums">
                  {formatValue(s.value)}
                </span>
                <span className="w-12 text-right font-mono text-muted-foreground tabular-nums">
                  {formatPct(s.value, sum)}
                </span>
              </div>
            ))}
          </div>
        </div>
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
