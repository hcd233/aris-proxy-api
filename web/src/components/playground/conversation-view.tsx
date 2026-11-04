"use client";

import { ChatTurn } from "./chat-turn";
import { ArrowDown, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Empty,
  EmptyContent,
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
  /** 模型别名 → 上下文窗口（tokens），用于 Context 卡片 */
  contextLengths: Record<string, number>;
  sending: boolean;
  /** 空态一键发送示例提示词；模型/Key 未就绪时由调用方关掉 */
  canSuggest: boolean;
  onSuggest: (content: string) => void;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}

/** 空态示例提示词：分别覆盖流式、结构化输出、推理链路与基础问答 */
const SUGGESTION_KEYS = [
  "playground.suggestion.stream",
  "playground.suggestion.json",
  "playground.suggestion.reasoning",
  "playground.suggestion.explain",
] as const;

/**
 * 对话区：滚动行为交给 MessageScroller——位于底部时跟随流式输出，用户上滚即释放跟随，
 * 「跳到最新」按钮恢复；每轮用户消息作为锚点，新一轮发出时滚到该轮起点。
 */
export function ConversationView({
  messages,
  contextLengths,
  sending,
  canSuggest,
  onSuggest,
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
        {/* Suggestion：可点即发的短提示词 */}
        <EmptyContent>
          <div className="flex flex-wrap justify-center gap-2">
            {SUGGESTION_KEYS.map((key) => (
              <Button
                key={key}
                type="button"
                variant="outline"
                size="sm"
                className="rounded-full"
                disabled={!canSuggest || sending}
                onClick={() => onSuggest(t(key))}
              >
                {t(key)}
              </Button>
            ))}
          </div>
        </EmptyContent>
      </Empty>
    );
  }

  return (
    <MessageScrollerProvider autoScroll defaultScrollPosition="end">
      {/* h-auto + flex-1：与上方工具行、下方输入区共享父容器高度（组件默认 size-full 会撑满父级） */}
      <MessageScroller className="h-auto min-h-0 flex-1">
        {/* 两侧对称预留滚动条槽（globals.css 的 8px 滚动条）：正文列宽不随滚动条出现而跳变，并与 Composer 的 px-3 等宽 */}
        <MessageScrollerViewport className="[scrollbar-gutter:stable_both-edges]">
          <MessageScrollerContent aria-busy={sending} className="px-1 py-6">
            {messages.map((m, i) => (
              <MessageScrollerItem key={m.id} messageId={m.id} scrollAnchor={m.role === "user"}>
                <ChatTurn
                  message={m}
                  index={i}
                  contextLength={m.meta?.model ? (contextLengths[m.meta.model] ?? 0) : 0}
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
