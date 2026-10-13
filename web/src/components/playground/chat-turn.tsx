"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { MarkdownLite } from "@/components/chat/markdown-lite";
import { ProviderIcon } from "@/components/provider-icon";
import { copyTextToClipboard } from "@/lib/clipboard";
import { formatCost } from "@/lib/money";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { buildCurl, type PlaygroundMessage } from "@/app/(dashboard)/playground/playground-logic";
import {
  AlertTriangle,
  Brain,
  ChevronRight,
  Copy,
  Pencil,
  RefreshCw,
  Sparkles,
  Trash2,
} from "lucide-react";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";

interface ChatTurnProps {
  message: PlaygroundMessage;
  index: number;
  sending: boolean;
  onEditSave: (index: number, content: string) => void;
  onDelete: (index: number) => void;
  onRerun: (index: number) => void;
  onRetry: (index: number) => void;
}

function formatDuration(ms?: number): string {
  if (ms === undefined) return "—";
  return ms < 1000 ? `${Math.round(ms)}ms` : `${(ms / 1000).toFixed(1)}s`;
}

function DetailCopyButton({ label, value }: { label: string; value: string }) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      onClick={() => void copyTextToClipboard(value)}
    >
      <Copy className="size-3.5" />
      {label}
    </Button>
  );
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

/**
 * 思考内容（reasoning_content）：流式期间默认展开，接收结束后自动收起；
 * 用户手动切换过就固定住不再跟随（userOpen 为 null 表示跟随流式态）。
 */
