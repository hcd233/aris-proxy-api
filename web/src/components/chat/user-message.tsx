import type { MessageItem } from "@/lib/types";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { CopyButton } from "@/components/ui/copy-button";
import { Message, MessageContent, MessageFooter } from "@/components/ui/message";
import { useT } from "@/lib/i18n";
import { extractContent } from "./content-extract";
import { MultimodalParts } from "./multimodal-parts";
import { MarkdownLite } from "./markdown-lite";

interface UserMessageProps {
  message: MessageItem;
  index: number;
}

export function UserMessage({ message, index }: UserMessageProps) {
  const t = useT();
  const { content } = message.message;
  const { text, parts } = extractContent(content);

  const style = { animationDelay: `${Math.min(index, 12) * 40}ms` };

  return (
    <Message
      align="end"
      style={style}
      className="group/msg animate-in fade-in slide-in-from-bottom-1 duration-300"
    >
      <MessageContent>
        <Bubble variant="secondary" align="end">
          <BubbleContent>
            {parts.length > 0 && <MultimodalParts parts={parts} />}
            {text && <MarkdownLite text={text} raw />}
            {!text && parts.length === 0 && <span className="text-muted-foreground">—</span>}
          </BubbleContent>
        </Bubble>
        {text && (
          <MessageFooter className="opacity-0 transition-opacity group-hover/msg:opacity-100">
            <CopyButton value={text} variant="icon" ariaLabel={t("chat_message.copy_aria")} />
          </MessageFooter>
        )}
      </MessageContent>
    </Message>
  );
}
