"use client";

import { type Ref } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import type { ToolItem } from "@/lib/types";
import { ToolSidebarItem } from "./tool-sidebar-item";

export interface ToolsRailProps {
  tools: ToolItem[];
  hasMore: boolean;
  sentinelRef?: Ref<HTMLDivElement>;
}

export function ToolsRail({ tools, hasMore, sentinelRef }: ToolsRailProps) {
  return (
    <div className="space-y-2">
      {tools.map((t) => (
        <ToolSidebarItem key={t.id} tool={t} />
      ))}
      {hasMore && (
        <div ref={sentinelRef} className="flex justify-center py-3">
          <Skeleton className="h-4 w-24" />
        </div>
      )}
    </div>
  );
}
