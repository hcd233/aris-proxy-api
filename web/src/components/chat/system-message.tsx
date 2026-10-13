import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { MarkdownLite } from "./markdown-lite";
import { CopyButton } from "@/components/ui/copy-button";
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
    <div style={style} className="group/msg animate-in fade-in slide-in-from-bottom-1 duration-300">
      <div className="mb-1.5 flex items-center gap-2 text-[11px] text-muted-foreground/70">
        <span className="font-medium uppercase tracking-[0.14em]">
          {t("system_message.system")}
        </span>
        {time && (
          <>
            <span className="text-muted-foreground/30">·</span>
            <span>{time}</span>
          </>
        )}
        <CopyButton
          value={trimmed}
          variant="icon"
          ariaLabel={t("chat_message.copy_aria")}
          className="opacity-0 transition-opacity group-hover/msg:opacity-100"
        />
      </div>
      <Collapsible
        open={open}
        onOpenChange={setOpen}
        className="rounded-xl border border-dashed border-border bg-muted/40 px-4 py-3 text-[13.5px] leading-relaxed text-muted-foreground"
      >
        <MarkdownLite text={display} />
        {isLong && (
          <CollapsibleTrigger className="mt-2 inline-flex items-center gap-1 font-medium text-primary/90 transition-colors hover:text-primary">
            {open ? t("system_message.show_less") : t("system_message.show_more")}
            <ChevronRight className="size-3 in-data-panel-open:rotate-90" />
          </CollapsibleTrigger>
        )}
      </Collapsible>
    </div>
  );
}
