"use client";

import { useEffect, useRef, useState } from "react";
import { Alert, AlertAction, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Marker, MarkerContent } from "@/components/ui/marker";
import { Spinner } from "@/components/ui/spinner";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { Message, MessageContent, MessageFooter, MessageHeader } from "@/components/ui/message";
import { Textarea } from "@/components/ui/textarea";
import { MarkdownLite } from "@/components/chat/markdown-lite";
import { Reasoning } from "@/components/chat/reasoning";
import { ProviderIcon } from "@/components/provider-icon";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { type PlaygroundMessage } from "@/app/(dashboard)/playground/playground-logic";
import { ContextCard } from "./context-card";
import { AlertTriangle, Pencil, RefreshCw, Sparkles, Trash2 } from "lucide-react";

interface ChatTurnProps {
  message: PlaygroundMessage;
  index: number;
  /** 该轮模型的上下文窗口（tokens），<=0 表示未知 */
  contextLength: number;
  sending: boolean;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}

/** 消息级 hover 操作（图标按钮，操作条默认透明，hover/focus 浮现） */
function IconAction({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      aria-label={label}
      onClick={onClick}
      className="text-muted-foreground hover:text-foreground"
    >
      {children}
    </Button>
  );
}

/** 首个增量到达前的等待指示：anthropic 依次点下的墨点 / moonshot 绕环卫星
 * （styles/motion-signature.css「⑤ Playground」）。 */
function PendingDots() {
  const t = useT();
  return (
    <span className="pending-dots inline-flex items-center gap-1 py-1.5">
      <span className="sr-only">{t("playground.thinking.streaming")}</span>
      {[0, 150, 300].map((delay) => (
        <span
          key={delay}
          aria-hidden
          style={{ animationDelay: `${delay}ms` }}
          className="pending-dot size-1.5 rounded-full bg-muted-foreground/50"
        />
      ))}
    </span>
  );
}

/** Settle cue after a stream ends (ink dries / light sweeps), ms. */
const STREAM_SETTLE_MS = 900;

