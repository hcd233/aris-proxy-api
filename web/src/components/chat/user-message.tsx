import type { MessageItem } from "@/lib/types";
import { CopyButton } from "@/components/ui/copy-button";
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
    <div
      style={style}
      className="group/msg animate-in fade-in slide-in-from-bottom-1 flex justify-end gap-1.5 duration-300"
    >
      {text && (
        <CopyButton
          value={text}
          variant="icon"
          ariaLabel={t("chat_message.copy_aria")}
          className="mt-1 self-start opacity-0 transition-opacity group-hover/msg:opacity-100"
        />
      )}
      <div className="w-full max-w-[85%] md:max-w-[80%]">
        <div className="rounded-[20px] rounded-br-[6px] bg-accent px-5 py-3.5 text-[15px] leading-[1.6]">
          {parts.length > 0 && <MultimodalParts parts={parts} />}
          {text && <MarkdownLite text={text} raw />}
          {!text && parts.length === 0 && (
            <span className="text-muted-foreground/60">—</span>
          )}
        </div>
      </div>
    </div>
  );
}
