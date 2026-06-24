"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";

interface ScoreDotsProps {
  score: number | undefined;
  scoring: boolean;
  onScore: (value: number) => void;
  onClear: () => void;
  size?: number;
}

export function ScoreDots({
  score,
  scoring,
  onScore,
  onClear,
  size = 16,
}: ScoreDotsProps) {
  const t = useT();
  const [hover, setHover] = useState<number | null>(null);

  const dotSize = Math.round(size * 0.625);
  const gap = Math.round(size * 0.25);
  const dotsWidth = 5 * dotSize + 4 * gap;
  const clearReserved = size + gap;
  const containerWidth = dotsWidth + clearReserved;

  const displayScore = hover != null ? hover : score;
  const isRated = score != null;

  return (
    <span
      role="group"
      aria-label={isRated ? t("score_dots.rated").replace("{score}", String(score)) : t("score_dots.rating")}
      className="relative inline-flex items-center"
      style={{ width: `${containerWidth}px` }}
      onMouseLeave={() => setHover(null)}
    >
      <span className="inline-flex items-center" style={{ gap: `${gap}px` }}>
        {[1, 2, 3, 4, 5].map((v) => (
          <button
            key={v}
            type="button"
            disabled={scoring}
            onClick={(e) => {
              e.stopPropagation();
              onScore(v);
            }}
            onMouseEnter={() => setHover(v)}
            aria-label={t("score_dots.rate").replace("{value}", String(v))}
            className={cn(
              "relative rounded-full transition-all duration-150 disabled:opacity-30 active:scale-90",
              "before:absolute before:-inset-1 before:content-['']",
              displayScore != null && v <= displayScore
                ? "bg-primary"
                : "bg-muted-foreground/30 hover:bg-primary/70",
            )}
            style={{
              width: `${dotSize}px`,
              height: `${dotSize}px`,
            }}
          />
        ))}
      </span>

      {isRated && (
        <button
          type="button"
          onClick={onClear}
          disabled={scoring}
          aria-label={t("score_dots.remove")}
          className={cn(
            "absolute right-0 top-1/2 inline-flex -translate-y-1/2 items-center justify-center rounded text-muted-foreground/50 transition-colors hover:text-destructive disabled:opacity-30",
            "before:absolute before:-inset-1 before:content-['']",
          )}
          style={{
            width: `${size}px`,
            height: `${size}px`,
            fontSize: `${Math.round(size * 0.9)}px`,
            lineHeight: 1,
          }}
        >
          ×
        </button>
      )}
    </span>
  );
}
