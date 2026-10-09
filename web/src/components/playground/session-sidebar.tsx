"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  TooltipContent,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { MessageSquarePlus, Pencil, Trash2 } from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { PlaygroundSession } from "@/app/(dashboard)/playground/playground-logic";

interface SessionSidebarProps {
  sessions: PlaygroundSession[];
  activeId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onRename: (id: string, title: string) => void;
  onDelete: (id: string) => void;
}

export function SessionSidebar({
  sessions,
  activeId,
  onSelect,
  onNew,
  onRename,
  onDelete,
}: SessionSidebarProps) {
  const t = useT();
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");

  return (
    <div className="flex h-full flex-col">
      <Button type="button" size="sm" className="mb-3" onClick={onNew}>
        <MessageSquarePlus className="size-4" />
        {t("playground.session.new")}
      </Button>
      <div className="flex-1 space-y-1 overflow-y-auto">
        {sessions.map((s) => {
          const title = s.title || t("playground.session.untitled");
          return (
            <div
              key={s.id}
              className={cn(
                "group/item flex cursor-pointer items-center gap-1 rounded-lg px-2.5 py-2 hover:bg-accent/50",
                s.id === activeId && "bg-accent",
              )}
              onClick={() => onSelect(s.id)}
            >
              {renamingId === s.id ? (
                <Input
                  value={draft}
                  autoFocus
                  className="h-7 flex-1"
                  onChange={(e) => setDraft(e.target.value)}
                  onClick={(e) => e.stopPropagation()}
                  onBlur={() => {
                    onRename(s.id, draft.trim());
                    setRenamingId(null);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      onRename(s.id, draft.trim());
                      setRenamingId(null);
                    }
                    if (e.key === "Escape") setRenamingId(null);
                  }}
                />
              ) : (
                <>
                  <TooltipProvider>
                    <TooltipRoot>
                      <TooltipTrigger
                        render={<span className="min-w-0 flex-1 truncate text-sm">{title}</span>}
                      />
                      <TooltipContent className="max-w-xs break-all">{title}</TooltipContent>
                    </TooltipRoot>
                  </TooltipProvider>
                  <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover/item:opacity-100">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t("playground.session.rename")}
                      onClick={(e) => {
                        e.stopPropagation();
                        setDraft(s.title);
                        setRenamingId(s.id);
                      }}
                    >
                      <Pencil className="size-3" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t("playground.session.delete")}
                      onClick={(e) => {
                        e.stopPropagation();
                        onDelete(s.id);
                      }}
                    >
                      <Trash2 className="size-3" />
                    </Button>
                  </div>
                </>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
