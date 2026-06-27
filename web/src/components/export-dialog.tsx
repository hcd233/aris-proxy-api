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
import { Check, Copy, FileDown, Search } from "lucide-react";

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
          limit: { context: 128000, output: 64000 },
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

export default function ExportDialog({
  open,
  onOpenChange,
  models,
}: ExportDialogProps) {
  const t = useT();
  const previewRef = useRef<HTMLDivElement>(null);
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
      setTimeout(() => searchInputRef.current?.focus(), 100);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[900px] w-[calc(100vw-3rem)] max-h-[85vh] p-0 gap-0 overflow-hidden">
        <DialogHeader className="px-6 pt-5 pb-0">
          <DialogTitle className="flex items-center gap-2.5">
            <span className="flex size-7 items-center justify-center rounded-lg border border-border bg-muted">
              <FileDown className="size-3.5" />
            </span>
            <span>{t("models.export")}</span>
          </DialogTitle>
          <DialogDescription>{t("models.export_desc")}</DialogDescription>
        </DialogHeader>

        <div className="grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-border">
          {/* ─── Left: Config Form ─── */}
          <div className="p-6 space-y-5 overflow-y-auto max-h-[calc(85vh-100px)]">
            {/* Provider ID */}
            <div className="space-y-1.5">
              <Label htmlFor="export-provider-id" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t("models.export_provider_id")}
              </Label>
              <Input
                id="export-provider-id"
                placeholder={t("models.export_provider_id_placeholder")}
                value={providerId}
                onChange={(e) => setProviderId(e.target.value)}
              />
            </div>

            {/* Base URL */}
            <div className="space-y-1.5">
              <Label htmlFor="export-base-url" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t("models.export_base_url")}
              </Label>
              <Input
                id="export-base-url"
                placeholder={t("models.export_base_url_placeholder")}
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
              />
            </div>

            {/* API Key */}
            <div className="space-y-1.5">
              <Label htmlFor="export-api-key" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t("models.export_api_key")}
              </Label>
              <Input
                id="export-api-key"
                placeholder={t("models.export_api_key_placeholder")}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
              />
            </div>

            <div className="border-t border-border" />

            {/* Model Selection */}
            <div className="space-y-2">
              <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {t("models.export_select_models")}
                <span className="ml-1.5 text-muted-foreground/60 font-normal normal-case">
                  ({selectedIds.size})
                </span>
              </Label>

              {/* Search */}
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground/60" />
                <Input
                  ref={searchInputRef}
                  placeholder={t("models.search_placeholder")}
                  value={modelSearch}
                  onChange={(e) => setModelSearch(e.target.value)}
                  className="h-8 pl-8 text-xs"
                />
              </div>

              {/* List */}
              <div className="space-y-0.5 max-h-52 overflow-y-auto rounded-lg border border-border">
                {filteredModels.length === 0 && (
                  <p className="py-6 text-center text-xs text-muted-foreground">
                    {modelSearch ? "No matches" : t("models.no_models")}
                  </p>
                )}
                {filteredModels.map((model) => {
                  const selected = selectedIds.has(model.id);
                  return (
                    <label
                      key={model.id}
                      className={`flex cursor-pointer items-center gap-2.5 px-3 py-2 text-sm transition-colors hover:bg-accent/50 ${
                        selected ? "bg-accent/30" : ""
                      }`}
                    >
                      <span
                        className={`flex size-4 shrink-0 items-center justify-center rounded border transition-colors ${
                          selected
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-border"
                        }`}
                      >
                        {selected && <Check className="size-3" />}
                      </span>
                      <input
                        type="checkbox"
                        className="sr-only"
                        checked={selected}
                        onChange={() => handleToggle(model.id)}
                      />
                      <div className="flex flex-col min-w-0">
                        <span className="font-medium truncate text-sm">{model.alias}</span>
                        <span className="truncate font-mono text-[10px] text-muted-foreground/70">
                          {model.modelName}
                        </span>
                      </div>
                    </label>
                  );
                })}
              </div>

              {selectedIds.size > 0 && (
                <button
                  onClick={() => setSelectedIds(new Set())}
                  className="text-[11px] text-muted-foreground/60 hover:text-muted-foreground transition-colors"
                >
                  Clear all ({selectedIds.size})
                </button>
              )}
            </div>

            {/* Footer buttons */}
            <div className="flex items-center gap-2 pt-2">
              <Button variant="outline" size="sm" onClick={handleClose}>
                {t("share_dialog.close")}
              </Button>
            </div>
          </div>

          {/* ─── Right: Script Preview ─── */}
          <div className="flex flex-col bg-[#0d1117] text-[#e6edf3] min-h-[300px]">
            {/* Preview toolbar */}
            <div className="flex items-center justify-between px-4 py-2 border-b border-white/5">
              <div className="flex items-center gap-2">
                <div className="flex gap-1.5">
                  <span className="size-2.5 rounded-full bg-[#ff5f56]" />
                  <span className="size-2.5 rounded-full bg-[#ffbd2e]" />
                  <span className="size-2.5 rounded-full bg-[#27c93f]" />
                </div>
                <span className="text-[11px] text-white/30 font-mono">
                  export.sh
                </span>
              </div>
              <div className="flex items-center gap-2">
                {selectedIds.size > 0 && (
                  <span className="text-[10px] text-white/25 font-mono">
                    {script.length.toLocaleString()}b
                  </span>
                )}
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={handleCopy}
                  disabled={!script}
                  className="h-7 text-white/40 hover:text-white hover:bg-white/5"
                >
                  {copied ? (
                    <><Check className="size-3 mr-1" />{t("models.export_copied")}</>
                  ) : (
                    <><Copy className="size-3 mr-1" />{t("models.export_copy")}</>
                  )}
                </Button>
              </div>
            </div>

            {/* Code */}
            <div
              ref={previewRef}
              className="flex-1 overflow-auto p-4 text-[13px] leading-relaxed font-mono"
            >
              {selectedIds.size > 0 ? (
                <div
                  className="[&_.hljs-comment]:text-[#8b949e] [&_.hljs-keyword]:text-[#ff7b72] [&_.hljs-built_in]:text-[#d2a8ff] [&_.hljs-string]:text-[#a5d6ff] [&_.hljs-number]:text-[#79c0ff] [&_.hljs-literal]:text-[#79c0ff] [&_.hljs-attr]:text-[#79c0ff] [&_.hljs-title]:text-[#d2a8ff] [&_.hljs-params]:text-[#e6edf3] [&_.hljs-variable]:text-[#ffa657] [&_.hljs-operator]:text-[#ff7b72] [&_.hljs-punctuation]:text-[#e6edf3] [&_.hljs-property]:text-[#79c0ff] whitespace-pre-wrap break-all"
                  dangerouslySetInnerHTML={{ __html: highlighted }}
                />
              ) : (
                <div className="flex flex-col items-center justify-center h-full text-white/20">
                  <svg className="size-10 mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                  </svg>
                  <p className="text-xs">{t("models.export_no_models_selected")}</p>
                </div>
              )}
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
