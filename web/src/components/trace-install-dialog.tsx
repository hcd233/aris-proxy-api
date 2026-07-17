"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useT } from "@/lib/i18n";
import { Check, Copy, Terminal, X } from "lucide-react";
import { Codex } from "@lobehub/icons";

interface TraceInstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function generateScript(traceUrl: string, apiKey: string): string {
  const hookUrl = `${traceUrl.replace(/\/$/, "")}/trace/event`;
  return `#!/usr/bin/env bash
# Install aris-proxy-api trace hooks for Codex
set -euo pipefail

HOOKS_DIR="$HOME/.aris/trace"
mkdir -p "$HOOKS_DIR"

# Hook is written with TRACE_URL and API_KEY baked in,
# because Codex invokes the hook in a fresh subprocess that does not inherit these vars.
cat > "$HOOKS_DIR/codex-hook.sh" <<'HOOKEOF'
#!/usr/bin/env bash
set -u
TRACE_URL="${hookUrl}"
API_KEY="${apiKey}"
LOG_DIR="\${LOG_DIR:-$HOME/.aris/trace/logs}"
LOG_FILE="$LOG_DIR/trace-$(date +%Y-%m-%d).log"
payload="$(cat)"
event_name="$(printf '%s' "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null)"
if [ "$event_name" = "Stop" ]; then printf '{}'; fi
(
  mkdir -p "$LOG_DIR" 2>/dev/null
  printf '%s' "$payload" | jq -c --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" '. + {_trace_local_ts: $ts}' >> "$LOG_FILE" 2>/dev/null
  find "$LOG_DIR" -name 'trace-*.log' -mtime +7 -delete 2>/dev/null
) >/dev/null 2>&1 &
if [ -n "$API_KEY" ]; then
  printf '%s' "$payload" | curl -sS --connect-timeout 2 --max-time 5 -X POST "$TRACE_URL" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $API_KEY" \
    -d @- >/dev/null 2>&1 || true
fi
exit 0
HOOKEOF
chmod +x "$HOOKS_DIR/codex-hook.sh"

HOOK_CMD="$HOOKS_DIR/codex-hook.sh"
python3 - "$HOOK_CMD" <<'PYEOF'
import json, os, sys
hook_cmd = sys.argv[1]
hooks_path = os.path.expanduser('~/.codex/hooks.json')
events = ["SessionStart","UserPromptSubmit","PreToolUse","PermissionRequest","PostToolUse","Stop","SubagentStart","SubagentStop","PreCompact","PostCompact"]
cfg = {}
if os.path.exists(hooks_path):
    with open(hooks_path) as f:
        try: cfg = json.load(f)
        except Exception: cfg = {}
hooks = cfg.setdefault("hooks", {})
for ev in events:
    grp = {"matcher": "", "hooks": [{"type": "command", "command": hook_cmd, "timeout": 30}]}
    # Idempotent: drop any existing group that points at this same hook command.
    existing = [g for g in hooks.setdefault(ev, []) if not (
        len(g.get("hooks", [])) == 1 and g["hooks"][0].get("command") == hook_cmd)]
    existing.append(grp)
    hooks[ev] = existing
os.makedirs(os.path.dirname(hooks_path), exist_ok=True)
with open(hooks_path, "w") as f:
    json.dump(cfg, f, indent=2)
print(f"Codex trace hooks installed to {hooks_path}")
PYEOF
echo "Done. In Codex, run /hooks and trust the new hook before first use."
`;
}

hljs.registerLanguage("bash", bash);

function highlightBash(code: string): string {
  return hljs.highlight(code, { language: "bash" }).value;
}

const CLAUDE_SYNTAX =
  "[&_.hljs-comment]:text-[#8C857B] [&_.hljs-comment]:italic " +
  "[&_.hljs-keyword]:text-[#C77B5A] " +
  "[&_.hljs-built_in]:text-[#7DA1C4] " +
  "[&_.hljs-string]:text-[#9CB071] " +
  "[&_.hljs-number]:text-[#D69A6B] [&_.hljs-literal]:text-[#D69A6B] " +
  "[&_.hljs-attr]:text-[#7DA1C4] [&_.hljs-title]:text-[#7DA1C4] " +
  "[&_.hljs-params]:text-[#E5E0D6] [&_.hljs-variable]:text-[#D69A6B] " +
  "[&_.hljs-operator]:text-[#9FB3C2] [&_.hljs-punctuation]:text-[#A8A296] " +
  "[&_.hljs-property]:text-[#7DA1C4] [&_.hljs-meta]:text-[#B98BC9] " +
  "[&_.hljs-section]:text-[#7DA1C4] [&_.hljs-selector-tag]:text-[#C77B5A] " +
  "[&_.hljs-type]:text-[#D6B86B]";

