"use client";

import { useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  TooltipContent,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ProviderIcon } from "@/components/provider-icon";
import { ArrowUp, KeyRound, Plus, Square } from "lucide-react";
import { useT } from "@/lib/i18n";
import type { APIKeyItem } from "@/lib/types";
import type {
  PlaygroundModelOption,
  PlaygroundParams,
} from "@/app/(dashboard)/playground/playground-logic";

interface ComposerProps {
  models: PlaygroundModelOption[];
  keys: APIKeyItem[];
  keysLoaded: boolean;
  params: PlaygroundParams;
  onParamsChange: (patch: Partial<PlaygroundParams>) => void;
  sending: boolean;
  disabled: boolean;
  onSend: (content: string) => void;
  onStop: () => void;
  onInsertRole: (role: "system" | "assistant") => void;
}

/** 选择器胶囊外观：无边框幽灵态，悬停浮出底色（宽度由外层 w-fit 容器按内容收紧） */
const PILL_CLASS =
  "h-7 gap-1.5 rounded-full border-transparent px-2 text-xs text-muted-foreground hover:bg-secondary hover:text-foreground dark:bg-transparent";

/** 截断文本 + 悬停展示全文（项目契约：truncate 必须可悬停看全） */
function TruncatedLabel({ text }: { text: string }) {
  return (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger render={<span className="truncate">{text}</span>} />
        <TooltipContent className="max-w-xs break-all">{text}</TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );
}

/** 插入特殊角色消息的图标按钮（system / assistant 由调试台手工构造上下文） */
function InsertRoleButton({ label, onInsert }: { label: string; onInsert: () => void }) {
  return (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={label}
              className="rounded-full text-muted-foreground hover:text-foreground"
              onClick={onInsert}
            >
              <Plus className="size-3.5" />
            </Button>
          }
        />
        <TooltipContent>{label}</TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );
}

export function Composer({
  models,
  keys,
  keysLoaded,
  params,
  onParamsChange,
  sending,
  disabled,
  onSend,
  onStop,
  onInsertRole,
}: ComposerProps) {
  const t = useT();
  const [value, setValue] = useState("");

  const submit = () => {
    const content = value.trim();
    if (!content || disabled || sending) return;
    onSend(content);
    setValue("");
  };

  const modelSelect = (
    <span className="flex w-fit max-w-[13rem] min-w-0">
      <Select
        value={params.model}
        onValueChange={(v) => onParamsChange({ model: String(v ?? "") })}
      >
        <SelectTrigger size="sm" aria-label={t("playground.model")} className={PILL_CLASS}>
          <SelectValue className="min-w-0" placeholder={t("playground.model.placeholder")}>
            {(selected) =>
              selected ? (
                <span className="flex min-w-0 items-center gap-1.5">
                  <ProviderIcon protocol={String(selected)} size={13} />
                  <TruncatedLabel text={String(selected)} />
                </span>
              ) : (
                <span>{t("playground.model.placeholder")}</span>
              )
            }
          </SelectValue>
        </SelectTrigger>
        <SelectContent className="w-auto max-w-[22rem] min-w-(--anchor-width)">
          {models.map((m) => (
            <SelectItem key={m.alias} value={m.alias}>
              <span className="flex items-center gap-2">
                <ProviderIcon protocol={m.alias} size={14} />
                {m.alias}
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </span>
  );

  // 悬停解释计费归属：调试用量计入所选 Key 的审计与限流配额
  const keySelect = (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <span className="flex w-fit max-w-[13rem] min-w-0">
              <Select
                value={params.apiKeyID === null ? "" : String(params.apiKeyID)}
                onValueChange={(v) => onParamsChange({ apiKeyID: v ? Number(v) : null })}
              >
                <SelectTrigger
                  size="sm"
                  aria-label={t("playground.api_key")}
                  className={PILL_CLASS}
                >
                  <SelectValue
                    className="min-w-0"
                    placeholder={t("playground.api_key.placeholder")}
                  >
                    {(selected) =>
                      selected ? (
                        <span className="flex min-w-0 items-center gap-1.5">
                          <KeyRound className="size-3.5" />
                          <TruncatedLabel
                            text={keys.find((k) => String(k.id) === String(selected))?.name ?? ""}
                          />
                        </span>
                      ) : (
                        <span>{t("playground.api_key.placeholder")}</span>
                      )
                    }
                  </SelectValue>
                </SelectTrigger>
                <SelectContent className="w-auto max-w-[22rem] min-w-(--anchor-width)">
                  {keys.map((k) => (
                    <SelectItem key={k.id} value={String(k.id)}>
                      <span className="flex items-center gap-2">
                        <KeyRound className="size-3.5 text-muted-foreground" />
                        {k.name}
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </span>
          }
        />
        <TooltipContent className="max-w-xs">{t("playground.api_key.hint")}</TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );

  return (
    <div className="space-y-2">
      {keysLoaded && keys.length === 0 && (
        <div className="flex items-start gap-2 rounded-xl border border-border bg-muted/40 px-3 py-2 text-[12.5px] text-muted-foreground">
          <KeyRound className="mt-0.5 size-3.5 shrink-0" />
          <span>
            {t("playground.api_key.empty")}{" "}
            <Link href="/apikeys/" className="underline underline-offset-2">
              {t("playground.api_key.create")}
            </Link>
          </span>
        </div>
      )}

      <div className="rounded-[1.5rem] border border-border bg-card px-3 pt-2 pb-2 shadow-sm transition-[border-color,box-shadow] focus-within:border-primary/40 focus-within:shadow-md">
        <Textarea
          value={value}
          placeholder={t("playground.composer.placeholder")}
          className="max-h-48 min-h-[3rem] resize-none border-0 bg-transparent px-1 py-1.5 text-[15px] leading-[1.6] shadow-none focus-visible:ring-0 dark:bg-transparent"
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            // 输入法组词中的 Enter 属于选词，不当作发送
            if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <div className="flex flex-wrap items-center justify-between gap-1.5 pt-1">
          <div className="flex items-center gap-0.5">
            <InsertRoleButton
              label={t("playground.composer.insert_system")}
              onInsert={() => onInsertRole("system")}
            />
            <InsertRoleButton
              label={t("playground.composer.insert_assistant")}
              onInsert={() => onInsertRole("assistant")}
            />
          </div>

          <div className="flex min-w-0 items-center gap-1">
            {modelSelect}
            {keySelect}
            {sending ? (
              <Button
                type="button"
                variant="outline"
                size="icon-sm"
                className="rounded-full"
                aria-label={t("playground.stop")}
                onClick={onStop}
              >
                <Square className="size-3.5" />
              </Button>
            ) : (
              <Button
                type="button"
                size="icon-sm"
                className="rounded-full"
                aria-label={t("playground.send")}
                disabled={disabled || value.trim() === ""}
                onClick={submit}
              >
                <ArrowUp className="size-4" />
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
