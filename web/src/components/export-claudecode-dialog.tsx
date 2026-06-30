"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useT } from "@/lib/i18n";
import { Check, ChevronDown, Copy, Search, Terminal, X } from "lucide-react";
import { ClaudeCode } from "@lobehub/icons";

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

hljs.registerLanguage("bash", bash);

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

  const filtered = useMemo(
    () =>
      search.trim()
        ? models.filter(
            (m) =>
              m.alias.toLowerCase().includes(search.toLowerCase()) ||
              m.modelName.toLowerCase().includes(search.toLowerCase())
          )
        : models,
    [models, search]
  );

  useEffect(() => {
    if (!open) setSearch("");
  }, [open]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
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
            {supports1M(selected) && (
              <span className="shrink-0 rounded-[4px] border border-[#C77B5A]/30 bg-[#C77B5A]/10 px-1 py-px font-mono text-[9px] font-semibold leading-none text-[#C77B5A]">
                1M
              </span>
            )}
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
        <div className="max-h-[260px] overflow-y-auto p-1.5">
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
            filtered.map((m) => {
              const isSel = selected?.id === m.id;
              return (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => {
                    onSelect(m);
                    setOpen(false);
                  }}
                  className={`flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors ${
                    isSel ? "bg-primary/[0.08]" : "hover:bg-secondary/60"
                  }`}
                >
                  <span
                    className={`flex size-4 shrink-0 items-center justify-center rounded-[5px] border ${
                      isSel
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-muted-foreground/30"
                    }`}
                  >
                    {isSel && <Check className="size-3" strokeWidth={3} />}
                  </span>
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="flex items-center gap-1.5">
                      <span className="truncate text-sm font-medium text-foreground">
                        {m.alias}
                      </span>
                      {supports1M(m) && (
                        <span className="shrink-0 rounded-[4px] border border-[#C77B5A]/30 bg-[#C77B5A]/10 px-1 py-px font-mono text-[9px] font-semibold leading-none text-[#C77B5A]">
                          1M
                        </span>
                      )}
                    </span>
                    <span className="truncate font-mono text-[11px] text-muted-foreground/70">
                      {m.modelName}
                    </span>
                  </span>
                  <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground/60">
                    {formatTokens(m.contextLength || 128000)}
                    <span className="mx-0.5 opacity-50">/</span>
                    {formatTokens(m.maxOutputTokens || 64000)}
                  </span>
                </button>
              );
            })
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

  const [baseUrl, setBaseUrl] = useState("");
  const [authToken, setAuthToken] = useState("YOUR_API_KEY");
  const [tiers, setTiers] = useState<Record<TierKey, ModelItem | null>>({
    opus: null,
    sonnet: null,
    haiku: null,
  });
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (typeof window !== "undefined") {
      setBaseUrl(`${window.location.origin}/api/anthropic/v1`);
    }
  }, []);

  const hasAnyTier = TIER_ORDER.some((k) => tiers[k] !== null);

  const script = useMemo(
    () => (hasAnyTier ? generateScript(baseUrl, authToken, tiers) : ""),
    [baseUrl, authToken, tiers, hasAnyTier]
  );

  const highlighted = useMemo(
    () => (script ? highlightBash(script) : ""),
    [script]
  );

  const lineCount = useMemo(
    () => (script ? script.split("\n").length : 0),
    [script]
  );

  const setTier = useCallback((key: TierKey, m: ModelItem | null) => {
    setTiers((prev) => ({ ...prev, [key]: m }));
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
      setTiers({ opus: null, sonnet: null, haiku: null });
      setCopied(false);
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
            <ClaudeCode.Color className="size-4.5" />
          </span>
          <div className="flex flex-col gap-0.5 min-w-0">
            <DialogTitle className="font-display text-base leading-tight">
              {t("models.export_claudecode_title")}
            </DialogTitle>
            <DialogDescription className="text-xs leading-snug">
              {t("models.export_claudecode_desc")}
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
        <div className="flex flex-col flex-1 min-h-0 overflow-y-auto md:overflow-hidden md:grid md:grid-cols-[minmax(0,1fr)_minmax(0,1.12fr)]">
          {/* ── Left: configuration ── */}
          <div className="md:min-h-0 md:overflow-y-auto md:border-r border-border px-6 py-5 space-y-6">
            {/* Connection */}
            <section className="space-y-4">
              <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                {t("models.export_connection")}
              </h3>

              <div className="space-y-1.5">
                <Label
                  htmlFor="cc-export-base-url"
                  className="text-xs font-medium text-foreground/80"
                >
                  {t("models.export_base_url")}
                </Label>
                <Input
                  id="cc-export-base-url"
                  placeholder={t("models.export_base_url_anthropic_placeholder")}
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <Label
                  htmlFor="cc-export-auth-token"
                  className="text-xs font-medium text-foreground/80"
                >
                  {t("models.export_auth_token")}
                </Label>
                <Input
                  id="cc-export-auth-token"
                  placeholder={t("models.export_auth_token_placeholder")}
                  value={authToken}
                  onChange={(e) => setAuthToken(e.target.value)}
                  className="font-mono text-sm"
                />
              </div>
            </section>

            {/* Model tiers */}
            <section className="space-y-3">
              <div className="space-y-1">
                <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
                  {t("models.export_tiers")}
                </h3>
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
                        selected
                          ? accent.ring
                          : "border-border bg-transparent"
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
          </div>

          {/* ── Right: Claude-style code preview ── */}
          <div className="flex flex-col md:min-h-0 md:overflow-hidden bg-[#262624]">
            {/* Window toolbar */}
            <div className="shrink-0 flex items-center justify-between gap-3 border-b border-white/[0.07] bg-[#30302E] px-4 py-2.5">
              <div className="flex items-center gap-2 min-w-0">
                <Terminal className="size-3.5 shrink-0 text-white/35" />
                <span className="truncate font-mono text-xs text-white/50">
                  claude-code-setup.sh
                </span>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                {hasAnyTier && (
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
              {hasAnyTier ? (
                <pre className="px-5 py-4 text-[12.5px] leading-[1.65] text-[#E5E0D6]">
                  <code
                    className={`block font-mono whitespace-pre ${CLAUDE_SYNTAX}`}
                    dangerouslySetInnerHTML={{ __html: highlighted }}
                  />
                </pre>
              ) : (
                <div className="flex h-full min-h-[280px] flex-col items-center justify-center gap-4 px-6 py-16 text-center">
                  <span className="flex size-14 items-center justify-center rounded-2xl border border-white/[0.07] bg-white/[0.03]">
                    <ClaudeCode className="size-7 opacity-30" />
                  </span>
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-white/45">
                      {t("models.export_no_tier_selected")}
                    </p>
                    <p className="text-xs text-white/25">
                      {t("models.export_tier_empty_hint")}
                    </p>
                  </div>
                </div>
              )}
            </div>

            {/* Run hint footer */}
            {hasAnyTier && (
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
