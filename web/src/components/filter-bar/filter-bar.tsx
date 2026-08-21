"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Loader2, Search, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";
import type { FilterToken } from "./filter-dsl";
import type { FacetDef } from "./types";
import type { UseFilterBarReturn } from "./use-filter-bar";

export interface FilterBarProps
  extends Pick<
    UseFilterBarReturn,
    "tokens" | "addToken" | "removeToken" | "clearTokens" | "loadOptions" | "loadingKey"
  > {
  facets: FacetDef[];
  placeholder?: string;
  className?: string;
}

type SuggestionRow =
  | { kind: "facet"; facet: FacetDef }
  | { kind: "value"; facet: FacetDef; value: string }
  | { kind: "keyword"; text: string };

const COLON_RE = /[:：]/;

/** input 是否为「facet 前缀 + 冒号」的值编辑态；前缀按 key 或 label 匹配（首个命中） */
function matchDraftFacet(input: string, facets: FacetDef[]) {
  const match = input.match(COLON_RE);
  if (!match || match.index === undefined || match.index === 0) return null;
  const prefix = input.slice(0, match.index);
  const facet = facets.find((f) => f.key.startsWith(prefix) || f.label.startsWith(prefix));
  if (!facet) return null;
  return { facet, valueQuery: input.slice(match.index + 1) };
}

