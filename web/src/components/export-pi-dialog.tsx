"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ModelItem } from "@/lib/types";
import { useT } from "@/lib/i18n";
import { Pi } from "@lobehub/icons";
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

interface ExportPiDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: ModelItem[];
}

function buildPiModels(models: ModelItem[]) {
  return models.map((model) => ({
    id: model.alias,
    name: model.alias,
    reasoning: true,
    input: ["text"],
    contextWindow: model.contextLength > 0 ? model.contextLength : 128000,
    maxTokens: model.maxOutputTokens > 0 ? model.maxOutputTokens : 16384,
    cost: {
      input: 0,
      output: 0,
      cacheRead: 0,
      cacheWrite: 0,
    },
  }));
}

function generateScript(
  providerId: string,
  baseUrl: string,
  apiKey: string,
  selectedModels: ModelItem[]
): string {
  if (selectedModels.length === 0) return "";

  const modelsJson = JSON.stringify(buildPiModels(selectedModels), null, 2);
  const providerIdJson = JSON.stringify(providerId);
  const baseUrlJson = JSON.stringify(baseUrl);
  const apiKeyJson = JSON.stringify(apiKey);

  return `#!/usr/bin/env bash
# Export models from aris-proxy-api to Pi
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────
# Edit these or set as environment variables before running
export PROVIDER_ID="\${PROVIDER_ID:-}"
export BASE_URL="\${BASE_URL:-}"
export API_KEY="\${API_KEY:-}"
export PI_MODELS_CONFIG="\${PI_MODELS_CONFIG:-\$HOME/.pi/agent/models.json}"
# ───────────────────────────────────────────────────────────────

python3 << 'PYEOF'
import json
import os
import shutil
import tempfile

config_path = os.path.expanduser(os.environ["PI_MODELS_CONFIG"])
provider_id = os.environ.get("PROVIDER_ID") or ${providerIdJson}
base_url = os.environ.get("BASE_URL") or ${baseUrlJson}
api_key = os.environ.get("API_KEY") or ${apiKeyJson}
models = json.loads(${JSON.stringify(modelsJson)})

config_dir = os.path.dirname(config_path) or "."
os.makedirs(config_dir, exist_ok=True)
if os.path.normpath(config_path) == os.path.expanduser("~/.pi/agent/models.json"):
    os.chmod(config_dir, 0o700)

if os.path.exists(config_path):
    shutil.copyfile(config_path, config_path + ".bak")
    os.chmod(config_path + ".bak", 0o600)
    with open(config_path, "r", encoding="utf-8") as file:
        config = json.load(file)
else:
    config = {}

providers = config.setdefault("providers", {})
provider = providers.setdefault(provider_id, {})
provider["baseUrl"] = base_url
provider["api"] = "openai-completions"
provider["apiKey"] = api_key
provider["compat"] = {"supportsDeveloperRole": False}

existing_models = provider.get("models", [])
selected_by_id = {model["id"]: model for model in models}
merged_models = [
    model
    for model in existing_models
    if model.get("id") not in selected_by_id
]
merged_models.extend(selected_by_id.values())
provider["models"] = merged_models

with tempfile.NamedTemporaryFile(
    mode="w", encoding="utf-8", dir=config_dir, prefix=".models.json.", delete=False
) as file:
    temp_path = file.name
    os.chmod(temp_path, 0o600)
    json.dump(config, file, indent=2, ensure_ascii=False)
    file.write("\\n")
    file.flush()
    os.fsync(file.fileno())
os.replace(temp_path, config_path)
os.chmod(config_path, 0o600)

print(f"Pi configured: provider '{provider_id}' with {len(models)} selected models")
PYEOF`;
}

export default function ExportPiDialog({
  open,
  onOpenChange,
  models,
}: ExportPiDialogProps) {
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
    () => models.filter((model) => selectedIds.has(model.id)),
    [models, selectedIds]
  );

  const duplicateAliases = useMemo(() => {
    const counts = new Map<string, number>();
    selectedModels.forEach((model) => {
      counts.set(model.alias, (counts.get(model.alias) ?? 0) + 1);
    });
    return [...counts.entries()]
      .filter(([, count]) => count > 1)
      .map(([alias]) => alias);
  }, [selectedModels]);

  const script = useMemo(
    () =>
      duplicateAliases.length > 0
        ? ""
        : generateScript(providerId, baseUrl, apiKey, selectedModels),
    [providerId, baseUrl, apiKey, selectedModels, duplicateAliases]
  );

  const allFilteredSelected =
    filteredModels.length > 0 &&
    filteredModels.every((model) => selectedIds.has(model.id));

  const handleToggle = useCallback((id: number) => {
    setSelectedIds((previous) => {
      const next = new Set(previous);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const handleToggleAll = useCallback(() => {
    setSelectedIds((previous) => {
      const next = new Set(previous);
      const everySelected = filteredModels.every((model) => next.has(model.id));
      filteredModels.forEach((model) =>
        everySelected ? next.delete(model.id) : next.add(model.id)
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
      icon={<Pi size={18} />}
      title={t("models.export_pi_title")}
      description={t("models.export_pi_desc")}
      fileName={t("models.export_pi_script_filename")}
      script={script}
      emptyIcon={<Pi size={28} className="opacity-30" />}
      emptyTitle={t("models.export_no_models_selected")}
      emptyHint={t("models.export_pi_empty_hint")}
      errorMessage={
        duplicateAliases.length > 0
          ? t("models.export_duplicate_aliases")
          : null
      }
    >
      {/* Connection */}
      <section className="space-y-4">
        <ExportSectionTitle>{t("models.export_connection")}</ExportSectionTitle>
        <ExportField
          id="export-pi-provider-id"
          label={t("models.export_provider_id")}
          placeholder={t("models.export_provider_id_placeholder")}
          value={providerId}
          onChange={setProviderId}
        />
        <ExportField
          id="export-pi-base-url"
          label={t("models.export_base_url")}
          placeholder={t("models.export_base_url_placeholder")}
          value={baseUrl}
          onChange={setBaseUrl}
        />
        <ExportField
          id="export-pi-api-key"
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
                outputFallback={16384}
              />
            ))
          )}
        </div>
        {duplicateAliases.length > 0 && (
          <p className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
            {t("models.export_duplicate_aliases")}
          </p>
        )}
      </section>
    </ExportDialogShell>
  );
}
