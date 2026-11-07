import { MarkdownLite } from "./markdown-lite";
import { FoldedContent, SYSTEM_FOLD_LINES } from "./folded-content";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { CopyButton } from "@/components/ui/copy-button";
import { Marker, MarkerContent } from "@/components/ui/marker";
import { useT } from "@/lib/i18n";

interface SystemMessageProps {
  text: string;
  time?: string;
  index: number;
}

export function SystemMessage({ text, time, index }: SystemMessageProps) {
  const t = useT();
  const trimmed = text.trim();

  const style = { animationDelay: `${Math.min(index, 12) * 40}ms` };

  return (
    <div
      style={style}
      className="group/msg animate-in fade-in slide-in-from-bottom-1 flex flex-col gap-2 duration-300"
    >
      <div className="flex items-center gap-2">
        <Marker variant="separator" className="text-xs">
          <MarkerContent>
            {t("system_message.system")}
            {time && ` · ${time}`}
          </MarkerContent>
        </Marker>
        <CopyButton
          value={trimmed}
          variant="icon"
          ariaLabel={t("chat_message.copy_aria")}
          className="opacity-0 transition-opacity group-hover/msg:opacity-100 pointer-coarse:opacity-100"
        />
      </div>
      <Bubble variant="muted" className="max-w-full">
        <BubbleContent>
          <FoldedContent lines={SYSTEM_FOLD_LINES}>
            <MarkdownLite text={trimmed} />
          </FoldedContent>
        </BubbleContent>
      </Bubble>
    </div>
  );
}
