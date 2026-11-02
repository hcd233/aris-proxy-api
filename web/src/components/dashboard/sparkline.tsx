"use client";

import { useId } from "react";
import { cn } from "@/lib/utils";

/** 折线点 → path：用 M/L 而非 polyline 的 points，才能经 CSS `d` 做形变过渡 */
function linePath(points: string[]): string {
  return points.map((p, i) => `${i === 0 ? "M" : "L"}${p}`).join(" ");
}

/** `d` 同时写属性（兜底：不支持 CSS `d` 的浏览器直接跳变）与样式（支持时平滑形变） */
function morphProps(d: string) {
  return { d, style: { d: `path("${d}")` } as React.CSSProperties, className: "data-morph" };
}

/**
 * 无坐标轴迷你趋势线：本期实线 + 上一周期淡虚线，同一纵向比例尺（两者可比）。
 * 颜色取 currentColor，由调用方用 text-* 主题类决定。刷新时点数不变则平滑形变。
 */
export function Sparkline({
  values,
  previous,
  className,
}: {
  values: number[];
  previous?: number[];
  className?: string;
}) {
  const gradientId = useId();
  const all = [...values, ...(previous ?? [])].filter(Number.isFinite);
  if (values.length < 2 || all.length === 0) {
    return <div className={cn("h-6", className)} />;
  }
  const max = Math.max(...all);
  const min = Math.min(...all);
  const span = max - min || 1;
  const W = 100;
  const H = 24;
  const toPoints = (arr: number[]) =>
    arr.map((v, i) => {
      const x = (i / Math.max(arr.length - 1, 1)) * W;
      const y = H - 2 - ((Number.isFinite(v) ? v - min : 0) / span) * (H - 4);
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    });
  const cur = toPoints(values);
  const curLine = linePath(cur);
  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      preserveAspectRatio="none"
      className={cn("h-6 w-full overflow-visible", className)}
      aria-hidden
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="currentColor" stopOpacity="0.22" />
          <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
        </linearGradient>
      </defs>
      {previous && previous.length > 1 && (
        <path
          {...morphProps(linePath(toPoints(previous)))}
          fill="none"
          stroke="currentColor"
          strokeOpacity="0.35"
          strokeWidth="1"
          strokeDasharray="2 2"
          vectorEffect="non-scaling-stroke"
        />
      )}
      <path
        {...morphProps(`M0,${H} L${cur.join(" L")} L${W},${H} Z`)}
        fill={`url(#${gradientId})`}
      />
      <path
        {...morphProps(curLine)}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
