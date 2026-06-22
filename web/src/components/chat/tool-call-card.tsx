import { useState } from "react";
import { ChevronDown, ChevronRight, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";
import type { UnifiedToolCall } from "@/lib/types";
import type { ToolResultInfo } from "./content-extract";
import { CompressionDiff } from "./compression-diff";

function prettyJSON(s: string): string {
  if (!s) return "";
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

function previewFirstArg(argsJSON: string): string {
  if (!argsJSON) return "";
  try {
    const parsed = JSON.parse(argsJSON) as Record<string, unknown>;
    const entries = Object.entries(parsed);
    if (entries.length === 0) return "";
    const [key, value] = entries[0];
    const valStr =
      typeof value === "string" ? `"${value}"` : String(value);
    return `${key}: ${valStr}`;
  } catch {
    return "";
  }
}

interface ToolCallCardProps {
  call: UnifiedToolCall;
  result?: ToolResultInfo;
}

export function ToolCallCard({ call, result }: ToolCallCardProps) {
  const [open, setOpen] = useState(false);
  const [showDiff, setShowDiff] = useState(false);
  const args = prettyJSON(call.arguments);
  const out = result ? prettyJSON(result.text) : undefined;
  const preview = previewFirstArg(call.arguments);
  const hasCompression = !!result?.rawContent && !!result?.compressionStrategy;

  return (
    <div
      className={cn(
        "mt-3 overflow-hidden rounded-lg border border-border bg-card",
      )}
    >
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2.5 px-3 py-2 text-left transition-colors hover:bg-muted/30"
      >
        <div className="flex size-6 shrink-0 items-center justify-center rounded-md bg-primary/12 text-primary">
          <Wrench className="size-3.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="font-mono text-[13px] font-medium text-foreground">
              {call.name || "tool"}
            </span>
            {!open && preview && (
              <span className="ml-1 flex-1 truncate font-mono text-[11px] text-muted-foreground">
                {preview}
              </span>
            )}
          </div>
        </div>
        {hasCompression && (
          <span className="rounded bg-amber-500/12 px-1.5 py-0.5 font-mono text-[9px] font-medium text-amber-600 dark:text-amber-400">
            {result!.compressionStrategy}
          </span>
        )}
        {open ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {open && (
        <div className="border-t border-border bg-background/40 min-w-0">
          {call.id && (
            <div className="border-b border-border/50 px-3 py-1.5">
              <span className="font-mono text-[10px] text-muted-foreground/60">
                {call.id}
              </span>
            </div>
          )}
          <div className="px-3 py-2.5">
            <p className="mb-1.5 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              Input
            </p>
            <pre className="overflow-x-auto rounded-md bg-muted/40 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90 max-w-full">
              {args || "{}"}
            </pre>
          </div>
          {out !== undefined && (
            <div className="border-t border-border px-3 py-2.5">
              <div className="mb-1.5 flex items-center justify-between">
                <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                  Output
                </p>
                {hasCompression && (
                  <button
                    type="button"
                    onClick={() => setShowDiff((v) => !v)}
                    className="font-mono text-[10px] text-primary hover:underline"
                  >
                    {showDiff ? "查看压缩后内容" : "查看原始内容"}
                  </button>
                )}
              </div>
              {showDiff && hasCompression ? (
                <CompressionDiff
                  before={result!.rawContent!}
                  after={result!.text}
                />
              ) : (
                <pre className="overflow-x-auto rounded-md bg-muted/40 px-3 py-2.5 font-mono text-[12px] leading-relaxed text-foreground/90 max-w-full">
                  {out}
                </pre>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
