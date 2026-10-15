"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Item, ItemActions, ItemContent, ItemGroup, ItemTitle } from "@/components/ui/item";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  TooltipContent,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { MessageSquarePlus, Pencil, Trash2 } from "lucide-react";
import { useT } from "@/lib/i18n";
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

  const commitRename = (id: string) => {
    onRename(id, draft.trim());
    setRenamingId(null);
  };

  return (
    <div className="flex h-full flex-col">
      <Button type="button" size="sm" className="mb-3" onClick={onNew}>
        <MessageSquarePlus data-icon="inline-start" />
        {t("playground.session.new")}
      </Button>
      <ScrollArea className="min-h-0 flex-1">
        <ItemGroup className="gap-1">
          {sessions.map((s) => {
            const title = s.title || t("playground.session.untitled");
            return (
              <Item
                key={s.id}
                size="xs"
                role="listitem"
                variant={s.id === activeId ? "muted" : "default"}
                className="cursor-pointer flex-nowrap gap-1 hover:bg-muted"
                onClick={() => onSelect(s.id)}
              >
                {renamingId === s.id ? (
                  <Input
                    value={draft}
                    autoFocus
                    className="h-7 flex-1"
                    onChange={(e) => setDraft(e.target.value)}
                    onClick={(e) => e.stopPropagation()}
                    onBlur={() => commitRename(s.id)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") commitRename(s.id);
                      if (e.key === "Escape") setRenamingId(null);
                    }}
                  />
                ) : (
                  <>
                    <ItemContent className="min-w-0">
                      <TooltipProvider>
                        <TooltipRoot>
                          <TooltipTrigger
                            render={<ItemTitle className="w-full truncate">{title}</ItemTitle>}
                          />
                          <TooltipContent className="max-w-xs break-all">{title}</TooltipContent>
                        </TooltipRoot>
                      </TooltipProvider>
                    </ItemContent>
                    <ItemActions className="gap-0.5 opacity-0 transition-opacity group-hover/item:opacity-100">
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
                        <Pencil />
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
                        <Trash2 />
                      </Button>
                    </ItemActions>
                  </>
                )}
              </Item>
            );
          })}
        </ItemGroup>
      </ScrollArea>
    </div>
  );
}
