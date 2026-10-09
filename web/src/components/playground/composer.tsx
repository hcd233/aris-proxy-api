"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Play, Plus, Settings2, Square } from "lucide-react";
import { useT } from "@/lib/i18n";

interface ComposerProps {
  model: string;
  stream: boolean;
  sending: boolean;
  disabled: boolean;
  onSend: (content: string) => void;
  onStop: () => void;
  onInsertRole: (role: "system" | "assistant") => void;
  onOpenParams: () => void;
}

export function Composer({
  model,
  stream,
  sending,
  disabled,
  onSend,
  onStop,
  onInsertRole,
  onOpenParams,
}: ComposerProps) {
  const t = useT();
  const [value, setValue] = useState("");

  const submit = () => {
    const content = value.trim();
    if (!content || disabled || sending) return;
    onSend(content);
    setValue("");
  };

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => onInsertRole("system")}>
          <Plus className="size-3.5" />
          {t("playground.composer.insert_system")}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => onInsertRole("assistant")}>
          <Plus className="size-3.5" />
          {t("playground.composer.insert_assistant")}
        </Button>
      </div>

      <div className="rounded-xl border bg-background p-2.5 shadow-sm">
        <Textarea
          value={value}
          placeholder={t("playground.composer.placeholder")}
          className="min-h-[3.5rem] resize-none border-0 p-1 shadow-none focus-visible:ring-0"
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <div className="mt-1.5 flex items-center justify-between">
          <div className="flex items-center gap-1.5">
            <Badge variant="secondary" className="font-mono text-[10px]">
              {model || t("playground.model.placeholder")}
            </Badge>
            <Badge variant="secondary" className="text-[10px]">
              {stream ? t("playground.stream") : t("playground.composer.non_stream")}
            </Badge>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={t("playground.params.open")}
              onClick={onOpenParams}
            >
              <Settings2 className="size-3.5" />
            </Button>
          </div>
          {sending ? (
            <Button type="button" variant="outline" size="sm" onClick={onStop}>
              <Square className="size-3.5" />
              {t("playground.stop")}
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              disabled={disabled || value.trim() === ""}
              onClick={submit}
            >
              <Play className="size-3.5" />
              {t("playground.send")}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
