"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ModelItem } from "@/lib/types";
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
import { Check, Copy, Search, Terminal, X } from "lucide-react";
import { Codex } from "@lobehub/icons";

interface ExportCodexDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: ModelItem[];
}

function generateScript(
  providerId: string,
  baseUrl: string,
  apiKey: string,
  selectedModel: ModelItem | null
): string {
  if (!selectedModel) return "";

  const providerIdJson = JSON.stringify(providerId);
  const modelJson = JSON.stringify(selectedModel.alias);
  const baseUrlJson = JSON.stringify(baseUrl);
  const apiKeyJson = JSON.stringify(apiKey);
  const contextWindow = selectedModel.contextLength > 0 ? selectedModel.contextLength : 128000;

  return `#!/usr/bin/env bash
# Export model from aris-proxy-api to Codex
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────
# Edit these or set as environment variables before running
PROVIDER_ID="\${PROVIDER_ID:-${providerId}}"
MODEL="\${MODEL:-${selectedModel.alias}}"
BASE_URL="\${BASE_URL:-${baseUrl}}"
API_KEY="\${API_KEY:-${apiKey}}"
MODEL_CONTEXT_WINDOW="\${MODEL_CONTEXT_WINDOW:-${contextWindow}}"
CODEX_CONFIG="\${CODEX_CONFIG:-\$HOME/.codex/config.toml}"
# ───────────────────────────────────────────────────────────────

python3 << 'PYEOF'
import json, os, re

config_path = os.path.expanduser(os.environ.get(
    'CODEX_CONFIG',
    os.path.join(os.path.expanduser('~'), '.codex/config.toml')
))
provider_id = os.environ.get('PROVIDER_ID', ${providerIdJson})
model = os.environ.get('MODEL', ${modelJson})
base_url = os.environ.get('BASE_URL', ${baseUrlJson})
api_key = os.environ.get('API_KEY', ${apiKeyJson})
context_window = int(os.environ.get('MODEL_CONTEXT_WINDOW', '${contextWindow}'))
provider_id_toml = json.dumps(provider_id)

os.makedirs(os.path.dirname(config_path), exist_ok=True)

if os.path.exists(config_path):
    with open(config_path, 'r') as f:
        original = f.read()
    backup_path = config_path + '.bak'
    with open(backup_path, 'w') as f:
        f.write(original)
    print(f"Backup saved to {backup_path}")
else:
    original = ''
    print(f"{config_path} not found, creating a new one")

lines = original.splitlines()
cleaned = []
in_provider = False
provider_header = re.compile(
    r'^\\s*\\[\\s*model_providers\\s*\\.\\s*(?:' + re.escape(provider_id) + r'|' + re.escape(provider_id_toml) + r')\\s*\\]\\s*$'
)
table_header = re.compile(r'^\\s*\\[')

for line in lines:
    stripped = line.strip()
    if in_provider:
        if table_header.match(line):
            in_provider = False
        else:
            continue
    if provider_header.match(line):
        in_provider = True
        continue
    cleaned.append(line)

root_cleaned = []
in_table = False
for line in cleaned:
    if table_header.match(line):
        in_table = True
    if not in_table and re.match(r'^\\s*(model|model_provider|model_context_window)\\s*=', line):
        continue
    root_cleaned.append(line)

first_table = next((i for i, line in enumerate(root_cleaned) if table_header.match(line)), len(root_cleaned))

root_block = [
    f'model = {json.dumps(model)}',
    f'model_provider = {json.dumps(provider_id)}',
    f'model_context_window = {context_window}',
]
provider_block = [
    f'[model_providers.{provider_id_toml}]',
    'name = "Aris Proxy"',
    f'base_url = {json.dumps(base_url)}',
    'wire_api = "responses"',
    f'experimental_bearer_token = {json.dumps(api_key)}',
]

next_lines = root_cleaned[:first_table]
while next_lines and next_lines[-1].strip() == '':
    next_lines.pop()
next_lines.extend([''] if next_lines else [])
next_lines.extend(root_block)
next_lines.extend([''])
next_lines.extend(root_cleaned[first_table:])
while next_lines and next_lines[-1].strip() == '':
    next_lines.pop()
next_lines.extend(['', *provider_block, ''])

with open(config_path, 'w') as f:
    f.write('\\n'.join(next_lines))

print(f"Codex configured: provider '{provider_id}' with model '{model}'")
PYEOF`;
}

