"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ModelItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useT } from "@/lib/i18n";
import { Check, Copy, FileDown } from "lucide-react";

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
PYEOF
`;
}

export default function ExportDialog({
  open,
  onOpenChange,
  models,
}: ExportDialogProps) {
  const t = useT();

  const [providerId, setProviderId] = useState("aris-proxy");
  const [baseUrl, setBaseUrl] = useState("");
  const [apiKey, setApiKey] = useState("YOUR_API_KEY");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [script, setScript] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (typeof window !== "undefined") {
      setBaseUrl(`${window.location.origin}/api/openai/v1`);
    }
  }, []);

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

  const selectedModels = useMemo(
    () => models.filter((m) => selectedIds.has(m.id)),
    [models, selectedIds]
  );

  const handleGenerate = useCallback(() => {
    const s = generateScript(providerId, baseUrl, apiKey, selectedModels);
    setScript(s);
    setCopied(false);
  }, [providerId, baseUrl, apiKey, selectedModels]);

  const handleCopy = useCallback(async () => {
    if (!script) return;
    await navigator.clipboard.writeText(script);
    setCopied(true);
  }, [script]);

  const handleClose = useCallback(() => {
    onOpenChange(false);
  }, [onOpenChange]);

  const hasSelection = selectedIds.size > 0;
  const canGenerate = providerId.trim() && baseUrl.trim() && apiKey.trim() && hasSelection;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileDown className="size-5" />
            {t("models.export")}
          </DialogTitle>
          <DialogDescription>{t("models.export_desc")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Provider ID */}
          <div className="space-y-1.5">
            <Label htmlFor="export-provider-id">{t("models.export_provider_id")}</Label>
            <Input
              id="export-provider-id"
              placeholder={t("models.export_provider_id_placeholder")}
              value={providerId}
              onChange={(e) => setProviderId(e.target.value)}
            />
          </div>

          {/* Base URL */}
          <div className="space-y-1.5">
            <Label htmlFor="export-base-url">{t("models.export_base_url")}</Label>
            <Input
              id="export-base-url"
              placeholder={t("models.export_base_url_placeholder")}
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
            />
          </div>

          {/* API Key */}
          <div className="space-y-1.5">
            <Label htmlFor="export-api-key">{t("models.export_api_key")}</Label>
            <Input
              id="export-api-key"
              placeholder={t("models.export_api_key_placeholder")}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
          </div>

          {/* Separator */}
          <div className="border-t border-border" />

          {/* Model Selection */}
          <div className="space-y-1.5">
            <Label>{t("models.export_select_models")}</Label>
            <div className="max-h-48 overflow-y-auto space-y-1 rounded-md border border-border p-2">
              {models.length === 0 && (
                <p className="py-4 text-center text-sm text-muted-foreground">
                  {t("models.no_models")}
                </p>
              )}
              {models.map((model) => {
                const selected = selectedIds.has(model.id);
                return (
                  <label
                    key={model.id}
                    className="flex cursor-pointer items-center gap-2.5 rounded-sm px-2 py-1.5 text-sm hover:bg-accent"
                  >
                    <div
                      className={`flex size-4 shrink-0 items-center justify-center rounded-sm border ${
                        selected
                          ? "border-primary bg-primary text-primary-foreground"
                          : "border-input"
                      }`}
                    >
                      {selected && <Check className="size-3" />}
                    </div>
                    <input
                      type="checkbox"
                      className="sr-only"
                      checked={selected}
                      onChange={() => handleToggle(model.id)}
                    />
                    <span className="font-medium">{model.alias}</span>
                    <span className="font-mono text-xs text-muted-foreground">
                      {model.modelName}
                    </span>
                  </label>
                );
              })}
            </div>
            {!hasSelection && script === null && (
              <p className="text-xs text-muted-foreground">
                {t("models.export_no_models_selected")}
              </p>
            )}
          </div>

          {/* Generated Script */}
          {script && (
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label>{t("models.export_script_title")}</Label>
                <Button variant="outline" size="xs" onClick={handleCopy}>
                  {copied ? (
                    <>
                      <Check className="mr-1 size-3" />
                      {t("models.export_copied")}
                    </>
                  ) : (
                    <>
                      <Copy className="mr-1 size-3" />
                      {t("models.export_copy")}
                    </>
                  )}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                {t("models.export_script_hint")}
              </p>
              <pre className="max-h-64 overflow-auto rounded-md border border-border bg-muted p-3 text-xs leading-relaxed">
                <code>{script}</code>
              </pre>
              <p className="text-xs text-muted-foreground">
                {t("models.export_footer")}
              </p>
            </div>
          )}
        </div>

        <DialogFooter className="gap-2">
          <Button variant="outline" onClick={handleClose}>
            {script ? t("share_dialog.close") : t("common.cancel")}
          </Button>
          <Button onClick={handleGenerate} disabled={!canGenerate}>
            {script ? t("models.export_regenerate") : t("models.export_generate")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
