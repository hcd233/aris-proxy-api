"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ModelItem } from "@/lib/types";
import { useT } from "@/lib/i18n";
import { Codex } from "@lobehub/icons";
import {
  ExportDialogShell,
  ExportField,
  ExportModelEmpty,
  ExportModelRow,
  ExportModelSearch,
  ExportSectionTitle,
  ExportTextButton,
  useFilteredModels,
} from "@/components/export-dialog-shared";

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

export default function ExportCodexDialog({
  open,
  onOpenChange,
  models,
}: ExportCodexDialogProps) {
  const t = useT();
  const searchInputRef = useRef<HTMLInputElement>(null);

  const [providerId, setProviderId] = useState("aris-proxy");
  // lazy initializer：对话框内容仅在打开时挂载，SSR 与客户端初始渲染无差异
  const [baseUrl, setBaseUrl] = useState(() =>
    typeof window === "undefined" ? "" : `${window.location.origin}/api/openai/v1`
  );
  const [apiKey, setApiKey] = useState("YOUR_API_KEY");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [modelSearch, setModelSearch] = useState("");

  const filteredModels = useFilteredModels(models, modelSearch);

  const selectedModel = useMemo(
    () => models.find((m) => m.id === selectedId) ?? null,
    [models, selectedId]
  );

  const script = useMemo(
    () => generateScript(providerId, baseUrl, apiKey, selectedModel),
    [providerId, baseUrl, apiKey, selectedModel]
  );

  // 统一拦截所有关闭路径，关闭时重置选择态
  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) {
        setSelectedId(null);
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
      icon={<Codex.Color className="size-4.5" />}
      title={t("models.export_codex_title")}
      description={t("models.export_codex_desc")}
      fileName="codex-setup.sh"
      script={script}
      emptyIcon={<Codex.Color className="size-7 opacity-30" />}
      emptyTitle={t("models.export_no_default_model_selected")}
      emptyHint={t("models.export_codex_empty_hint")}
    >
      {/* Connection */}
      <section className="space-y-4">
        <ExportSectionTitle>{t("models.export_connection")}</ExportSectionTitle>
        <ExportField
          id="export-codex-provider-id"
          label={t("models.export_provider_id")}
          placeholder={t("models.export_provider_id_placeholder")}
          value={providerId}
          onChange={setProviderId}
        />
        <ExportField
          id="export-codex-base-url"
          label={t("models.export_base_url")}
          placeholder={t("models.export_base_url_placeholder")}
          value={baseUrl}
          onChange={setBaseUrl}
        />
        <ExportField
          id="export-codex-api-key"
          label={t("models.export_api_key")}
          placeholder={t("models.export_api_key_placeholder")}
          value={apiKey}
          onChange={setApiKey}
        />
      </section>

      {/* Default model (single select) */}
      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <ExportSectionTitle>
            {t("models.export_default_model")}
          </ExportSectionTitle>
          {selectedModel && (
            <ExportTextButton onClick={() => setSelectedId(null)}>
              {t("models.export_tier_clear")}
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
                selected={selectedId === model.id}
                onSelect={() => setSelectedId(model.id)}
              />
            ))
          )}
        </div>
      </section>
    </ExportDialogShell>
  );
}
