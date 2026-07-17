"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ModelItem } from "@/lib/types";
import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import python from "highlight.js/lib/languages/python";
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
import { OpenCode } from "@lobehub/icons";

interface ExportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: ModelItem[];
}

function generateScript(
  providerId: string,
  baseUrl: string,
  apiKey: string,
  selectedModels: ModelItem[]
): string {
  if (selectedModels.length === 0) return "";

  const modelsJson = JSON.stringify(
    Object.fromEntries(
      selectedModels.map((m) => [
        m.alias,
        {
          name: m.alias.charAt(0).toUpperCase() + m.alias.slice(1),
          limit: {
            context: m.contextLength > 0 ? m.contextLength : 128000,
            output: m.maxOutputTokens > 0 ? m.maxOutputTokens : 64000,
          },
          temperature: true,
          tool_call: true,
        },
      ])
    ),
    null,
    4
  );

  return `#!/usr/bin/env bash
# Export models from aris-proxy-api to OpenCode
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────
# Edit these or set as environment variables before running
PROVIDER_ID="\${PROVIDER_ID:-${providerId}}"
BASE_URL="\${BASE_URL:-${baseUrl}}"
API_KEY="\${API_KEY:-${apiKey}}"
OPENCODE_CONFIG="\${OPENCODE_CONFIG:-\\$HOME/.config/opencode/opencode.json}"
# ───────────────────────────────────────────────────────────────

python3 << 'PYEOF'
import json, os, sys

config_path = os.path.expanduser(os.environ.get(
    'OPENCODE_CONFIG',
    os.path.expanduser(os.path.join(os.path.expanduser('~'), '.config/opencode/opencode.json'))
))
provider_id = os.environ.get('PROVIDER_ID', '${providerId}')
base_url = os.environ.get('BASE_URL', '${baseUrl}')
api_key = os.environ.get('API_KEY', '${apiKey}')

models = ${modelsJson}

if not os.path.exists(config_path):
    print(f"Error: {config_path} not found", file=sys.stderr)
    sys.exit(1)

with open(config_path, 'r') as f:
    config = json.load(f)

if 'provider' not in config:
    config['provider'] = {}

if provider_id in config['provider']:
    print(f"Provider '{provider_id}' already exists. Updating...")
    existing = config['provider'][provider_id]
    existing.setdefault('models', {}).update(models)
else:
    config['provider'][provider_id] = {
        "name": provider_id,
        "npm": "@ai-sdk/openai-compatible",
        "options": {
            "baseURL": base_url,
            "headers": {
                "Authorization": f"Bearer {api_key}"
            }
        },
        "models": models
    }

backup_path = config_path + '.bak'
with open(backup_path, 'w') as f:
    json.dump(config, f, indent=2)
print(f"Backup saved to {backup_path}")

with open(config_path, 'w') as f:
    json.dump(config, f, indent=2)

print(f"Provider '{provider_id}' configured with {len(models)} models")
PYEOF`;
}

hljs.registerLanguage("bash", bash);
hljs.registerLanguage("python", python);

function highlightBash(code: string): string {
  return hljs.highlight(code, { language: "bash" }).value;
}

// 将 token 数格式化为紧凑可读形式：128000 -> 128K
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

// Claude.ai-style warm syntax palette mapped onto highlight.js tokens.
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

