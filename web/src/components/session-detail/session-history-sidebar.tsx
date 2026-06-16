"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Separator } from "@/components/ui/separator";
import { SessionHistoryList } from "./session-history-list";

export function SessionHistorySidebar() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const activeSessionId = Number(searchParams.get("id") ?? NaN);

  if (Number.isNaN(activeSessionId)) return null;

  return (
    <>
      <Separator className="my-3 bg-sidebar-border/50" />
      <div className="px-3 pb-2">
        <h3 className="text-[11px] font-semibold uppercase tracking-wider text-sidebar-foreground/50">
          History
        </h3>
      </div>
      <div className="px-2">
        <SessionHistoryList
          activeSessionId={activeSessionId}
          onSelect={(sessionId) => router.push(`/sessions/detail?id=${sessionId}`)}
        />
      </div>
    </>
  );
}
