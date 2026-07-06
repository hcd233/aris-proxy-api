"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Star } from "lucide-react";
import { cn } from "@/lib/utils";

interface ScoreSliderProps {
  distribution: Record<number, number>;
  value: number;
  onChange: (score: number) => void;
}

export function ScoreSlider({ distribution, value, onChange }: ScoreSliderProps) {
  const trackRef = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);

  const maxCount = Math.max(...Object.values(distribution), 1);
  const total = Object.values(distribution).reduce((a, b) => a + b, 0);

  const getPositionFromClientX = useCallback(
    (clientX: number) => {
      if (!trackRef.current) return value;
      const rect = trackRef.current.getBoundingClientRect();
      const pct = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
      const score = Math.round(pct * 4) + 1;
      return Math.max(1, Math.min(5, score));
    },
    [value]
  );

  const handlePointerDown = useCallback(
    (e: React.PointerEvent) => {
      setDragging(true);
      const score = getPositionFromClientX(e.clientX);
      onChange(score);
      e.currentTarget.setPointerCapture(e.pointerId);
    },
    [getPositionFromClientX, onChange]
  );

  const handlePointerMove = useCallback(
    (e: React.PointerEvent) => {
      if (!dragging) return;
      const score = getPositionFromClientX(e.clientX);
      onChange(score);
    },
    [dragging, getPositionFromClientX, onChange]
  );

  const handlePointerUp = useCallback(() => {
    setDragging(false);
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "ArrowLeft" || e.key === "ArrowDown") {
        e.preventDefault();
        onChange(Math.max(1, value - 1));
      } else if (e.key === "ArrowRight" || e.key === "ArrowUp") {
        e.preventDefault();
        onChange(Math.min(5, value + 1));
      }
    },
    [value, onChange]
  );

  useEffect(() => {
    const handleGlobalUp = () => setDragging(false);
    document.addEventListener("pointerup", handleGlobalUp);
    return () => document.removeEventListener("pointerup", handleGlobalUp);
  }, []);

  const pct = ((value - 1) / 4) * 100;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
          Minimum Score
        </span>
        <div className="flex items-center gap-1">
          {Array.from({ length: 5 }).map((_, i) => (
            <Star
              key={i}
              className={cn(
                "size-3.5 transition-colors",
                i + 1 >= value
                  ? "fill-amber-400 text-amber-400"
                  : "text-muted-foreground/30"
              )}
            />
          ))}
        </div>
      </div>

      <div
        ref={trackRef}
        role="slider"
        aria-valuemin={1}
        aria-valuemax={5}
        aria-valuenow={value}
        tabIndex={0}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onKeyDown={handleKeyDown}
        className="relative flex h-10 cursor-pointer touch-none items-end gap-[3px] select-none rounded-lg bg-secondary/50 px-1 pb-1 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
      >
        {Array.from({ length: 5 }).map((_, i) => {
          const score = i + 1;
          const count = distribution[score] || 0;
          const barH = total > 0 ? Math.max(2, (count / maxCount) * 100) : 2;
          const isBelow = score >= value;
          return (
            <div
              key={score}
              className="relative flex-1 flex items-end justify-center"
            >
              <div
                className={cn(
                  "w-full rounded-t-sm transition-all duration-300",
                  isBelow
                    ? "bg-amber-400/60"
                    : "bg-muted-foreground/15"
                )}
                style={{ height: `${barH}%` }}
              />
              <span
                className={cn(
                  "absolute -bottom-0.5 text-[9px] font-medium tabular-nums transition-colors",
                  isBelow
                    ? "text-amber-500/80"
                    : "text-muted-foreground/30"
                )}
              >
                {score}
              </span>
            </div>
          );
        })}

        <div
          className="absolute top-0 -translate-x-1/2"
          style={{ left: `${pct}%` }}
        >
          <div className="flex flex-col items-center">
            <div className="size-3 rounded-full border-2 border-amber-400 bg-background shadow-md" />
            <div className="mt-0.5 text-[10px] font-bold tabular-nums text-amber-500">
              ≥{value}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