export function ChatTurn({
  message,
  index,
  contextLength,
  sending,
  onEditSave,
  onDelete,
  onRerun,
  onRetry,
}: ChatTurnProps) {
  const t = useT();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message.content);

  const isAssistant = message.role === "assistant";
  const isUser = message.role === "user";
  const meta = message.meta;
  const reasoning = message.reasoning ?? "";
  const showPending = isAssistant && sending && !message.content && !meta?.error;
  const streaming = isAssistant && sending && !!message.content;
  const bodyRef = useRef<HTMLDivElement>(null);
  const wasStreaming = useRef(false);

  // Stream end → brief `data-settling` (DOM write, no re-render) so the wet
  // ink / glow tail can dry out instead of vanishing with the caret.
  useEffect(() => {
    const el = bodyRef.current;
    const ended = wasStreaming.current && !streaming;
    wasStreaming.current = streaming;
    if (!el || !ended) return;
    el.dataset.settling = "";
    const timer = window.setTimeout(() => delete el.dataset.settling, STREAM_SETTLE_MS);
    return () => {
      window.clearTimeout(timer);
      delete el.dataset.settling;
    };
  }, [streaming]);

  const actions = !sending && !editing && (
    <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover/turn:opacity-100 group-focus-within/turn:opacity-100">
      {isUser && (
        <CopyButton value={message.content} variant="icon" ariaLabel={t("playground.turn.copy")} />
      )}
      <IconAction
        label={t("playground.turn.edit")}
        onClick={() => {
          setDraft(message.content);
          setEditing(true);
        }}
      >
        <Pencil className="size-3.5" />
      </IconAction>
      {isAssistant && (
        <IconAction label={t("playground.turn.rerun")} onClick={() => onRerun(index)}>
          <RefreshCw className="size-3.5" />
        </IconAction>
      )}
      <IconAction label={t("playground.turn.delete")} onClick={() => onDelete(index)}>
        <Trash2 className="size-3.5" />
      </IconAction>
    </div>
  );

  const editBox = (
    <div className="w-full space-y-2">
      <Textarea
        value={draft}
        autoFocus
        className="min-h-[5rem]"
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") setEditing(false);
        }}
      />
      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          size="sm"
          onClick={() => {
            setEditing(false);
            onEditSave(index, draft);
          }}
        >
          {t("playground.turn.edit_save")}
        </Button>
        <Button type="button" size="sm" variant="outline" onClick={() => setEditing(false)}>
          {t("playground.turn.edit_cancel")}
        </Button>
        <span className="text-[11px] text-destructive">{t("playground.turn.edit_warning")}</span>
      </div>
    </div>
  );

  return (
    <Message
      align={isUser ? "end" : "start"}
      style={{ animationDelay: `${Math.min(index, 12) * 40}ms` }}
      className="group/turn turn-enter"
    >
      <MessageContent>
        {/* 角色标识：assistant 显示模型 icon + 模型名 + 复制；system 用 Marker 分隔条 */}
        {isAssistant && (
          <MessageHeader className="gap-1.5 px-0">
            {meta?.model ? (
              <ProviderIcon protocol={meta.model} size={15} />
            ) : (
              <Sparkles className="size-3.5" />
            )}
            <span>{meta?.model ?? message.role}</span>
            {message.content && (
              <CopyButton
                value={message.content}
                variant="icon"
                ariaLabel={t("playground.turn.copy")}
                className="ml-auto opacity-0 transition-opacity group-hover/turn:opacity-100 focus-visible:opacity-100"
              />
            )}
          </MessageHeader>
        )}
        {!isAssistant && !isUser && (
          <Marker variant="border" className="text-xs">
            <MarkerContent className="font-mono">{message.role}</MarkerContent>
          </Marker>
        )}

        {editing ? (
          editBox
        ) : isUser ? (
          <Bubble variant="secondary" align="end">
            <BubbleContent className="whitespace-pre-wrap">{message.content}</BubbleContent>
          </Bubble>
        ) : isAssistant ? (
          <>
            {reasoning && <Reasoning text={reasoning} streaming={sending} />}
            <Bubble variant="ghost">
              <BubbleContent>
                {/* stream-body + data-streaming/data-settling：motion-signature.css 的流式签名动效
                  按「.stream-body > div（MarkdownLite 根）」命中，这一层结构不能省 */}
                <div
                  ref={bodyRef}
                  className="stream-body"
                  data-streaming={streaming ? "" : undefined}
                >
                  {message.content ? (
                    <MarkdownLite text={message.content} />
                  ) : showPending ? (
                    <PendingDots />
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </div>
              </BubbleContent>
            </Bubble>
          </>
        ) : (
          <Bubble variant="outline" className="max-w-full">
            <BubbleContent className="whitespace-pre-wrap">
              {message.content || <span className="text-muted-foreground">—</span>}
            </BubbleContent>
          </Bubble>
        )}

        {/* 错误卡 */}
        {meta?.error && (
          <Alert variant="destructive">
            <AlertTriangle />
            <AlertDescription className="break-words">
              {meta.error.status ? `${meta.error.status} · ` : ""}
              {meta.error.message}
            </AlertDescription>
            <AlertAction>
              <Button type="button" size="sm" variant="outline" onClick={() => onRetry(index)}>
                <RefreshCw data-icon="inline-start" />
                {t("playground.error.retry")}
              </Button>
            </AlertAction>
          </Alert>
        )}

        {/* 页脚：指标（可展开详情）+ hover 操作条 */}
        {!editing && (
          <MessageFooter className="min-h-7 gap-2 px-0">
            {isAssistant && meta && <ContextCard meta={meta} contextLength={contextLength} />}
            {actions && <div className={cn(isAssistant && "ml-auto")}>{actions}</div>}
          </MessageFooter>
        )}
      </MessageContent>
    </Message>
  );
}
