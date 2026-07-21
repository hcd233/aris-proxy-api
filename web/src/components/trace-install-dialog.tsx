"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import { Codex } from "@lobehub/icons";
import { Check, Copy, ShieldCheck, Terminal, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useT } from "@/lib/i18n";

interface TraceInstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function generateInstallCommand(hostValue: string): string {
  const host = hostValue.replace(/\/$/, "");
  return `curl -fsSL ${host}/install.sh | sh`;
}

hljs.registerLanguage("bash", bash);

const CODE_SYNTAX =
  "[&_.hljs-comment]:text-[#8C857B] [&_.hljs-comment]:italic " +
  "[&_.hljs-keyword]:text-[#C77B5A] [&_.hljs-built_in]:text-[#7DA1C4] " +
  "[&_.hljs-string]:text-[#9CB071] [&_.hljs-number]:text-[#D69A6B] " +
  "[&_.hljs-literal]:text-[#D69A6B] [&_.hljs-attr]:text-[#7DA1C4] " +
  "[&_.hljs-title]:text-[#7DA1C4] [&_.hljs-params]:text-[#E5E0D6] " +
  "[&_.hljs-variable]:text-[#D69A6B] [&_.hljs-operator]:text-[#9FB3C2] " +
  "[&_.hljs-punctuation]:text-[#A8A296] [&_.hljs-property]:text-[#7DA1C4]";

export default function TraceInstallDialog({
  open,
  onOpenChange,
}: TraceInstallDialogProps) {
  const t = useT();
  const closeBtnRef = useRef<HTMLButtonElement>(null);
  const [host] = useState(() =>
    typeof window === "undefined" ? "" : window.location.origin
  );
  const [copied, setCopied] = useState(false);

  const previewCommand = useMemo(
    () => generateInstallCommand(host || "https://your-aris-server.example"),
    [host]
  );
  const highlighted = useMemo(
    () => hljs.highlight(previewCommand, { language: "bash" }).value,
    [previewCommand]
  );

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(generateInstallCommand(host));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* noop */
    }
  }, [host]);

  const handleClose = useCallback(() => {
    setCopied(false);
    onOpenChange(false);
  }, [onOpenChange]);

  useEffect(() => {
    if (open && closeBtnRef.current) {
      setTimeout(() => closeBtnRef.current?.focus(), 120);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="!max-w-[1040px] w-[calc(100vw-1.5rem)] h-[min(86vh,720px)] p-0 gap-0 overflow-hidden flex flex-col sm:!max-w-[1040px]"
      >
        <DialogHeader className="shrink-0 flex-row items-center gap-3 border-b border-border px-6 py-4">
          <span className="flex size-9 items-center justify-center rounded-xl border border-border bg-gradient-to-b from-secondary to-muted shadow-sm">
            <Codex.Color className="size-4.5" />
          </span>
          <div className="flex min-w-0 flex-col gap-0.5">
            <DialogTitle className="font-display text-base leading-tight">
              {t("trace.install_title")}
            </DialogTitle>
            <DialogDescription className="min-h-[2.5rem] text-xs leading-snug">
              {t("trace.install_desc")}
            </DialogDescription>
          </div>
          <Button
            ref={closeBtnRef}
            variant="ghost"
            size="icon-sm"
            onClick={handleClose}
            className="ml-auto shrink-0 text-muted-foreground"
            aria-label={t("trace.install_close")}
          >
            <X className="size-4" />
          </Button>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto md:grid md:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)] md:overflow-hidden">
          <div className="space-y-5 border-border px-6 py-5 md:min-h-0 md:overflow-y-auto md:border-r">
            <p className="text-sm leading-relaxed text-muted-foreground">
              {t("trace.install_terminal_hint")}
            </p>
            <ol className="space-y-3">
              {[
                [Terminal, "trace.install_step_download"],
                [ShieldCheck, "trace.install_step_approve"],
              ].map(([Icon, key], index) => (
                <li key={key as string} className="flex gap-3 rounded-xl border border-border bg-secondary/35 p-3.5">
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-background text-muted-foreground shadow-sm">
                    <Icon className="size-4" />
                  </span>
                  <div className="min-w-0">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/70">
                      0{index + 1}
                    </p>
                    <p className="mt-1 text-sm leading-relaxed">{t(key as string)}</p>
                  </div>
                </li>
              ))}
            </ol>
          </div>

          <div className="flex flex-col bg-[#262624] md:min-h-0 md:overflow-hidden">
            <div className="flex shrink-0 items-center justify-between gap-3 border-b border-white/[0.07] bg-[#30302E] px-4 py-2.5">
              <div className="flex min-w-0 items-center gap-2">
                <Terminal className="size-3.5 shrink-0 text-white/35" />
                <span className="truncate font-mono text-xs text-white/50">
                  {t("trace.install_script_filename")}
                </span>
              </div>
              <div className="flex shrink-0 items-center gap-3">
                <button
                  type="button"
                  onClick={handleCopy}
                  disabled={!host}
                  className="inline-flex h-9 min-w-20 items-center justify-center gap-1.5 rounded-md px-3 text-[11px] font-medium text-white/60 transition-colors hover:bg-white/[0.08] hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40 disabled:pointer-events-none disabled:opacity-35"
                >
                  {copied ? (
                    <Check className="size-3.5 text-[#9CB071]" />
                  ) : (
                    <Copy className="size-3.5" />
                  )}
                  {copied ? t("trace.install_copied") : t("trace.install_copy")}
                </button>
              </div>
            </div>
            <div className="min-h-[280px] flex-1 overflow-auto md:min-h-0">
              <pre className="px-5 py-4 text-[12.5px] leading-[1.65] text-[#E5E0D6]">
                <code
                  className={`block font-mono whitespace-pre ${CODE_SYNTAX}`}
                  dangerouslySetInnerHTML={{ __html: highlighted }}
                />
              </pre>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
