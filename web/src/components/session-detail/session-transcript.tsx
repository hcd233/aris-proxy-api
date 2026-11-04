"use client";

import { useMemo, type Ref } from "react";
import { MessagesSquare } from "lucide-react";
import type { MessageItem } from "@/lib/types";
import { Marker, MarkerContent } from "@/components/ui/marker";
import { MessageScrollerItem } from "@/components/ui/message-scroller";
import { Skeleton } from "@/components/ui/skeleton";
import { ChainOfThought } from "@/components/chat/chain-of-thought";
import { ChatMessage, buildToolResultsByID } from "@/components/chat/chat-message";
import { groupTranscript } from "@/components/chat/transcript-groups";

interface SessionTranscriptProps {
  messages: MessageItem[];
  hasMore: boolean;
  /** 无限加载哨兵（IntersectionObserver 的 root 是 ReadingLayout 的 MessageScrollerViewport） */
  sentinelRef: Ref<HTMLDivElement>;
  emptyText: string;
  endText: string;
}

/**
 * 会话转录（详情页与分享页共用）：渲染在 ReadingLayout 提供的 MessageScrollerContent 内，
 * 每个条目一个 MessageScrollerItem；连续工具步骤折叠为 Chain of Thought。
 */
export function SessionTranscript({
  messages,
  hasMore,
  sentinelRef,
  emptyText,
  endText,
}: SessionTranscriptProps) {
  const toolResultsByID = useMemo(() => buildToolResultsByID(messages), [messages]);
  const entries = useMemo(() => groupTranscript(messages), [messages]);

  if (messages.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <MessagesSquare className="mb-3 size-10 text-muted-foreground/40" />
        <p className="text-sm text-muted-foreground">{emptyText}</p>
      </div>
    );
  }

  return (
    <>
      {entries.map((entry) =>
        entry.kind === "steps" ? (
          <MessageScrollerItem key={`steps-${entry.id}`} messageId={`steps-${entry.id}`}>
            <ChainOfThought entry={entry} toolResultsByID={toolResultsByID} />
          </MessageScrollerItem>
        ) : (
          <MessageScrollerItem key={entry.message.id} messageId={String(entry.message.id)}>
            <ChatMessage
              message={entry.message}
              index={entry.index}
              toolResultsByID={toolResultsByID}
            />
          </MessageScrollerItem>
        ),
      )}
      {hasMore ? (
        <div ref={sentinelRef} className="flex justify-center py-3">
          <Skeleton className="h-4 w-32" />
        </div>
      ) : (
        <Marker variant="separator" className="pt-3 pb-1 text-xs">
          <MarkerContent>{endText}</MarkerContent>
        </Marker>
      )}
    </>
  );
}