export default function TraceInstallDialog({
  open,
  onOpenChange,
}: TraceInstallDialogProps) {
  const t = useT();
  const closeBtnRef = useRef<HTMLButtonElement>(null);

  const [traceUrl, setTraceUrl] = useState(() =>
    typeof window === "undefined" ? "" : `${window.location.origin}/api/v1`
  );
  const [apiKey, setApiKey] = useState("YOUR_API_KEY");
  const [copied, setCopied] = useState(false);

  const script = useMemo(
    () => generateScript(traceUrl, apiKey),
    [traceUrl, apiKey]
  );

  const highlighted = useMemo(
    () => (script ? highlightBash(script) : ""),
    [script]
  );

  const lineCount = useMemo(
    () => (script ? script.split("\n").length : 0),
    [script]
  );

  const handleCopy = useCallback(async () => {
    if (!script) return;
    await navigator.clipboard.writeText(script);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [script]);

  const handleClose = useCallback(() => {
    setCopied(false);
    onOpenChange(false);
  }, [onOpenChange]);

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) setCopied(false);
      onOpenChange(nextOpen);
    },
    [onOpenChange]
  );

  useEffect(() => {
    if (open && closeBtnRef.current) {
      setTimeout(() => closeBtnRef.current?.focus(), 120);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="!max-w-[1040px] w-[calc(100vw-1.5rem)] h-[min(86vh,720px)] p-0 gap-0 overflow-hidden flex flex-col sm:!max-w-[1040px]"
      >
        <DialogHeader className="shrink-0 flex-row items-center gap-3 px-6 py-4 border-b border-border">
          <span className="flex size-9 items-center justify-center rounded-xl border border-border bg-gradient-to-b from-secondary to-muted shadow-sm">
            <Codex.Color className="size-4.5" />
          </span>
          <div className="flex flex-col gap-0.5 min-w-0">
            <DialogTitle className="font-display text-base leading-tight">
              {t("trace.install_title")}
            </DialogTitle>
            <DialogDescription className="text-xs leading-snug">
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

        <div className="flex flex-col flex-1 min-h-0 overflow-y-auto md:overflow-hidden md:grid md:grid-cols-[minmax(0,1fr)_minmax(0,1.12fr)]">
          <div className="md:min-h-0 md:overflow-y-auto md:border-r border-border px-6 py-5 space-y-6">
            <section className="space-y-4">
              <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("trace.install_connection")}
              </h3>

              <div className="space-y-1.5">
                <Label htmlFor="trace-base-url" className="text-xs font-medium text-foreground/80">
                  {t("trace.install_base_url")}
                </Label>
                <Input
                  id="trace-base-url"
                  placeholder={t("trace.install_base_url_placeholder")}
                  value={traceUrl}
                  onChange={(e) => setTraceUrl(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="trace-api-key" className="text-xs font-medium text-foreground/80">
                  {t("trace.install_api_key")}
                </Label>
                <Input
                  id="trace-api-key"
                  placeholder={t("trace.install_api_key_placeholder")}
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>
            </section>
          </div>

          <div className="flex flex-col md:min-h-0 md:overflow-hidden bg-[#262624]">
            <div className="shrink-0 flex items-center justify-between gap-3 border-b border-white/[0.07] bg-[#30302E] px-4 py-2.5">
              <div className="flex items-center gap-2 min-w-0">
                <Terminal className="size-3.5 shrink-0 text-white/35" />
                <span className="truncate font-mono text-xs text-white/50">
                  {t("trace.install_script_filename")}
                </span>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                <span className="hidden font-mono text-[10px] tabular-nums text-white/25 sm:inline">
                  {lineCount} {t("trace.install_lines")}
                </span>
                <button
                  type="button"
                  onClick={handleCopy}
                  disabled={!script}
                  className="inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-[11px] font-medium text-white/55 transition-colors hover:bg-white/[0.08] hover:text-white disabled:pointer-events-none disabled:opacity-30"
                >
                  {copied ? (
                    <>
                      <Check className="size-3.5 text-[#9CB071]" />
                      {t("trace.install_copied")}
                    </>
                  ) : (
                    <>
                      <Copy className="size-3.5" />
                      {t("trace.install_copy")}
                    </>
                  )}
                </button>
              </div>
            </div>

            <div className="flex-1 md:min-h-0 overflow-auto">
              <pre className="px-5 py-4 text-[12.5px] leading-[1.65] text-[#E5E0D6]">
                <code
                  className={`block font-mono whitespace-pre ${CLAUDE_SYNTAX}`}
                  dangerouslySetInnerHTML={{ __html: highlighted }}
                />
              </pre>
            </div>

            <div className="shrink-0 border-t border-white/[0.07] bg-[#30302E] px-4 py-2">
              <p className="font-mono text-[10.5px] leading-relaxed text-white/30">
                {t("trace.install_footer")}
              </p>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
