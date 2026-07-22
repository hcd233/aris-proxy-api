"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ModelItem } from "@/lib/types";
import { useT } from "@/lib/i18n";
import { OpenCode } from "@lobehub/icons";
import {
  ExportDialogShell,
  ExportField,
  ExportModelEmpty,
  ExportModelRow,
  ExportModelSearch,
  ExportSectionTitle,
  ExportSelectionBadge,
  ExportTextButton,
  useFilteredModels,
} from "@/components/export-dialog-shared";

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

export default function ExportDialog({
  open,
  onOpenChange,
  models,
}: ExportDialogProps) {
  const t = useT();
  const searchInputRef = useRef<HTMLInputElement>(null);

  const [providerId, setProviderId] = useState("aris-proxy");
  // lazy initializer：对话框内容仅在打开时挂载，SSR 与客户端初始渲染无差异
  const [baseUrl, setBaseUrl] = useState(() =>
    typeof window === "undefined" ? "" : `${window.location.origin}/api/openai/v1`
  );
  const [apiKey, setApiKey] = useState("YOUR_API_KEY");
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [modelSearch, setModelSearch] = useState("");

  const filteredModels = useFilteredModels(models, modelSearch);

  const selectedModels = useMemo(
    () => models.filter((m) => selectedIds.has(m.id)),
    [models, selectedIds]
  );

  const script = useMemo(
    () => generateScript(providerId, baseUrl, apiKey, selectedModels),
    [providerId, baseUrl, apiKey, selectedModels]
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

  // 统一拦截所有关闭路径，关闭时重置选择态
  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        setSelectedIds(new Set());
        setModelSearch("");
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
    <ExportDialogShell
      open={open}
      onOpenChange={handleOpenChange}
      icon={<OpenCode className="size-4.5" />}
      title={t("models.export_opencode_title")}
      description={t("models.export_desc")}
      fileName="opencode-setup.sh"
      script={script}
      emptyIcon={<OpenCode className="size-7 opacity-30" />}
      emptyTitle={t("models.export_no_models_selected")}
      emptyHint={t("models.export_empty_hint")}
    >
      {/* Connection */}
      <section className="space-y-4">
        <ExportSectionTitle>{t("models.export_connection")}</ExportSectionTitle>
        <ExportField
          id="export-provider-id"
          label={t("models.export_provider_id")}
          placeholder={t("models.export_provider_id_placeholder")}
          value={providerId}
          onChange={setProviderId}
        />
        <ExportField
          id="export-base-url"
          label={t("models.export_base_url")}
          placeholder={t("models.export_base_url_placeholder")}
          value={baseUrl}
          onChange={setBaseUrl}
        />
        <ExportField
          id="export-api-key"
          label={t("models.export_api_key")}
          placeholder={t("models.export_api_key_placeholder")}
          value={apiKey}
          onChange={setApiKey}
        />
      </section>

      {/* Models */}
      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <ExportSectionTitle>
            {t("models.export_select_models")}
            {selectedIds.size > 0 && (
              <ExportSelectionBadge count={selectedIds.size} />
            )}
          </ExportSectionTitle>
          {filteredModels.length > 0 && (
            <ExportTextButton onClick={handleToggleAll}>
              {allFilteredSelected
                ? t("models.export_clear_all")
                : t("models.export_select_all")}
            </ExportTextButton>
          )}
        </div>

        <ExportModelSearch
          value={modelSearch}
          onChange={setModelSearch}
          inputRef={searchInputRef}
        />

        <div className="space-y-1">
          {filteredModels.length === 0 ? (
            <ExportModelEmpty searching={modelSearch.trim().length > 0} />
          ) : (
            filteredModels.map((model) => (
              <ExportModelRow
                key={model.id}
                model={model}
                selected={selectedIds.has(model.id)}
                onSelect={() => handleToggle(model.id)}
              />
            ))
          )}
        </div>
      </section>
    </ExportDialogShell>
  );
}
