import { useState } from "react";
import { Brain, ChevronDown, ChevronRight } from "lucide-react";

export function ReasoningBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  if (!text.trim()) return null;

  return (
    <div className="mb-3">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-[12px] text-muted-foreground transition-colors hover:bg-muted/40 hover:text-foreground"
      >
        <Brain className="size-3.5 text-primary/70" />
        <span className="font-medium tracking-wide">Thought process</span>
        {open ? (
          <ChevronDown className="size-3 opacity-60" />
        ) : (
          <ChevronRight className="size-3 opacity-60" />
        )}
      </button>
      {open && (
        <div className="mt-2 rounded-r-md border-l-2 border-border pl-4 pr-2 py-2">
          <p className="whitespace-pre-wrap break-words text-[13.5px] italic leading-[1.55] text-muted-foreground">
            {text}
          </p>
        </div>
      )}
    </div>
  );
}
