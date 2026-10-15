import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { MarkdownLite } from "./markdown-lite";
import { Bubble, BubbleContent } from "@/components/ui/bubble";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Marker, MarkerContent } from "@/components/ui/marker";
import { useT } from "@/lib/i18n";
import { Collapsible, CollapsibleTrigger } from "@/components/ui/collapsible";

const SYSTEM_MSG_PREVIEW_CHARS = 200;

interface SystemMessageProps {
  text: string;
  time?: string;
  index: number;
}

export function SystemMessage({ text, time, index }: SystemMessageProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const trimmed = text.trim();
  const isLong = trimmed.length > SYSTEM_MSG_PREVIEW_CHARS;
  const display =
    !isLong || open ? trimmed : `${trimmed.slice(0, SYSTEM_MSG_PREVIEW_CHARS).trimEnd()}…`;

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
          className="opacity-0 transition-opacity group-hover/msg:opacity-100"
        />
      </div>
      <Bubble variant="muted" className="max-w-full">
        <BubbleContent>
          <Collapsible open={open} onOpenChange={setOpen}>
            <MarkdownLite text={display} />
            {isLong && (
              <CollapsibleTrigger
                render={<Button variant="link" size="xs" className="mt-1 h-auto min-w-0 px-0" />}
              >
                {open ? t("system_message.show_less") : t("system_message.show_more")}
                <ChevronRight data-icon="inline-end" className="in-data-panel-open:rotate-90" />
              </CollapsibleTrigger>
            )}
          </Collapsible>
        </BubbleContent>
      </Bubble>
    </div>
  );
}