hljs.registerLanguage("bash", bash);

function highlightBash(code: string): string {
  return hljs.highlight(code, { language: "bash" }).value;
}

function formatTokens(n: number): string {
  if (!n || n <= 0) return "—";
  if (n >= 1_000_000) {
    const v = n / 1_000_000;
    return `${Number.isInteger(v) ? v : v.toFixed(1)}M`;
  }
  if (n >= 1_000) {
    const v = n / 1_000;
    return `${Number.isInteger(v) ? v : v.toFixed(1)}K`;
  }
  return String(n);
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

export default function ExportCodexDialog({
  open,
  onOpenChange,
  models,
}: ExportCodexDialogProps) {
  const t = useT();
  const searchInputRef = useRef<HTMLInputElement>(null);

  const [providerId, setProviderId] = useState("aris-proxy");
  const [baseUrl, setBaseUrl] = useState(() =>
    typeof window === "undefined" ? "" : `${window.location.origin}/api/openai/v1`
  );
  const [apiKey, setApiKey] = useState("YOUR_API_KEY");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [copied, setCopied] = useState(false);
  const [modelSearch, setModelSearch] = useState("");

  const filteredModels = useMemo(
    () =>
      modelSearch.trim()
        ? models.filter(
            (m) =>
              m.alias.toLowerCase().includes(modelSearch.toLowerCase()) ||
              m.modelName.toLowerCase().includes(modelSearch.toLowerCase())
          )
        : models,
    [models, modelSearch]
  );

  const selectedModel = useMemo(
    () => models.find((m) => m.id === selectedId) ?? null,
    [models, selectedId]
  );

  const script = useMemo(
    () => generateScript(providerId, baseUrl, apiKey, selectedModel),
    [providerId, baseUrl, apiKey, selectedModel]
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
    setSelectedId(null);
    setModelSearch("");
    setCopied(false);
    onOpenChange(false);
  }, [onOpenChange]);

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        setSelectedId(null);
        setModelSearch("");
        setCopied(false);
      }
      onOpenChange(nextOpen);
    },
    [onOpenChange]
  );

  useEffect(() => {
    if (open && searchInputRef.current) {
      setTimeout(() => searchInputRef.current?.focus(), 120);
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
              {t("models.export_codex_title")}
            </DialogTitle>
            <DialogDescription className="text-xs leading-snug">
              {t("models.export_codex_desc")}
            </DialogDescription>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={handleClose}
            className="ml-auto shrink-0 text-muted-foreground"
            aria-label={t("share_dialog.close")}
          >
            <X className="size-4" />
          </Button>
        </DialogHeader>

        <div className="flex flex-col flex-1 min-h-0 overflow-y-auto md:overflow-hidden md:grid md:grid-cols-[minmax(0,1fr)_minmax(0,1.12fr)]">
          <div className="md:min-h-0 md:overflow-y-auto md:border-r border-border px-6 py-5 space-y-6">
            <section className="space-y-4">
              <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("models.export_connection")}
              </h3>

              <div className="space-y-1.5">
                <Label htmlFor="export-codex-provider-id" className="text-xs font-medium text-foreground/80">
                  {t("models.export_provider_id")}
                </Label>
                <Input
                  id="export-codex-provider-id"
                  placeholder={t("models.export_provider_id_placeholder")}
                  value={providerId}
                  onChange={(e) => setProviderId(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="export-codex-base-url" className="text-xs font-medium text-foreground/80">
                  {t("models.export_base_url")}
                </Label>
                <Input
                  id="export-codex-base-url"
                  placeholder={t("models.export_base_url_placeholder")}
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="export-codex-api-key" className="text-xs font-medium text-foreground/80">
                  {t("models.export_api_key")}
                </Label>
                <Input
                  id="export-codex-api-key"
                  placeholder={t("models.export_api_key_placeholder")}
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>
            </section>

            <section className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                  {t("models.export_default_model")}
                </h3>
                {selectedModel && (
                  <button
                    type="button"
                    onClick={() => setSelectedId(null)}
                    className="text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {t("models.export_tier_clear")}
                  </button>
                )}
              </div>

              <div className="relative">
                <Search className="absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground/60" />
                <Input
                  ref={searchInputRef}
                  placeholder={t("models.search_placeholder")}
                  value={modelSearch}
                  onChange={(e) => setModelSearch(e.target.value)}
                  className="h-8 pl-8 text-sm"
                />
              </div>

              <div className="max-h-[360px] space-y-1.5 overflow-y-auto pr-1">
                {filteredModels.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border px-3 py-8 text-center text-xs text-muted-foreground">
                    {t("models.export_no_matches")}
                  </p>
                ) : (
                  filteredModels.map((model) => {
                    const selected = selectedId === model.id;
                    return (
                      <button
                        key={model.id}
                        type="button"
                        onClick={() => setSelectedId(model.id)}
                        className={`group flex w-full items-center gap-2.5 rounded-xl border px-3 py-2.5 text-left transition-all ${
                          selected
                            ? "border-primary/50 bg-primary/[0.06] shadow-sm"
                            : "border-border hover:border-muted-foreground/30 hover:bg-secondary/50"
                        }`}
                      >
                        <span
                          className={`flex size-4 shrink-0 items-center justify-center rounded-[5px] border ${
                            selected
                              ? "border-primary bg-primary text-primary-foreground"
                              : "border-muted-foreground/30 group-hover:border-muted-foreground/50"
                          }`}
                        >
                          {selected && <Check className="size-3" strokeWidth={3} />}
                        </span>
                        <span className="flex min-w-0 flex-1 flex-col">
                          <span className="truncate text-sm font-medium text-foreground">
                            {model.alias}
                          </span>
                          <span className="truncate font-mono text-[11px] text-muted-foreground/70">
                            {model.modelName}
                          </span>
                        </span>
                        <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground/60">
                          {formatTokens(model.contextLength || 128000)}
                        </span>
                      </button>
                    );
                  })
                )}
              </div>
            </section>
          </div>

          <div className="flex flex-col md:min-h-0 md:overflow-hidden bg-[#262624]">
            <div className="shrink-0 flex items-center justify-between gap-3 border-b border-white/[0.07] bg-[#30302E] px-4 py-2.5">
              <div className="flex items-center gap-2 min-w-0">
                <Terminal className="size-3.5 shrink-0 text-white/35" />
                <span className="truncate font-mono text-xs text-white/50">
                  codex-setup.sh
                </span>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                {selectedModel && (
                  <span className="hidden font-mono text-[10px] tabular-nums text-white/25 sm:inline">
                    {lineCount} {t("models.export_lines")}
                  </span>
                )}
                <button
                  type="button"
                  onClick={handleCopy}
                  disabled={!script}
                  className="inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-[11px] font-medium text-white/55 transition-colors hover:bg-white/[0.08] hover:text-white disabled:pointer-events-none disabled:opacity-30"
                >
                  {copied ? (
                    <>
                      <Check className="size-3.5 text-[#9CB071]" />
                      {t("models.export_copied")}
                    </>
                  ) : (
                    <>
                      <Copy className="size-3.5" />
                      {t("models.export_copy")}
                    </>
                  )}
                </button>
              </div>
            </div>

            <div className="flex-1 md:min-h-0 overflow-auto">
              {selectedModel ? (
                <pre className="px-5 py-4 text-[12.5px] leading-[1.65] text-[#E5E0D6]">
                  <code
                    className={`block font-mono whitespace-pre ${CLAUDE_SYNTAX}`}
                    dangerouslySetInnerHTML={{ __html: highlighted }}
                  />
                </pre>
              ) : (
                <div className="flex h-full min-h-[280px] flex-col items-center justify-center gap-4 px-6 py-16 text-center">
                  <span className="flex size-14 items-center justify-center rounded-2xl border border-white/[0.07] bg-white/[0.03]">
                    <Codex.Color className="size-7 opacity-30" />
                  </span>
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-white/45">
                      {t("models.export_no_default_model_selected")}
                    </p>
                    <p className="text-xs text-white/25">
                      {t("models.export_codex_empty_hint")}
                    </p>
                  </div>
                </div>
              )}
            </div>

            {selectedModel && (
              <div className="shrink-0 border-t border-white/[0.07] bg-[#30302E] px-4 py-2">
                <p className="font-mono text-[10.5px] leading-relaxed text-white/30">
                  {t("models.export_footer")}
                </p>
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
