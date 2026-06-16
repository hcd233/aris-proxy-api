"use client";

import { useState } from "react";
import { History } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { SessionHistoryList } from "./session-history-list";

export interface SessionHistorySheetProps {
  activeSessionId: number;
  onSelect: (sessionId: number) => void;
}

export function SessionHistorySheet({
  activeSessionId,
  onSelect,
}: SessionHistorySheetProps) {
  const [open, setOpen] = useState(false);

  const handleSelect = (sessionId: number) => {
    setOpen(false);
    onSelect(sessionId);
  };

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label="Session history">
          <History className="size-5" />
        </Button>
      </SheetTrigger>
      <SheetContent
        side="bottom"
        showCloseButton={false}
        className="h-[80dvh] max-h-[80dvh] rounded-t-[20px] border-border/70 p-0"
      >
        <div className="flex h-full flex-col">
          <SheetHeader className="border-b border-border/60 px-4 py-3 text-left">
            <SheetTitle>History</SheetTitle>
          </SheetHeader>
          <div className="min-h-0 flex-1">
            <SessionHistoryList
              activeSessionId={activeSessionId}
              onSelect={handleSelect}
            />
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
