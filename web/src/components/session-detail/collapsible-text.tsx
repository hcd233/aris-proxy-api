"use client";

import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { useT } from "@/lib/i18n";
import { Collapsible, CollapsibleTrigger } from "@/components/ui/collapsible";

export function CollapsibleText({
  text,
  previewChars = 140,
  className,
}: {
  text: string;
  previewChars?: number;
  className?: string;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const trimmed = text.trim();
  const isLong = trimmed.length > previewChars;
  const display = !isLong || open ? trimmed : `${trimmed.slice(0, previewChars).trimEnd()}…`;

  return (
    <Collapsible open={open} onOpenChange={setOpen} className={className}>
      <p className="whitespace-pre-wrap break-words">{display}</p>
      {isLong && (
        <CollapsibleTrigger
          onClick={(e) => e.stopPropagation()}
          className="mt-1 inline-flex items-center gap-0.5 font-medium text-primary transition-colors hover:text-primary"
        >
          {open ? t("collapsible.show_less") : t("collapsible.show_more")}
          <ChevronRight className="size-3 in-data-panel-open:rotate-90" />
        </CollapsibleTrigger>
      )}
    </Collapsible>
  );
}