export default function ExportDialog({
  open,
  onOpenChange,
  models,
}: ExportDialogProps) {
  const t = useT();
  const searchInputRef = useRef<HTMLInputElement>(null);

  const [providerId, setProviderId] = useState("aris-proxy");
  const [baseUrl, setBaseUrl] = useState("");
  const [apiKey, setApiKey] = useState("YOUR_API_KEY");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [copied, setCopied] = useState(false);
  const [modelSearch, setModelSearch] = useState("");

  useEffect(() => {
    if (typeof window !== "undefined") {
      setBaseUrl(`${window.location.origin}/api/openai/v1`);
    }
  }, []);

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

  const selectedModels = useMemo(
    () => models.filter((m) => selectedIds.has(m.id)),
    [models, selectedIds]
  );

  const script = useMemo(
    () => generateScript(providerId, baseUrl, apiKey, selectedModels),
    [providerId, baseUrl, apiKey, selectedModels]
  );

  const highlighted = useMemo(
    () => (script ? highlightBash(script) : ""),
    [script]
  );

  const lineCount = useMemo(
    () => (script ? script.split("\n").length : 0),
    [script]
  );

  const handleToggle = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const allFilteredSelected =
    filteredModels.length > 0 &&
    filteredModels.every((m) => selectedIds.has(m.id));

  const handleToggleAll = useCallback(() => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      const everySelected = filteredModels.every((m) => next.has(m.id));
      filteredModels.forEach((m) =>
        everySelected ? next.delete(m.id) : next.add(m.id)
      );
      return next;
    });
  }, [filteredModels]);

  const handleCopy = useCallback(async () => {
    if (!script) return;
    await navigator.clipboard.writeText(script);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [script]);

  const handleClose = useCallback(() => {
    onOpenChange(false);
  }, [onOpenChange]);

  useEffect(() => {
    if (!open) {
      setSelectedIds(new Set());
      setModelSearch("");
      setCopied(false);
    }
  }, [open]);

  useEffect(() => {
    if (open && searchInputRef.current) {
      setTimeout(() => searchInputRef.current?.focus(), 120);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="!max-w-[1040px] w-[calc(100vw-1.5rem)] h-[min(86vh,720px)] p-0 gap-0 overflow-hidden flex flex-col sm:!max-w-[1040px]"
      >
        {/* ─── Header ─── */}
        <DialogHeader className="shrink-0 flex-row items-center gap-3 px-6 py-4 border-b border-border">
          <span className="flex size-9 items-center justify-center rounded-xl border border-border bg-gradient-to-b from-secondary to-muted shadow-sm">
            <OpenCode className="size-4.5" />
          </span>
          <div className="flex flex-col gap-0.5 min-w-0">
            <DialogTitle className="font-display text-base leading-tight">
              {t("models.export_opencode_title")}
            </DialogTitle>
            <DialogDescription className="text-xs leading-snug">
              {t("models.export_desc")}
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

        {/* ─── Body: two independently-scrolling panes ─── */}
        <div className="flex flex-1 min-h-0 flex-col overflow-y-auto md:grid md:grid-cols-[minmax(0,1fr)_minmax(0,1.12fr)] md:overflow-hidden">
          {/* ── Left: configuration ── */}
          <div className="md:min-h-0 md:overflow-y-auto md:border-r border-border px-6 py-5 space-y-6">
            {/* Connection */}
            <section className="space-y-4">
              <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("models.export_connection")}
              </h3>

              <div className="space-y-1.5">
                <Label
                  htmlFor="export-provider-id"
                  className="text-xs font-medium text-foreground/80"
                >
                  {t("models.export_provider_id")}
                </Label>
                <Input
                  id="export-provider-id"
                  placeholder={t("models.export_provider_id_placeholder")}
                  value={providerId}
                  onChange={(e) => setProviderId(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label
                  htmlFor="export-base-url"
                  className="text-xs font-medium text-foreground/80"
                >
                  {t("models.export_base_url")}
                </Label>
                <Input
                  id="export-base-url"
                  placeholder={t("models.export_base_url_placeholder")}
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label
                  htmlFor="export-api-key"
                  className="text-xs font-medium text-foreground/80"
                >
                  {t("models.export_api_key")}
                </Label>
                <Input
                  id="export-api-key"
                  placeholder={t("models.export_api_key_placeholder")}
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>
            </section>

            {/* Models */}
            <section className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                  {t("models.export_select_models")}
                  {selectedIds.size > 0 && (
                    <span className="ml-1.5 inline-flex items-center rounded-full bg-primary/10 px-1.5 py-px text-[10px] font-semibold text-primary tabular-nums normal-case tracking-normal">
                      {selectedIds.size}
                    </span>
                  )}
                </h3>
                {filteredModels.length > 0 && (
                  <button
                    type="button"
                    onClick={handleToggleAll}
                    className="text-[11px] font-medium text-primary/80 transition-colors hover:text-primary"
                  >
                    {allFilteredSelected
                      ? t("models.export_clear_all")
                      : t("models.export_select_all")}
                  </button>
                )}
              </div>

              {/* Search */}
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground/60" />
                <Input
                  ref={searchInputRef}
                  placeholder={t("models.search_placeholder")}
                  value={modelSearch}
                  onChange={(e) => setModelSearch(e.target.value)}
                  className="h-8 pl-8 text-sm"
                />
              </div>

              {/* List */}
              <div className="space-y-1">
                {filteredModels.length === 0 ? (
                  <p className="rounded-lg border border-dashed border-border py-8 text-center text-xs text-muted-foreground">
                    {modelSearch
                      ? t("models.export_no_matches")
                      : t("models.no_models")}
                  </p>
                ) : (
                  filteredModels.map((model) => {
                    const selected = selectedIds.has(model.id);
                    return (
                      <label
                        key={model.id}
                        className={`group relative flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2 transition-all focus-within:ring-2 focus-within:ring-ring/40 ${
                          selected
                            ? "border-primary/40 bg-primary/[0.06]"
                            : "border-transparent hover:border-border hover:bg-secondary/60"
                        }`}
                      >
                        <span
                          className={`flex size-4 shrink-0 items-center justify-center rounded-[5px] border transition-colors ${
                            selected
                              ? "border-primary bg-primary text-primary-foreground"
                              : "border-muted-foreground/30 group-hover:border-muted-foreground/50"
                          }`}
                        >
                          {selected && <Check className="size-3" strokeWidth={3} />}
                        </span>
                        <input
                          type="checkbox"
                          className="absolute inset-0 z-10 size-full cursor-pointer opacity-0"
                          checked={selected}
                          onChange={() => handleToggle(model.id)}
                        />
                        <div className="flex min-w-0 flex-1 flex-col">
                          <span className="truncate text-sm font-medium text-foreground">
                            {model.alias}
                          </span>
                          <span className="truncate font-mono text-[11px] text-muted-foreground/70">
                            {model.modelName}
                          </span>
                        </div>
                        <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground/60">
                          {formatTokens(model.contextLength || 128000)}
                          <span className="mx-0.5 opacity-50">/</span>
                          {formatTokens(model.maxOutputTokens || 64000)}
                        </span>
                      </label>
                    );
                  })
                )}
              </div>
            </section>
          </div>

          {/* ── Right: Claude-style code preview ── */}
          <div className="flex flex-col md:min-h-0 md:overflow-hidden bg-[#262624]">
            {/* Window toolbar */}
            <div className="shrink-0 flex items-center justify-between gap-3 border-b border-white/[0.07] bg-[#30302E] px-4 py-2.5">
              <div className="flex items-center gap-2 min-w-0">
                <Terminal className="size-3.5 shrink-0 text-white/35" />
                <span className="truncate font-mono text-xs text-white/50">
                  opencode-setup.sh
                </span>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                {selectedIds.size > 0 && (
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

            {/* Code surface */}
            <div className="flex-1 md:min-h-0 overflow-auto">
              {selectedIds.size > 0 ? (
                <pre className="px-5 py-4 text-[12.5px] leading-[1.65] text-[#E5E0D6]">
                  <code
                    className={`block font-mono whitespace-pre ${CLAUDE_SYNTAX}`}
                    dangerouslySetInnerHTML={{ __html: highlighted }}
                  />
                </pre>
              ) : (
                <div className="flex h-full min-h-[280px] flex-col items-center justify-center gap-4 px-6 py-16 text-center">
                  <span className="flex size-14 items-center justify-center rounded-2xl border border-white/[0.07] bg-white/[0.03]">
                    <OpenCode className="size-7 opacity-30" />
                  </span>
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-white/45">
                      {t("models.export_no_models_selected")}
                    </p>
                    <p className="text-xs text-white/25">
                      {t("models.export_empty_hint")}
                    </p>
                  </div>
                </div>
              )}
            </div>

            {/* Run hint footer */}
            {selectedIds.size > 0 && (
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
