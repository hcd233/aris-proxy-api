"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";

interface ScoreDotInputProps {
  onPick: (value: number) => void;
  disabled?: boolean;
  size?: number;
}

export function ScoreDotInput({ onPick, disabled, size = 8 }: ScoreDotInputProps) {
  const [hover, setHover] = useState<number | null>(null);
  return (
    <div
      className="flex items-center gap-0.5"
      onMouseLeave={() => setHover(null)}
    >
      {[1, 2, 3, 4, 5].map((v) => (
        <button
          key={v}
          type="button"
          disabled={disabled}
          onClick={(e) => {
            e.stopPropagation();
            onPick(v);
          }}
          onMouseEnter={() => setHover(v)}
          className={cn(
            "rounded-full transition-colors disabled:opacity-30",
            hover != null && v <= hover
              ? "bg-primary"
              : "bg-muted-foreground/30 hover:bg-primary",
          )}
          style={{ width: `${size}px`, height: `${size}px` }}
          aria-label={`Rate ${v}`}
        />
      ))}
    </div>
  );
}