export function FilterBar({
  facets,
  tokens,
  addToken,
  removeToken,
  clearTokens,
  loadOptions,
  loadingKey,
  placeholder,
  className,
}: FilterBarProps) {
  const t = useT();
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const [valueOptions, setValueOptions] = useState<string[]>([]);

  const draft = matchDraftFacet(input, facets);
  const draftKey = draft?.facet.key ?? null;

  // 值编辑态变化时解析该 facet 的选项（静态直接给，异步经 loadOptions 缓存）
  /* eslint-disable react-hooks/set-state-in-effect -- 值建议选项随 draft facet 切换而解析，与 sessions 页 options 加载同模式 */
  useEffect(() => {
    if (!draft) {
      setValueOptions([]);
      return;
    }
    let cancelled = false;
    void loadOptions(draft.facet).then((items) => {
      if (!cancelled) setValueOptions(items);
    });
    return () => {
      cancelled = true;
    };
    /* eslint-disable-next-line react-hooks/exhaustive-deps -- 仅 draft facet 切换驱动；loadOptions 引用稳定 */
  }, [draftKey]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // 点击组件外收起建议
  useEffect(() => {
    const onDocMouseDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDocMouseDown);
    return () => document.removeEventListener("mousedown", onDocMouseDown);
  }, []);

  const { heading, rows } = useMemo((): { heading: string; rows: SuggestionRow[] } => {
    if (draft) {
      const q = draft.valueQuery.toLowerCase();
      const candidates = valueOptions
        .filter((v) => !q || v.toLowerCase().includes(q))
        .filter((v) => !tokens.some((tk) => tk.key === draft.facet.key && tk.value === v));
      return {
        heading: t("filter_bar.suggest_values").replace("{facet}", draft.facet.label),
        rows: candidates.map((v) => ({ kind: "value", facet: draft.facet, value: v })),
      };
    }
    const q = input.trim().toLowerCase();
    const matched = facets.filter(
      (f) => !q || f.key.toLowerCase().includes(q) || f.label.includes(input.trim()),
    );
    if (q && matched.length === 0) {
      return { heading: t("filter_bar.keyword"), rows: [{ kind: "keyword", text: input.trim() }] };
    }
    return {
      heading: t("filter_bar.suggest_facets"),
      rows: matched.map((f) => ({ kind: "facet", facet: f })),
    };
  }, [draft, valueOptions, tokens, facets, input, t]);

  const hl = Math.min(highlight, Math.max(rows.length - 1, 0));

  const pick = (row: SuggestionRow) => {
    if (row.kind === "facet") {
      setInput(`${row.facet.key}:`);
      setHighlight(0);
      inputRef.current?.focus();
      return;
    }
    if (row.kind === "value") {
      addToken({ key: row.facet.key, value: row.value });
    } else {
      addToken({ key: null, value: row.text });
    }
    setInput("");
    setHighlight(0);
    inputRef.current?.focus();
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (rows.length === 0) return;
      setOpen(true);
      setHighlight((hl + (e.key === "ArrowDown" ? 1 : -1) + rows.length) % rows.length);
      return;
    }
    if (e.key === "Enter" || e.key === "Tab") {
      if (open && rows.length > 0) {
        e.preventDefault();
        pick(rows[hl]);
      } else if (input.trim()) {
        e.preventDefault();
        pick({ kind: "keyword", text: input.trim() });
      }
      return;
    }
    if (e.key === "Backspace" && !input && tokens.length > 0) {
      removeToken(tokens.length - 1);
      return;
    }
    if (e.key === "Escape") setOpen(false);
  };

  const tokenLabel = (tk: FilterToken) => {
    if (tk.key === null) return { k: t("filter_bar.keyword"), v: `“${tk.value}”`, free: true };
    const facet = facets.find((f) => f.key === tk.key);
    return {
      k: facet?.label ?? tk.key,
      v: facet?.formatValue ? facet.formatValue(tk.value) : tk.value,
      free: false,
    };
  };

  return (
    <div ref={rootRef} className={cn("relative min-w-0 flex-1", className)}>
      <div
        className="flex min-h-10 cursor-text flex-wrap items-center gap-1.5 rounded-lg border border-input bg-background px-2.5 py-1.5 transition-colors focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30"
        onClick={() => inputRef.current?.focus()}
      >
        <Search className="size-4 shrink-0 text-muted-foreground" />
        {tokens.map((tk, i) => {
          const label = tokenLabel(tk);
          return (
            <span
              key={`${tk.key ?? "kw"}:${tk.value}:${i}`}
              className={cn(
                "inline-flex h-7 items-center rounded-md border text-xs",
                label.free
                  ? "border-dashed border-input bg-transparent"
                  : "border-ring/40 bg-accent/60",
              )}
            >
              <span className="pl-2 pr-1 text-muted-foreground">{label.k}</span>
              <span className="max-w-40 truncate pr-1 font-medium">{label.v}</span>
              <button
                type="button"
                aria-label={t("filter_bar.remove_token")}
                className="mr-0.5 rounded p-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-destructive"
                onClick={(e) => {
                  e.stopPropagation();
                  removeToken(i);
                }}
              >
                <X className="size-3" />
              </button>
            </span>
          );
        })}
        <input
          ref={inputRef}
          value={input}
          placeholder={tokens.length === 0 ? placeholder : undefined}
          autoComplete="off"
          spellCheck={false}
          className="min-w-32 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
          onChange={(e) => {
            setInput(e.target.value);
            setHighlight(0);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
        />
        {tokens.length > 0 && (
          <button
            type="button"
            aria-label={t("filter_bar.clear_all")}
            className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md px-1.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-destructive"
            onClick={(e) => {
              e.stopPropagation();
              clearTokens();
              setInput("");
            }}
          >
            <X className="size-3" />
            {t("filter_bar.clear_all")}
          </button>
        )}
      </div>

      {open && (rows.length > 0 || loadingKey !== null) && (
        <div className="absolute left-0 top-full z-50 mt-1.5 w-72 max-w-[calc(100vw-3rem)] rounded-lg border bg-popover p-1 text-sm shadow-md">
          <div className="px-2 pb-1 pt-1.5 text-[0.65rem] font-medium uppercase tracking-wider text-muted-foreground">
            {loadingKey !== null && draft ? t("filter_bar.loading_options") : heading}
          </div>
          {loadingKey !== null && draft ? (
            <div className="flex items-center gap-2 px-2 py-2 text-xs text-muted-foreground">
              <Loader2 className="size-3.5 animate-spin" />
              {t("filter_bar.loading_options")}
            </div>
          ) : rows.length === 0 ? (
            <div className="px-2 py-2 text-xs text-muted-foreground">
              {t("filter_bar.no_options")}
            </div>
          ) : (
            rows.map((row, i) => (
              <button
                key={
                  row.kind === "facet"
                    ? `f:${row.facet.key}`
                    : row.kind === "value"
                      ? `v:${row.facet.key}:${row.value}`
                      : `kw:${row.text}`
                }
                type="button"
                role="option"
                aria-selected={i === hl}
                className={cn(
                  "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left hover:bg-accent",
                  i === hl && "bg-accent",
                )}
                onMouseDown={(e) => {
                  e.preventDefault();
                  pick(row);
                }}
                onMouseEnter={() => setHighlight(i)}
              >
                {row.kind === "facet" && (
                  <>
                    <span className="font-medium">{row.facet.label}</span>
                    <span className="text-xs text-muted-foreground">{row.facet.key}:</span>
                  </>
                )}
                {row.kind === "value" && (
                  <span className="truncate">
                    {row.facet.formatValue ? row.facet.formatValue(row.value) : row.value}
                  </span>
                )}
                {row.kind === "keyword" && (
                  <span className="truncate">
                    {t("filter_bar.keyword_hint").replace("{text}", row.text)}
                  </span>
                )}
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}
