"use client";

import { useMemo, useState } from "react";
import { diffLines } from "diff";
import { cn } from "@/lib/utils";

interface CompressionDiffProps {
  before: string;
  after: string;
}

type ViewMode = "split" | "inline";

export function CompressionDiff({ before, after }: CompressionDiffProps) {
  const [mode, setMode] = useState<ViewMode>("split");

  const parts = useMemo(() => diffLines(before, after), [before, after]);

  const stats = useMemo(() => {
    let added = 0;
    let removed = 0;
    for (const part of parts) {
      const lineCount = part.value.split("\n").filter((_, i, arr) => i < arr.length - 1 || part.value.endsWith("\n")).length;
      if (part.added) added += lineCount;
      else if (part.removed) removed += lineCount;
    }
    return { added, removed };
  }, [parts]);

  return (
    <div className="mt-2 overflow-hidden rounded-md border border-border">
      <div className="flex items-center justify-between border-b border-border bg-muted/20 px-3 py-1.5">
        <div className="flex items-center gap-3">
          <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
            Diff
          </span>
          <span className="font-mono text-[10px] text-green-600 dark:text-green-400">
            +{stats.added}
          </span>
          <span className="font-mono text-[10px] text-red-600 dark:text-red-400">
            -{stats.removed}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => setMode("split")}
            className={cn(
              "rounded px-2 py-0.5 font-mono text-[10px] transition-colors",
              mode === "split"
                ? "bg-primary/12 text-primary"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            Split
          </button>
          <button
            type="button"
            onClick={() => setMode("inline")}
            className={cn(
              "rounded px-2 py-0.5 font-mono text-[10px] transition-colors",
              mode === "inline"
                ? "bg-primary/12 text-primary"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            Inline
          </button>
        </div>
      </div>

      {mode === "inline" ? (
        <div className="overflow-x-auto">
          <pre className="font-mono text-[11px] leading-relaxed">
            {parts.map((part, i) => {
              const lines = part.value.replace(/\n$/, "").split("\n");
              return lines.map((line, j) => (
                <div
                  key={`${i}-${j}`}
                  className={cn(
                    "px-3",
                    part.added && "bg-green-500/10 text-green-700 dark:text-green-300",
                    part.removed && "bg-red-500/10 text-red-700 dark:text-red-300",
                    !part.added && !part.removed && "text-foreground/70",
                  )}
                >
                  <span className="select-none mr-2 inline-block w-3 text-muted-foreground/40">
                    {part.added ? "+" : part.removed ? "-" : " "}
                  </span>
                  {line}
                </div>
              ));
            })}
          </pre>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-px bg-border">
          <div className="bg-background/40">
            <div className="border-b border-border px-3 py-1 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              Before
            </div>
            <pre className="overflow-x-auto font-mono text-[11px] leading-relaxed">
              {parts.filter((p) => !p.added).map((part, i) => {
                const lines = part.value.replace(/\n$/, "").split("\n");
                return lines.map((line, j) => (
                  <div
                    key={`${i}-${j}`}
                    className={cn(
                      "px-3",
                      part.removed && "bg-red-500/10 text-red-700 dark:text-red-300",
                      !part.removed && "text-foreground/70",
                    )}
                  >
                    {line}
                  </div>
                ));
              })}
            </pre>
          </div>
          <div className="bg-background/40">
            <div className="border-b border-border px-3 py-1 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              After
            </div>
            <pre className="overflow-x-auto font-mono text-[11px] leading-relaxed">
              {parts.filter((p) => !p.removed).map((part, i) => {
                const lines = part.value.replace(/\n$/, "").split("\n");
                return lines.map((line, j) => (
                  <div
                    key={`${i}-${j}`}
                    className={cn(
                      "px-3",
                      part.added && "bg-green-500/10 text-green-700 dark:text-green-300",
                      !part.added && "text-foreground/70",
                    )}
                  >
                    {line}
                  </div>
                ));
              })}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}
