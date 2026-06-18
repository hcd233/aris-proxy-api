"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

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
  const [hover, setHover] = useState<number | null>(null);

  const gap = Math.round(size * 0.5);
  const clearBtn = Math.round(size * 1.25);
  const reserved = clearBtn + gap + 4;
  const dotsWidth = 5 * size + 4 * gap;
  const containerWidth = dotsWidth + reserved;

  const displayScore = hover != null ? hover : score;
  const isRated = score != null;

  return (
    <span
      role="group"
      aria-label={isRated ? `Rated ${score} out of 5` : "Session rating"}
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
            aria-label={`Rate ${v}`}
            className={cn(
              "rounded-full transition-colors duration-150 disabled:opacity-30",
              displayScore != null && v <= displayScore
                ? "bg-primary"
                : "bg-muted-foreground/30 hover:bg-primary",
            )}
            style={{
              width: `${size}px`,
              height: `${size}px`,
              padding: `${Math.round(size * 0.25)}px`,
              backgroundClip: "content-box",
            }}
          />
        ))}
      </span>

      {isRated && (
        <button
          type="button"
          onClick={onClear}
          disabled={scoring}
          aria-label="Remove rating"
          className="absolute rounded text-muted-foreground/40 transition-colors hover:text-destructive disabled:opacity-30"
          style={{
            right: "4px",
            top: "50%",
            transform: "translateY(-50%)",
            width: `${clearBtn}px`,
            height: `${clearBtn}px`,
            fontSize: `${Math.round(size * 0.9)}px`,
            lineHeight: 1,
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          ×
        </button>
      )}
    </span>
  );
}
