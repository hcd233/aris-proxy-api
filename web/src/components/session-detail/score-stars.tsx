"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

interface ScoreStarsProps {
  score: number | undefined;
  scoring: boolean;
  onScore: (value: number) => void;
  onClear: () => void;
  size?: number;
}

export function ScoreStars({
  score,
  scoring,
  onScore,
  onClear,
  size = 11,
}: ScoreStarsProps) {
  const [confirmValue, setConfirmValue] = useState<number | null>(null);

  if (score != null) {
    return (
      <span className="inline-flex items-center gap-1">
        <span className="inline-flex items-center gap-0.5">
          {[1, 2, 3, 4, 5].map((v) => (
            <span
              key={v}
              className={cn(
                "inline-block rounded-full",
                v <= score ? "bg-primary" : "bg-muted-foreground/30",
              )}
              style={{ width: `${size}px`, height: `${size}px` }}
              aria-hidden
            />
          ))}
        </span>
        <button
          type="button"
          onClick={onClear}
          disabled={scoring}
          className="ml-0.5 rounded text-muted-foreground/40 transition-colors hover:text-destructive disabled:opacity-30"
          aria-label="Remove score"
        >
          ×
        </button>
      </span>
    );
  }

  if (confirmValue != null) {
    return (
      <div className="inline-flex items-center gap-1 rounded-md border border-border bg-secondary/50 px-2 py-1">
        <span className="text-xs text-muted-foreground">
          Rate {confirmValue}?
        </span>
        <button
          type="button"
          onClick={() => {
            onScore(confirmValue);
            setConfirmValue(null);
          }}
          disabled={scoring}
          className="rounded px-1.5 py-0.5 text-xs font-medium text-foreground transition-colors hover:bg-green-500/10 hover:text-green-600 disabled:opacity-50"
        >
          Yes
        </button>
        <button
          type="button"
          onClick={() => setConfirmValue(null)}
          className="rounded px-1.5 py-0.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          No
        </button>
      </div>
    );
  }

  return (
    <span className="inline-flex items-center gap-0.5">
      {[1, 2, 3, 4, 5].map((v) => (
        <button
          key={v}
          type="button"
          disabled={scoring}
          onClick={() => setConfirmValue(v)}
          className="rounded-full bg-muted-foreground/30 transition-colors hover:bg-primary disabled:opacity-30"
          style={{ width: `${size}px`, height: `${size}px` }}
          aria-label={`Rate ${v}`}
        />
      ))}
    </span>
  );
}
