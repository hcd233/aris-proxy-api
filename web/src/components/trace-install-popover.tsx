"use client";

import { useCallback, useMemo, useState } from "react";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import { Check, Copy, Radar } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useT } from "@/lib/i18n";
import { copyTextToClipboard } from "@/lib/clipboard";

hljs.registerLanguage("bash", bash);

const CODE_SYNTAX =
  "[&_.hljs-comment]:text-[#8C857B] [&_.hljs-comment]:italic " +
  "[&_.hljs-keyword]:text-[#C77B5A] [&_.hljs-built_in]:text-[#7DA1C4] " +
  "[&_.hljs-string]:text-[#9CB071] [&_.hljs-number]:text-[#D69A6B] " +
  "[&_.hljs-literal]:text-[#D69A6B] [&_.hljs-attr]:text-[#7DA1C4] " +
  "[&_.hljs-title]:text-[#7DA1C4] [&_.hljs-params]:text-[#E5E0D6] " +
  "[&_.hljs-variable]:text-[#D69A6B] [&_.hljs-operator]:text-[#9FB3C2] " +
  "[&_.hljs-punctuation]:text-[#A8A296] [&_.hljs-property]:text-[#7DA1C4]";

function generateInstallCommand(hostValue: string): string {
  const host = hostValue.replace(/\/$/, "");
  return `curl -fsSL ${host}/install.sh | sh`;
}

export default function TraceInstallPopover() {
  const t = useT();
  const [host] = useState(() => (typeof window === "undefined" ? "" : window.location.origin));
  const [copied, setCopied] = useState(false);

  const previewCommand = useMemo(
    () => generateInstallCommand(host || "https://your-aris-server.example"),
    [host],
  );
  const highlighted = useMemo(
    () => hljs.highlight(previewCommand, { language: "bash" }).value,
    [previewCommand],
  );

  const handleCopy = useCallback(async () => {
    await copyTextToClipboard(generateInstallCommand(host));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [host]);

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button variant="outline" className="gap-1.5">
            <Radar className="size-4" />
            {t("trace.install")}
          </Button>
        }
      />
      <PopoverContent
        side="bottom"
        align="end"
        sideOffset={8}
        className="w-[480px] max-w-[calc(100vw-1.5rem)] overflow-hidden rounded-xl bg-(--code-bg) p-0 text-(--code-text) ring-1 ring-(--code-border)"
      >
        {/* Terminal dots */}
        <div className="flex items-center gap-[5px] px-4 pt-3">
          <span className="size-[7px] rounded-full bg-[#FF5F57]" />
          <span className="size-[7px] rounded-full bg-[#FEBC2E]" />
          <span className="size-[7px] rounded-full bg-[#28C840]" />
          {/* 装饰性终端标题，恒定短文案不会截断，无需 truncate/tooltip */}
          <span className="ml-2 font-mono text-[11px] font-medium tracking-wide text-(--code-text)/35">
            ~/install.sh
          </span>
        </div>

        {/* Command body */}
        <div className="relative px-4 pb-3 pt-2.5">
          <div className="flex items-start gap-2">
            <span className="mt-px select-none font-mono text-[12.5px] leading-[1.65] text-(--code-text)/20">
              $
            </span>
            <pre className="min-w-0 flex-1 overflow-x-auto whitespace-pre text-[12.5px] leading-[1.65]">
              <code
                className={`block font-mono whitespace-pre text-(--code-text) ${CODE_SYNTAX}`}
                dangerouslySetInnerHTML={{ __html: highlighted }}
              />
            </pre>
            <button
              type="button"
              onClick={handleCopy}
              disabled={!host}
              className="mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-(--code-border) bg-white/[0.04] text-(--code-text)/35 transition-all hover:border-(--code-text)/25 hover:bg-white/[0.08] hover:text-(--code-text)/80 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-(--code-text)/40 disabled:pointer-events-none disabled:opacity-30"
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
          <span className="ml-[18px] mt-0.5 inline-block size-[7px] bg-[#28C840]/80 animate-pulse" />
        </div>
      </PopoverContent>
    </Popover>
  );
}
