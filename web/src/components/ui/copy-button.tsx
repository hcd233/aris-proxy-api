"use client";

/**
 * CopyButton — shared "copy to clipboard" affordance.
 *
 * Two visual variants:
 *  - "label": icon + copy/copied text, for code-block-style headers
 *    (e.g. tool input/output blocks).
 *  - "icon": icon-only, compact, for hover-reveal actions on chat bubbles.
 */

import { useState, type MouseEvent } from "react";
import { Check, Copy } from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";

interface CopyButtonProps {
  /** Text copied to the clipboard when clicked. */
  value: string;
  ariaLabel?: string;
  variant?: "label" | "icon";
  className?: string;
}

export function CopyButton({
  value,
  ariaLabel,
  variant = "label",
  className,
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false);
  const t = useT();

  const onCopy = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation();
    if (!value) return;
    // 非安全上下文（纯 HTTP 非 localhost）下 clipboard API 不存在
    if (!navigator.clipboard) {
      toast.error(t("common.copy_failed"));
      return;
    }
    void navigator.clipboard.writeText(value).then(
      () => {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1400);
      },
      () => toast.error(t("common.copy_failed")),
    );
  };

  return (
    <button
      type="button"
      onClick={onCopy}
      disabled={!value}
      aria-label={ariaLabel ?? t("markdown.copy")}
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded font-mono text-[10px] text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:pointer-events-none disabled:opacity-30",
        variant === "label" ? "px-1.5 py-0.5" : "size-6 justify-center p-0",
        className,
      )}
    >
      {copied ? (
        <Check className="size-3" />
      ) : (
        <Copy className="size-3" />
      )}
      {variant === "label" && (copied ? t("markdown.copied") : t("markdown.copy"))}
    </button>
  );
}
