"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import { Check, Copy, Terminal, X } from "lucide-react";

import { useT } from "@/lib/i18n";

hljs.registerLanguage("bash", bash);

const CODE_SYNTAX =
  "[&_.hljs-comment]:text-[#8C857B] [&_.hljs-comment]:italic " +
  "[&_.hljs-keyword]:text-[#C77B5A] [&_.hljs-built_in]:text-[#7DA1C4] " +
  "[&_.hljs-string]:text-[#9CB071] [&_.hljs-number]:text-[#D69A6B] " +
  "[&_.hljs-literal]:text-[#D69A6B] [&_.hljs-attr]:text-[#7DA1C4] " +
  "[&_.hljs-title]:text-[#7DA1C4] [&_.hljs-params]:text-[#E5E0D6] " +
  "[&_.hljs-variable]:text-[#D69A6B] [&_.hljs-operator]:text-[#9FB3C2] " +
  "[&_.hljs-punctuation]:text-[#A8A296] [&_.hljs-property]:text-[#7DA1C4]";

interface TraceInstallInlineProps {
  open: boolean;
  onClose: () => void;
}

function generateInstallCommand(hostValue: string): string {
  const host = hostValue.replace(/\/$/, "");
  return `curl -fsSL ${host}/install.sh | sh`;
}

export default function TraceInstallInline({
  open,
  onClose,
}: TraceInstallInlineProps) {
  const t = useT();
  const [host] = useState(() =>
    typeof window === "undefined" ? "" : window.location.origin,
  );
  const [copied, setCopied] = useState(false);
  const preRef = useRef<HTMLPreElement>(null);

  const previewCommand = useMemo(
    () => generateInstallCommand(host || "https://your-aris-server.example"),
    [host],
  );
  const highlighted = useMemo(
    () => hljs.highlight(previewCommand, { language: "bash" }).value,
    [previewCommand],
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

  if (!open) return null;

  return (
    <div
      className="group relative overflow-hidden rounded-xl border border-border/60 bg-gradient-to-b from-[#1A1D23] to-[#111318] shadow-lg shadow-black/30 animate-in fade-in slide-in-from-top-2 duration-300"
    >
      {/* Dots decoration */}
      <div className="pointer-events-none absolute inset-0 select-none">
        <div className="absolute left-4 top-3 flex items-center gap-[5px]">
          <span className="size-[7px] rounded-full bg-[#FF5F57]" />
          <span className="size-[7px] rounded-full bg-[#FEBC2E]" />
          <span className="size-[7px] rounded-full bg-[#28C840]" />
        </div>
      </div>

      {/* Top bar */}
      <div className="flex items-center justify-end gap-2 border-b border-white/[0.06] px-4 py-2.5 pl-[68px]">
        <span className="mr-auto truncate font-mono text-[11px] font-medium tracking-wide text-white/35">
          ~/install.sh
        </span>
        <button
          type="button"
          onClick={onClose}
          className="inline-flex size-6 items-center justify-center rounded-md text-white/25 transition-colors hover:bg-white/[0.06] hover:text-white/60 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-white/30"
          aria-label={t("trace.install_close")}
        >
          <X className="size-3.5" />
        </button>
      </div>

      {/* Terminal body */}
      <div className="relative px-5 py-[18px]">
        {/* Prompt sign */}
        <div className="absolute left-4 top-[22px] select-none font-mono text-[12.5px] leading-[1.65] text-white/20">
          $
        </div>

        <div className="flex items-start gap-2 pl-[14px]">
          <pre
            ref={preRef}
            className="min-w-0 flex-1 overflow-x-auto whitespace-pre text-[12.5px] leading-[1.65]"
          >
            <code
              className={`block font-mono whitespace-pre ${CODE_SYNTAX}`}
              dangerouslySetInnerHTML={{ __html: highlighted }}
            />
          </pre>

          {/* Copy button — on the right, docked to the code */}
          <button
            type="button"
            onClick={handleCopy}
            disabled={!host}
            className="mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-white/[0.08] bg-white/[0.04] text-white/35 transition-all hover:border-white/[0.15] hover:bg-white/[0.08] hover:text-white/80 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-white/40 disabled:pointer-events-none disabled:opacity-30"
            aria-label={copied ? t("trace.install_copied") : t("trace.install_copy")}
          >
            {copied ? (
              <Check className="size-3.5 text-[#28C840]" />
            ) : (
              <Copy className="size-3.5" />
            )}
          </button>
        </div>

        {/* Blinking cursor */}
        <span className="ml-[14px] mt-1 inline-block size-[7px] bg-[#28C840]/80 animate-pulse" />
      </div>
    </div>
  );
}
