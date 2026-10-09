"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { MarkdownLite } from "@/components/chat/markdown-lite";
import { copyTextToClipboard } from "@/lib/clipboard";
import { formatCost } from "@/lib/money";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { buildCurl, type PlaygroundMessage } from "@/app/(dashboard)/playground/playground-logic";
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  Copy,
  Pencil,
  RefreshCw,
  Sparkles,
  Trash2,
} from "lucide-react";

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
  const totalTokens = usage ? usage.promptTokens + usage.completionTokens : null;

  return (
    <div className="group/turn flex gap-3">
      <div
        className={cn(
          "flex size-7 shrink-0 items-center justify-center rounded-full",
          isAssistant ? "bg-primary/15 text-primary" : "bg-muted text-muted-foreground",
        )}
      >
        {isAssistant ? (
          <Sparkles className="size-3.5" />
        ) : (
          <span className="text-[10px] font-medium">{message.role === "system" ? "S" : "U"}</span>
        )}
      </div>

      <div className="min-w-0 flex-1">
        {/* 角色标签 + hover 操作条 */}
        <div className="flex items-center justify-between">
          <span className="text-[10px] text-muted-foreground">{message.role}</span>
          {!sending && (
            <div className="flex gap-1 opacity-0 transition-opacity group-hover/turn:opacity-100">
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("playground.turn.copy")}
                onClick={() => void copyTextToClipboard(message.content)}
              >
                <Copy className="size-3.5" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("playground.turn.edit")}
                onClick={() => {
                  setDraft(message.content);
                  setEditing(true);
                }}
              >
                <Pencil className="size-3.5" />
              </Button>
              {isAssistant && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t("playground.turn.rerun")}
                  onClick={() => onRerun(index)}
                >
                  <RefreshCw className="size-3.5" />
                </Button>
              )}
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("playground.turn.delete")}
                onClick={() => onDelete(index)}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          )}
        </div>

        {/* 内容：编辑态 / 展示态 */}
        {editing ? (
          <div className="space-y-2">
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
              <span className="text-[11px] text-destructive">
                {t("playground.turn.edit_warning")}
              </span>
            </div>
          </div>
        ) : (
          <div
            className={cn(
              "mt-1 text-[15px] leading-[1.6]",
              isUser &&
                "w-fit max-w-[85%] rounded-[20px] rounded-br-[6px] bg-accent px-4 py-2.5 md:max-w-[75%]",
            )}
          >
            {isAssistant ? (
              message.content === "" && !meta?.error && !meta?.interrupted ? (
                <span className="text-muted-foreground/60">…</span>
              ) : (
                <MarkdownLite text={message.content} />
              )
            ) : (
              <span className="whitespace-pre-wrap break-words">{message.content}</span>
            )}
          </div>
        )}

        {/* 错误卡 */}
        {meta?.error && (
          <div className="mt-2 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
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

        {/* 指标行 + 展开详情（仅 assistant 且有 meta） */}
        {isAssistant && meta && (
          <div className="mt-1.5">
            <button
              type="button"
              className="flex items-center gap-1 rounded-md bg-muted/60 px-2 py-0.5 text-[10px] text-muted-foreground hover:bg-muted"
              onClick={() => setDetailsOpen((v) => !v)}
            >
              {detailsOpen ? (
                <ChevronDown className="size-3" />
              ) : (
                <ChevronRight className="size-3" />
              )}
              <span>
                {totalTokens !== null ? `${totalTokens} tok` : "—"} · {formatDuration(meta.totalMs)}{" "}
                · {formatCost(meta.cost)}
                {meta.interrupted ? ` · ${t("playground.metrics.interrupted")}` : ""}
              </span>
            </button>
            {detailsOpen && (
              <div className="mt-1.5 rounded-lg border bg-background p-2.5 text-[11px] text-muted-foreground">
                <div>
                  {t("playground.metrics.tokens_in")} <strong>{usage?.promptTokens ?? "—"}</strong>
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
            )}
          </div>
        )}
      </div>
    </div>
  );
}
