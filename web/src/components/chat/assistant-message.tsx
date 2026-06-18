import { Sparkles, ShieldAlert } from "lucide-react";
import type { MessageItem, UnifiedToolCall } from "@/lib/types";
import { ProviderIcon } from "@/components/provider-icon";
import { cn } from "@/lib/utils";
import { extractContent, lookupToolResult } from "./content-extract";
import { MultimodalParts } from "./multimodal-parts";
import { ReasoningBlock } from "./reasoning-block";
import { ToolCallCard } from "./tool-call-card";
import { MarkdownLite } from "./markdown-lite";

function PartRefusal({ text }: { text: string }) {
  return (
    <div className="my-2 flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
      <ShieldAlert className="mt-0.5 size-4 shrink-0" />
      <span className="whitespace-pre-wrap break-words">{text}</span>
    </div>
  );
}

function modelIcon(model: string) {
  return <ProviderIcon protocol={model} size={14} className="shrink-0" />;
}

interface AssistantMessageProps {
  message: MessageItem;
  index: number;
  toolResultsByID: Record<string, string>;
}

export function AssistantMessage({
  message,
  index,
  toolResultsByID,
}: AssistantMessageProps) {
  const { role, content, tool_calls, reasoning_content, refusal } =
    message.message;
  const { model } = message;
  const { text, parts } = extractContent(content);
  const isAssistant = role === "assistant";

  const time = message.createdAt
    ? new Date(message.createdAt).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : undefined;

  const style = { animationDelay: `${Math.min(index, 12) * 40}ms` };

  return (
    <div
      style={style}
      className="animate-in fade-in slide-in-from-bottom-1 flex gap-3 duration-300"
    >
      <div className="flex flex-col items-center gap-1 pt-0.5">
        <div
          className={cn(
            "flex size-7 shrink-0 items-center justify-center rounded-full",
            isAssistant
              ? "bg-primary/15 text-primary"
              : "bg-muted text-muted-foreground",
          )}
        >
          {isAssistant ? (
            model ? (
              modelIcon(model) ?? <Sparkles className="size-3.5" />
            ) : (
              <Sparkles className="size-3.5" />
            )
          ) : (
            <ShieldAlert className="size-3.5" />
          )}
        </div>
        {time && (
          <span className="text-[9px] leading-none text-muted-foreground/60">
            {time}
          </span>
        )}
      </div>

      <div className="min-w-0 flex-1">
        {reasoning_content && <ReasoningBlock text={reasoning_content} />}

        <div className="text-[15px] leading-[1.6] text-foreground">
          {parts.length > 0 && <MultimodalParts parts={parts} />}
          {text && <MarkdownLite text={text} />}
          {!text &&
            parts.length === 0 &&
            !tool_calls?.length &&
            !refusal && (
              <span className="text-muted-foreground/60">—</span>
            )}
        </div>

        {refusal && <PartRefusal text={refusal} />}

        {tool_calls && tool_calls.length > 0 && (
          <div>
            {tool_calls.map((call: UnifiedToolCall, i: number) => (
              <ToolCallCard
                key={call.id ?? i}
                call={call}
                result={
                  call.id
                    ? lookupToolResult(toolResultsByID, call.id)
                    : undefined
                }
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
