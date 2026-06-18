"use client";

import { useState } from "react";
import {
  Braces,
  ChevronDown,
  ChevronRight,
  FileText,
  Hash,
  Wrench,
} from "lucide-react";
import type { ToolItem, UnifiedTool } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { CollapsibleText } from "./collapsible-text";

export function ToolSidebarItem({ tool }: { tool: ToolItem }) {
  const [expanded, setExpanded] = useState(false);
  const toolData: UnifiedTool = tool.tool;

  const params = toolData.parameters;
  const paramProperties =
    (params?.properties as Record<string, Record<string, unknown>>) ?? {};
  const requiredParams = (params?.required as string[]) ?? [];

  return (
    <div className="rounded-xl border border-border/70 bg-card/60">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex min-h-[52px] w-full items-center gap-3 px-3.5 py-2.5 text-left transition-colors active:bg-accent/50 md:hover:bg-accent/40"
      >
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary/15 text-primary">
          <Wrench className="size-4" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="truncate font-mono text-[13.5px] font-medium text-foreground">
            {toolData.name}
          </p>
          <p className="truncate text-[12px] leading-snug text-muted-foreground">
            {toolData.description || "No description"}
          </p>
        </div>
        {expanded ? (
          <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronRight className="size-4 shrink-0 text-muted-foreground" />
        )}
      </button>
      {expanded && (
        <div className="space-y-3 border-t border-border/60 px-3.5 py-3">
          {toolData.description && (
            <div>
              <p className="mb-1 flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                <FileText className="size-3" />
                Description
              </p>
              <CollapsibleText
                text={toolData.description}
                previewChars={140}
                className="text-[13px] leading-relaxed text-foreground/85"
              />
            </div>
          )}
          {Object.keys(paramProperties).length > 0 && (
            <div>
              <p className="mb-1.5 flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                <Braces className="size-3" />
                Parameters
              </p>
              <div className="space-y-1.5">
                {Object.entries(paramProperties).map(([name, schema]) => (
                  <div
                    key={name}
                    className="rounded-md bg-muted/50 px-2.5 py-1.5"
                  >
                    <div className="flex items-center gap-1.5">
                      <span className="font-mono text-[12px] font-medium text-foreground">
                        {name}
                      </span>
                      {requiredParams.includes(name) && (
                        <span className="text-[9px] font-medium uppercase tracking-wider text-rose-500">
                          required
                        </span>
                      )}
                      {schema.type !== undefined && (
                        <Badge
                          variant="secondary"
                          className="ml-auto px-1.5 py-0 font-mono text-[9px]"
                        >
                          {schema.type as string}
                        </Badge>
                      )}
                    </div>
                    {schema.description !== undefined && (
                      <CollapsibleText
                        text={schema.description as string}
                        previewChars={100}
                        className="mt-1 text-[11.5px] leading-relaxed text-muted-foreground"
                      />
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
          {params?.type !== undefined && (
            <div className="flex items-center gap-1.5 text-[10px] text-muted-foreground/60">
              <Hash className="size-3" />
              Schema type: {params.type as string}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