function ThinkingBlock({ text, streaming }: { text: string; streaming: boolean }) {
  const t = useT();
  const [userOpen, setUserOpen] = useState<boolean | null>(null);
  const open = userOpen ?? streaming;

  return (
    <Collapsible
      open={open}
      onOpenChange={setUserOpen}
      className="rounded-xl border border-border/70 bg-muted/30 px-3 py-2"
    >
      <CollapsibleTrigger className="flex w-full items-center gap-1.5 text-[12px] font-medium text-muted-foreground transition-colors hover:text-foreground">
        <Brain className="size-3.5 text-primary/70" />
        <span>{streaming ? t("playground.thinking.streaming") : t("chat.thought_process")}</span>
        {streaming && (
          <span className="size-1.5 animate-pulse rounded-full bg-primary/70 motion-reduce:animate-none" />
        )}
        <ChevronRight className="ml-auto size-3.5 in-data-panel-open:rotate-90" />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <p className="mt-2 border-l-2 border-border pl-3 text-[13px] italic leading-[1.6] break-words whitespace-pre-wrap text-muted-foreground">
          {text}
        </p>
      </CollapsibleContent>
    </Collapsible>
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
  sending,
  onEditSave,
  onDelete,
  onRerun,
  onRetry,
}: ChatTurnProps) {
  const t = useT();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message.content);
  const [detailsOpen, setDetailsOpen] = useState(false);

  const isAssistant = message.role === "assistant";
  const isUser = message.role === "user";
  const meta = message.meta;
  const usage = meta?.usage;
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
      <IconAction
        label={t("playground.turn.copy")}
        onClick={() => void copyTextToClipboard(message.content)}
      >
        <Copy className="size-3.5" />
      </IconAction>
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
    <div
      style={{ animationDelay: `${Math.min(index, 12) * 40}ms` }}
      className={cn(
        "group/turn turn-enter",
        isUser ? "flex flex-col items-end gap-1" : "flex flex-col gap-1.5",
      )}
    >
      {/* 角色标识：assistant 显示模型 icon + 模型名；system 显示角色标签 */}
      {isAssistant && (
        <div className="flex items-center gap-1.5 text-[13px] font-medium text-foreground/80">
          {meta?.model ? (
            <ProviderIcon protocol={meta.model} size={15} />
          ) : (
            <Sparkles className="size-3.5 text-muted-foreground" />
          )}
          <span>{meta?.model ?? message.role}</span>
        </div>
      )}
      {!isAssistant && !isUser && (
        <span className="w-fit rounded-md bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
          {message.role}
        </span>
      )}

      {editing ? (
        editBox
      ) : isUser ? (
        <div className="w-fit max-w-[85%] rounded-[20px] rounded-br-[8px] bg-accent px-4 py-2.5 text-[15px] leading-[1.6] break-words whitespace-pre-wrap">
          {message.content}
        </div>
      ) : isAssistant ? (
        <>
          {reasoning && <ThinkingBlock text={reasoning} streaming={sending} />}
          <div
            ref={bodyRef}
            className="stream-body text-[15px] leading-[1.7]"
            data-streaming={streaming ? "" : undefined}
          >
            {message.content ? (
              <MarkdownLite text={message.content} />
            ) : showPending ? (
              <PendingDots />
            ) : (
              <span className="text-muted-foreground/60">—</span>
            )}
          </div>
        </>
      ) : (
        <div className="w-full rounded-xl border border-dashed border-border bg-muted/30 px-3 py-2 text-[13.5px] leading-[1.6] break-words whitespace-pre-wrap text-muted-foreground">
          {message.content || <span className="text-muted-foreground/60">—</span>}
        </div>
      )}

      {/* 错误卡 */}
      {meta?.error && (
        <div className="flex items-start gap-2 rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" />
          <div className="min-w-0 flex-1">
            <div className="break-words">
              {meta.error.status ? `${meta.error.status} · ` : ""}
              {meta.error.message}
            </div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              className="mt-1.5"
              onClick={() => onRetry(index)}
            >
              <RefreshCw className="size-3.5" />
              {t("playground.error.retry")}
            </Button>
          </div>
        </div>
      )}

      {/* 页脚：指标（可展开详情）+ hover 操作条 */}
      {!editing && (
        <div className={cn("flex min-h-7 items-center gap-2", isUser && "justify-end")}>
          {isAssistant && meta && (
            <Collapsible open={detailsOpen} onOpenChange={setDetailsOpen}>
              <CollapsibleTrigger className="flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground">
                <ChevronRight className="size-3 in-data-panel-open:rotate-90" />
                <span>
                  {usage ? `${usage.promptTokens + usage.completionTokens} tok` : "—"} ·{" "}
                  {formatDuration(meta.totalMs)} · {formatCost(meta.cost)}
                  {meta.interrupted ? ` · ${t("playground.metrics.interrupted")}` : ""}
                </span>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div className="mt-1.5 rounded-xl border bg-background p-2.5 text-[11px] text-muted-foreground">
                  <div>
                    {t("playground.metrics.tokens_in")}{" "}
                    <strong>{usage?.promptTokens ?? "—"}</strong>
                    {" · "}
                    {t("playground.metrics.tokens_cached")}{" "}
                    <strong>{usage?.cachedTokens ?? "—"}</strong>
                    {" · "}
                    {t("playground.metrics.tokens_out")}{" "}
                    <strong>{usage?.completionTokens ?? "—"}</strong>
                  </div>
                  <div>
                    {t("playground.metrics.first_token")}{" "}
                    <strong>{formatDuration(meta.firstTokenMs)}</strong>
                    {" · "}
                    {t("playground.metrics.total_time")}{" "}
                    <strong>{formatDuration(meta.totalMs)}</strong>
                  </div>
                  <div className="mt-1">{t("playground.metrics.estimate_hint")}</div>
                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    <DetailCopyButton
                      label={t("playground.metrics.request_json")}
                      value={JSON.stringify(meta.requestSnapshot ?? {}, null, 2)}
                    />
                    <DetailCopyButton
                      label={t("playground.metrics.copy_curl")}
                      value={buildCurl(meta.requestSnapshot ?? {}, window.location.origin)}
                    />
                  </div>
                </div>
              </CollapsibleContent>
            </Collapsible>
          )}
          {actions && <div className={cn(isAssistant && "ml-auto")}>{actions}</div>}
        </div>
      )}
    </div>
  );
}
