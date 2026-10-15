"use client";

import { ChatTurn } from "./chat-turn";
import { ArrowDown, Sparkles } from "lucide-react";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import {
  MessageScroller,
  MessageScrollerButton,
  MessageScrollerContent,
  MessageScrollerItem,
  MessageScrollerProvider,
  MessageScrollerViewport,
} from "@/components/ui/message-scroller";
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

/**
 * 对话区：滚动行为交给 MessageScroller——位于底部时跟随流式输出，用户上滚即释放跟随，
 * 「跳到最新」按钮恢复；每轮用户消息作为锚点，新一轮发出时滚到该轮起点。
 */
export function ConversationView({
  messages,
  sending,
  onEditSave,
  onDelete,
  onRerun,
  onRetry,
}: ConversationViewProps) {
  const t = useT();

  if (messages.length === 0) {
    return (
      <Empty className="min-h-0 flex-1">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Sparkles />
          </EmptyMedia>
          <EmptyTitle>{t("playground.empty.title")}</EmptyTitle>
          <EmptyDescription>{t("playground.empty.hint")}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    );
  }

  return (
    <MessageScrollerProvider autoScroll defaultScrollPosition="end">
      {/* h-auto + flex-1：与上方工具行、下方输入区共享父容器高度（组件默认 size-full 会撑满父级） */}
      <MessageScroller className="h-auto min-h-0 flex-1">
        <MessageScrollerViewport>
          <MessageScrollerContent aria-busy={sending} className="px-1 py-6">
            {messages.map((m, i) => (
              <MessageScrollerItem key={m.id} messageId={m.id} scrollAnchor={m.role === "user"}>
                <ChatTurn
                  message={m}
                  index={i}
                  sending={sending && i === messages.length - 1}
                  onEditSave={onEditSave}
                  onDelete={onDelete}
                  onRerun={onRerun}
                  onRetry={onRetry}
                />
              </MessageScrollerItem>
            ))}
          </MessageScrollerContent>
        </MessageScrollerViewport>
        <MessageScrollerButton>
          <ArrowDown />
          <span className="sr-only">{t("playground.jump_to_latest")}</span>
        </MessageScrollerButton>
      </MessageScroller>
    </MessageScrollerProvider>
  );
}
