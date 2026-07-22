"use client";

import { useMemo, useState } from "react";
import type { ModelItem } from "@/lib/types";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useT } from "@/lib/i18n";
import { ChevronDown, Search, X } from "lucide-react";
import { ClaudeCode } from "@lobehub/icons";
import {
  ExportDialogShell,
  ExportField,
  ExportModelRow,
  ExportSectionTitle,
  formatTokens,
  useFilteredModels,
} from "@/components/export-dialog-shared";

interface ExportClaudecodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: ModelItem[];
}

// The three model tiers Claude Code resolves via env vars.
type TierKey = "opus" | "sonnet" | "haiku";

const TIER_ENV: Record<TierKey, string> = {
  opus: "ANTHROPIC_DEFAULT_OPUS_MODEL",
  sonnet: "ANTHROPIC_DEFAULT_SONNET_MODEL",
  haiku: "ANTHROPIC_DEFAULT_HAIKU_MODEL",
};

const TIER_ORDER: TierKey[] = ["opus", "sonnet", "haiku"];

// Threshold (tokens) at/above which a model's context window qualifies for
// Claude Code's 1M context window — opted in via the [1m] model-name suffix.
const ONE_MILLION = 1_000_000;

function supports1M(m: ModelItem): boolean {
  return m.contextLength >= ONE_MILLION;
}

// Per-tier accent — warm/strong (opus) → balanced blue (sonnet) → cool gray (haiku),
// expressing the capability-to-speed gradient.
const TIER_ACCENT: Record<TierKey, { badge: string; ring: string; dot: string }> = {
  opus: {
    badge: "bg-[#C77B5A]/15 text-[#C77B5A] border-[#C77B5A]/25",
    ring: "border-[#C77B5A]/40 bg-[#C77B5A]/[0.05]",
    dot: "bg-[#C77B5A]",
  },
  sonnet: {
    badge: "bg-[#7DA1C4]/15 text-[#7DA1C4] border-[#7DA1C4]/25",
    ring: "border-[#7DA1C4]/40 bg-[#7DA1C4]/[0.05]",
    dot: "bg-[#7DA1C4]",
  },
  haiku: {
    badge: "bg-muted-foreground/15 text-muted-foreground border-muted-foreground/25",
    ring: "border-muted-foreground/30 bg-muted-foreground/[0.05]",
    dot: "bg-muted-foreground",
  },
};

function generateScript(
  baseUrl: string,
  authToken: string,
  tiers: Record<TierKey, ModelItem | null>
): string {
  const envEntries: [string, string][] = [
    ["ANTHROPIC_BASE_URL", baseUrl],
    ["ANTHROPIC_AUTH_TOKEN", authToken],
  ];
  for (const key of TIER_ORDER) {
    const m = tiers[key];
    // Claude Code enables the 1M-token context window via a [1m] suffix on the
    // model name, but only for models that actually support it. We append it
    // when the upstream model's context window reaches 1M. Claude Code strips
    // the suffix before forwarding upstream, so it stays transparent to the proxy.
    if (m) {
      const alias = supports1M(m) ? `${m.alias}[1m]` : m.alias;
      envEntries.push([TIER_ENV[key], alias]);
    }
  }

  const envJson = JSON.stringify(Object.fromEntries(envEntries), null, 4);

  return `#!/usr/bin/env bash
# Export models from aris-proxy-api to Claude Code
set -euo pipefail

# ─── Configuration ────────────────────────────────────────────
# Edit these or set as environment variables before running
BASE_URL="\${BASE_URL:-${baseUrl}}"
AUTH_TOKEN="\${AUTH_TOKEN:-${authToken}}"
CLAUDE_SETTINGS="\${CLAUDE_SETTINGS:-\\$HOME/.claude/settings.json}"
# ───────────────────────────────────────────────────────────────

python3 << 'PYEOF'
import json, os, sys

settings_path = os.path.expanduser(os.environ.get(
    'CLAUDE_SETTINGS',
    os.path.join(os.path.expanduser('~'), '.claude/settings.json')
))

new_env = ${envJson}

# Allow shell overrides to win over the embedded defaults.
if os.environ.get('BASE_URL'):
    new_env['ANTHROPIC_BASE_URL'] = os.environ['BASE_URL']
if os.environ.get('AUTH_TOKEN'):
    new_env['ANTHROPIC_AUTH_TOKEN'] = os.environ['AUTH_TOKEN']

os.makedirs(os.path.dirname(settings_path), exist_ok=True)

if os.path.exists(settings_path):
    with open(settings_path, 'r') as f:
        settings = json.load(f)
    backup_path = settings_path + '.bak'
    with open(backup_path, 'w') as f:
        json.dump(settings, f, indent=2)
    print(f"Backup saved to {backup_path}")
else:
    settings = {}
    print(f"{settings_path} not found, creating a new one")

env = settings.setdefault('env', {})
env.update(new_env)

with open(settings_path, 'w') as f:
    json.dump(settings, f, indent=2)

configured = [k for k in new_env if k.startswith('ANTHROPIC_DEFAULT_')]
print(f"Claude Code configured: base URL + {len(configured)} model tier(s)")
PYEOF`;
}

