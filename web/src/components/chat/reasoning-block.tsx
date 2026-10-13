import { Brain, ChevronRight } from "lucide-react";
import { useT } from "@/lib/i18n";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";

export function ReasoningBlock({ text }: { text: string }) {
  const t = useT();
  if (!text.trim()) return null;

  return (
    <Collapsible className="mb-3">
      <CollapsibleTrigger className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-[12px] text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground">
        <Brain className="size-3.5 text-primary/70" />
        <span className="font-medium tracking-wide">{t("chat.thought_process")}</span>
        <ChevronRight className="size-3 opacity-60 in-data-panel-open:rotate-90" />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="mt-2 rounded-r-md border-l-2 border-border pl-4 pr-2 py-2">
          <p className="whitespace-pre-wrap break-words text-[13.5px] italic leading-[1.55] text-muted-foreground">
            {text}
          </p>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
