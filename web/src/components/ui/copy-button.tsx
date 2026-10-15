"use client";

/**
 * CopyButton — shared "copy to clipboard" affordance built on the shadcn Button.
 *
 * Two variants:
 *  - "label": icon + copy/copied text (ghost xs), for code-block-style headers.
 *  - "icon": icon-only (ghost icon-xs), for hover-reveal actions on chat bubbles.
 */

import { useState, type MouseEvent } from "react";
import { Check, Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { copyTextToClipboard } from "@/lib/clipboard";
import { useT } from "@/lib/i18n";

interface CopyButtonProps {
  /** Text copied to the clipboard when clicked. */
  value: string;
  ariaLabel?: string;
  variant?: "label" | "icon";
  className?: string;
}

export function CopyButton({ value, ariaLabel, variant = "label", className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);
  const t = useT();

  const onCopy = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation();
    if (!value) return;
    void copyTextToClipboard(value).then((ok) => {
      if (ok) {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1400);
      } else {
        toast.error(t("common.copy_failed"));
      }
    });
  };

  const Icon = copied ? Check : Copy;

  return (
    <Button
      type="button"
      variant="ghost"
      size={variant === "label" ? "xs" : "icon-xs"}
      onClick={onCopy}
      disabled={!value}
      aria-label={ariaLabel ?? t("markdown.copy")}
      // min-w-0：抵消 xs 尺寸的 i18n 宽度预留，复制按钮跟随内容宽度
      className={cn("min-w-0", className)}
    >
      {variant === "label" ? (
        <>
          <Icon data-icon="inline-start" />
          {copied ? t("markdown.copied") : t("markdown.copy")}
        </>
      ) : (
        <Icon />
      )}
    </Button>
  );
}