function OneMBadge() {
  return (
    <span className="shrink-0 rounded-[4px] border border-[#C77B5A]/30 bg-[#C77B5A]/10 px-1 py-px font-mono text-[9px] font-semibold leading-none text-[#C77B5A]">
      1M
    </span>
  );
}

interface TierPickerProps {
  models: ModelItem[];
  selected: ModelItem | null;
  onSelect: (m: ModelItem | null) => void;
}

// Searchable single-select dropdown for one tier.
function TierPicker({ models, selected, onSelect }: TierPickerProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");

  const filtered = useFilteredModels(models, search);

  // 关闭 popover 时清空搜索
  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) setSearch("");
    setOpen(nextOpen);
  };

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={
          <button
            type="button"
            className="flex h-9 w-full items-center gap-2 rounded-lg border border-input bg-background px-3 text-sm transition-colors hover:border-muted-foreground/40 focus:outline-none focus:ring-2 focus:ring-ring/40"
          />
        }
      >
        {selected ? (
          <span className="flex min-w-0 flex-1 items-center gap-2">
            <span className="truncate font-medium text-foreground">
              {selected.alias}
            </span>
            {supports1M(selected) && <OneMBadge />}
            <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground/60">
              {formatTokens(selected.contextLength || 128000)}
              <span className="mx-0.5 opacity-50">/</span>
              {formatTokens(selected.maxOutputTokens || 64000)}
            </span>
          </span>
        ) : (
          <span className="flex-1 text-left text-muted-foreground">
            {t("models.export_tier_select")}
          </span>
        )}
        <ChevronDown className="size-4 shrink-0 text-muted-foreground/60" />
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className="w-[var(--anchor-width)] min-w-[260px] gap-0 p-0"
      >
        <div className="relative border-b border-border p-2">
          <Search className="absolute left-4 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground/60" />
          <Input
            autoFocus
            placeholder={t("models.search_placeholder")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 pl-8 text-sm"
          />
        </div>
        <div className="max-h-[260px] overflow-y-auto p-1.5 space-y-1">
          {selected && (
            <button
              type="button"
              onClick={() => {
                onSelect(null);
                setOpen(false);
              }}
              className="flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-xs text-muted-foreground transition-colors hover:bg-secondary/60"
            >
              <X className="size-3.5" />
              {t("models.export_tier_clear")}
            </button>
          )}
          {filtered.length === 0 ? (
            <p className="px-2.5 py-6 text-center text-xs text-muted-foreground">
              {t("models.export_no_matches")}
            </p>
          ) : (
            filtered.map((m) => (
              <ExportModelRow
                key={m.id}
                model={m}
                selected={selected?.id === m.id}
                onSelect={() => {
                  onSelect(m);
                  setOpen(false);
                }}
                badge={supports1M(m) ? <OneMBadge /> : undefined}
              />
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export default function ExportClaudecodeDialog({
  open,
  onOpenChange,
  models,
}: ExportClaudecodeDialogProps) {
  const t = useT();

  // lazy initializer：对话框内容仅在打开时挂载，SSR 与客户端初始渲染无差异
  const [baseUrl, setBaseUrl] = useState(() =>
    typeof window === "undefined" ? "" : `${window.location.origin}/api/anthropic/v1`
  );
  const [authToken, setAuthToken] = useState("YOUR_API_KEY");
  const [tiers, setTiers] = useState<Record<TierKey, ModelItem | null>>({
    opus: null,
    sonnet: null,
    haiku: null,
  });

  const hasAnyTier = TIER_ORDER.some((k) => tiers[k] !== null);

  const script = useMemo(
    () => (hasAnyTier ? generateScript(baseUrl, authToken, tiers) : ""),
    [baseUrl, authToken, tiers, hasAnyTier]
  );

  const setTier = (key: TierKey, m: ModelItem | null) => {
    setTiers((prev) => ({ ...prev, [key]: m }));
  };

  // 统一拦截所有关闭路径，关闭时重置选择态
  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setTiers({ opus: null, sonnet: null, haiku: null });
    }
    onOpenChange(nextOpen);
  };

  return (
    <ExportDialogShell
      open={open}
      onOpenChange={handleOpenChange}
      icon={<ClaudeCode.Color className="size-4.5" />}
      title={t("models.export_claudecode_title")}
      description={t("models.export_claudecode_desc")}
      fileName="claude-code-setup.sh"
      script={script}
      emptyIcon={<ClaudeCode.Color className="size-7 opacity-30" />}
      emptyTitle={t("models.export_no_tier_selected")}
      emptyHint={t("models.export_tier_empty_hint")}
    >
      {/* Connection */}
      <section className="space-y-4">
        <ExportSectionTitle>{t("models.export_connection")}</ExportSectionTitle>
        <ExportField
          id="cc-export-base-url"
          label={t("models.export_base_url")}
          placeholder={t("models.export_base_url_anthropic_placeholder")}
          value={baseUrl}
          onChange={setBaseUrl}
        />
        <ExportField
          id="cc-export-auth-token"
          label={t("models.export_auth_token")}
          placeholder={t("models.export_auth_token_placeholder")}
          value={authToken}
          onChange={setAuthToken}
        />
      </section>

      {/* Model tiers */}
      <section className="space-y-3">
        <div className="space-y-1">
          <ExportSectionTitle>{t("models.export_tiers")}</ExportSectionTitle>
          <p className="text-[11px] leading-snug text-muted-foreground/70">
            {t("models.export_tier_optional")}
          </p>
        </div>

        <div className="space-y-2.5">
          {TIER_ORDER.map((key) => {
            const accent = TIER_ACCENT[key];
            const selected = tiers[key];
            return (
              <div
                key={key}
                className={`rounded-xl border p-3 transition-colors ${
                  selected ? accent.ring : "border-border bg-transparent"
                }`}
              >
                <div className="mb-2.5 flex items-center gap-2">
                  <span
                    className={`inline-flex items-center gap-1.5 rounded-md border px-2 py-0.5 text-[11px] font-semibold ${accent.badge}`}
                  >
                    <span className={`size-1.5 rounded-full ${accent.dot}`} />
                    {t(`models.export_tier_${key}_name`)}
                  </span>
                  <span className="truncate text-[11px] text-muted-foreground/70">
                    {t(`models.export_tier_${key}_desc`)}
                  </span>
                </div>
                <TierPicker
                  models={models}
                  selected={selected}
                  onSelect={(m) => setTier(key, m)}
                />
              </div>
            );
          })}
        </div>
      </section>
    </ExportDialogShell>
  );
}
