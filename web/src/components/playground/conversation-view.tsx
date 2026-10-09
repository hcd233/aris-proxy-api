"use client";

import { useEffect, useRef } from "react";
import { ChatTurn } from "./chat-turn";
import { useT } from "@/lib/i18n";
import type { PlaygroundMessage } from "@/app/(dashboard)/playground/playground-logic";

interface ConversationViewProps {
  messages: PlaygroundMessage[];
  sending: boolean;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}

export function ConversationView({
  messages,
  sending,
  onEditSave,
  onDelete,
  onRerun,
  onRetry,
}: ConversationViewProps) {
  const t = useT();
  const scrollRef = useRef<HTMLDivElement>(null);
  const followRef = useRef(true);

  // 有新内容且处于跟随态时滚到底
  useEffect(() => {
    const el = scrollRef.current;
    if (el && followRef.current) el.scrollTop = el.scrollHeight;
  }, [messages]);

  return (
    <div
      ref={scrollRef}
      className="flex-1 space-y-5 overflow-y-auto px-1 py-4"
      aria-live="polite"
      onScroll={() => {
        const el = scrollRef.current;
        if (!el) return;
        followRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
      }}
    >
      {messages.length === 0 ? (
        <p className="pt-16 text-center text-sm text-muted-foreground">
          {t("playground.session.empty_hint")}
        </p>
      ) : (
        messages.map((m, i) => (
          <ChatTurn
            key={m.id}
            message={m}
            index={i}
            sending={sending && i === messages.length - 1}
            onEditSave={onEditSave}
            onDelete={onDelete}
            onRerun={onRerun}
            onRetry={onRetry}
          />
        ))
      )}
    </div>
  );
}
