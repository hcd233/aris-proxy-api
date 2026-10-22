import { Sparkles, ShieldAlert } from "lucide-react";
import type { MessageItem, UnifiedToolCall } from "@/lib/types";
import { ProviderIcon } from "@/components/provider-icon";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { CopyButton } from "@/components/ui/copy-button";
import {
  Message,
  MessageAvatar,
  MessageContent,
  MessageFooter,
  MessageHeader,
} from "@/components/ui/message";
import { useT } from "@/lib/i18n";
import { extractContent, lookupToolResult, type ToolResultInfo } from "./content-extract";
import { MultimodalParts } from "./multimodal-parts";
import { ReasoningBlock } from "./reasoning-block";
import { ToolCallCard } from "./tool-call-card";
import { MarkdownLite } from "./markdown-lite";

function PartRefusal({ text }: { text: string }) {
  return (
    <Alert variant="destructive" className="my-2">
      <ShieldAlert />
      <AlertDescription className="whitespace-pre-wrap break-words">{text}</AlertDescription>
    </Alert>
  );
}

function modelIcon(model: string) {
  return <ProviderIcon protocol={model} size={14} className="shrink-0" />;
}

interface AssistantMessageProps {
  message: MessageItem;
  index: number;
  toolResultsByID: Record<string, ToolResultInfo>;
}

export function AssistantMessage({ message, index, toolResultsByID }: AssistantMessageProps) {
  const t = useT();
  const { role, content, tool_calls, reasoning_content, refusal } = message.message;
  const { modelId } = message;
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
    <Message
      style={style}
      className="group/msg animate-in fade-in slide-in-from-bottom-1 duration-300"
    >
      <MessageAvatar className="size-8">
        {isAssistant ? (
          modelId ? (
            (modelIcon(modelId) ?? <Sparkles className="size-4" />)
          ) : (
            <Sparkles className="size-4" />
          )
        ) : (
          <ShieldAlert className="size-4" />
        )}
      </MessageAvatar>

      <MessageContent>
        {(modelId || time) && (
          <MessageHeader className="min-h-8 gap-2 px-0">
            {isAssistant && modelId && (
              <span className="font-mono text-[11px] font-medium tracking-tight text-foreground/80">
                {modelId}
              </span>
            )}
            {isAssistant && modelId && time && (
              <span aria-hidden className="text-muted-foreground/40">
                ·
              </span>
            )}
            {time && <span className="tabular-nums">{time}</span>}
          </MessageHeader>
        )}
        {reasoning_content && <ReasoningBlock text={reasoning_content} />}

        <Bubble variant="ghost">
          <BubbleContent>
            {parts.length > 0 && <MultimodalParts parts={parts} />}
            {text && <MarkdownLite text={text} />}
            {!text && parts.length === 0 && !tool_calls?.length && !refusal && (
              <span className="text-muted-foreground">—</span>
            )}
          </BubbleContent>
        </Bubble>

        {text && (
          <MessageFooter className="px-0 opacity-0 transition-opacity group-hover/msg:opacity-100">
            <CopyButton value={text} variant="icon" ariaLabel={t("chat_message.copy_aria")} />
          </MessageFooter>
        )}

        {refusal && <PartRefusal text={refusal} />}

        {tool_calls && tool_calls.length > 0 && (
          <div>
            {tool_calls.map((call: UnifiedToolCall, i: number) => (
              <ToolCallCard
                key={call.id ?? i}
                call={call}
                result={call.id ? lookupToolResult(toolResultsByID, call.id) : undefined}
              />
            ))}
          </div>
        )}
      </MessageContent>
    </Message>
  );
}
