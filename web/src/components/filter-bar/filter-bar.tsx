"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Combobox as ComboboxPrimitive } from "@base-ui/react";
import { Search, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxItem,
  ComboboxList,
  useComboboxAnchor,
} from "@/components/ui/combobox";
import { Spinner } from "@/components/ui/spinner";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import type { FilterToken } from "./filter-dsl";
import type { FacetDef } from "./types";
import type { UseFilterBarReturn } from "./use-filter-bar";

export interface FilterBarProps extends Pick<
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

function rowKey(row: SuggestionRow) {
  if (row.kind === "facet") return `f:${row.facet.key}`;
  if (row.kind === "value") return `v:${row.facet.key}:${row.value}`;
  return `kw:${row.text}`;
}

/**
 * 筛选条：已选条件以标签形式内联在输入框前，建议弹层由 shadcn Combobox 承载
 * （弹层 Portal 到 body、键盘高亮与选中、点击外部收起都交给 Base UI）。
 *
 * 两步选择：先选 facet（输入变为 `key:`，弹层保持打开展示值建议），再选值生成标签；
 * 无匹配 facet 的输入作为关键词标签。Combobox 的 value 恒为 null，只把「选中事件」
 * 当作动作；输入框文本完全受控，只接受用户键入引起的变化。
 */
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
  const anchorRef = useComboboxAnchor();
  const inputRef = useRef<HTMLInputElement>(null);
  const highlighted = useRef<SuggestionRow | undefined>(undefined);
  const [input, setInput] = useState("");
  const [open, setOpen] = useState(false);
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

  const loading = loadingKey !== null && !!draft;

  const pick = (row: SuggestionRow) => {
    if (row.kind === "facet") {
      setInput(`${row.facet.key}:`);
    } else if (row.kind === "value") {
      addToken({ key: row.facet.key, value: row.value });
      setInput("");
    } else {
      addToken({ key: null, value: row.text });
      setInput("");
    }
    setOpen(true);
    inputRef.current?.focus();
  };

  // 方向键 / Enter（选中高亮项）/ Esc 由 Base UI 处理；这里只补筛选条特有的按键
  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Tab" && open && highlighted.current) {
      e.preventDefault();
      pick(highlighted.current);
      return;
    }
    if (e.key === "Enter" && rows.length === 0 && input.trim()) {
      e.preventDefault();
      pick({ kind: "keyword", text: input.trim() });
      return;
    }
    if (e.key === "Backspace" && !input && tokens.length > 0) {
      removeToken(tokens.length - 1);
    }
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

  const rowText = (row: SuggestionRow) => {
    if (row.kind === "facet") return row.facet.label;
    if (row.kind === "value")
      return row.facet.formatValue ? row.facet.formatValue(row.value) : row.value;
    return t("filter_bar.keyword_hint").replace("{text}", row.text);
  };

  return (
    <Combobox<SuggestionRow>
      items={rows}
      filter={null}
      value={null}
      onValueChange={(row) => {
        if (row) pick(row);
      }}
      inputValue={input}
      onInputValueChange={(value, details) => {
        // 选中、失焦等引起的回填/清空一律忽略：输入框只反映用户键入
        if (details.reason !== "input-change") return;
        setInput(value);
        setOpen(true);
      }}
      open={open && (rows.length > 0 || loading)}
      onOpenChange={(next, details) => {
        // 选中后保持打开：选完 facet 展示值建议，选完值继续展示 facet 便于连续添加条件
        if (!next && details.reason === "item-press") return;
        setOpen(next);
      }}
      onItemHighlighted={(row) => {
        highlighted.current = row ?? undefined;
      }}
      itemToStringLabel={rowText}
      autoHighlight
    >
      <div
        ref={anchorRef}
        data-slot="filter-bar"
        className={cn(
          "flex min-h-10 min-w-0 flex-1 cursor-text flex-wrap items-center gap-1.5 rounded-lg border border-input bg-background px-2.5 py-1.5 transition-colors focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30",
          className,
        )}
        onClick={() => inputRef.current?.focus()}
      >
        <Search className="size-4 shrink-0 text-muted-foreground" />
        {tokens.map((tk, i) => {
          const label = tokenLabel(tk);
          return (
            <Badge
              key={`${tk.key ?? "kw"}:${tk.value}:${i}`}
              variant="outline"
              className={cn("h-7 gap-1 pr-0.5", label.free && "border-dashed")}
            >
              <span className="text-muted-foreground">{label.k}</span>
              <TooltipRoot>
                <TooltipTrigger render={<span className="max-w-40 truncate">{label.v}</span>} />
                <TooltipContent side="top" className="max-w-xs break-all">
                  {label.v}
                </TooltipContent>
              </TooltipRoot>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                aria-label={t("filter_bar.remove_token")}
                className="rounded-full"
                onClick={(e) => {
                  e.stopPropagation();
                  removeToken(i);
                }}
              >
                <X />
              </Button>
            </Badge>
          );
        })}
        <ComboboxPrimitive.Input
          ref={inputRef}
          placeholder={tokens.length === 0 ? placeholder : undefined}
          autoComplete="off"
          spellCheck={false}
          className="min-w-32 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground/60"
          onFocus={() => setOpen(true)}
          onKeyDown={onKeyDown}
        />
        {tokens.length > 0 && (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            className="min-w-0"
            onClick={(e) => {
              e.stopPropagation();
              clearTokens();
              setInput("");
            }}
          >
            <X data-icon="inline-start" />
            {t("filter_bar.clear_all")}
          </Button>
        )}
      </div>
      <ComboboxContent anchor={anchorRef} className="w-72 data-[chips=true]:min-w-0">
        <div className="px-2 pt-2 pb-1 text-xs text-muted-foreground">
          {loading ? t("filter_bar.loading_options") : heading}
        </div>
        {loading ? (
          <div className="flex items-center gap-2 px-2 pb-2 text-xs text-muted-foreground">
            <Spinner />
            {t("filter_bar.loading_options")}
          </div>
        ) : (
          <>
            <ComboboxEmpty>{t("filter_bar.no_options")}</ComboboxEmpty>
            <ComboboxList>
              {(row: SuggestionRow) => (
                <ComboboxItem key={rowKey(row)} value={row}>
                  {row.kind === "facet" ? (
                    <>
                      <span className="font-medium">{row.facet.label}</span>
                      <span className="text-xs text-muted-foreground">{row.facet.key}:</span>
                    </>
                  ) : (
                    <TooltipRoot>
                      <TooltipTrigger render={<span className="truncate">{rowText(row)}</span>} />
                      <TooltipContent side="top" className="max-w-xs break-all">
                        {rowText(row)}
                      </TooltipContent>
                    </TooltipRoot>
                  )}
                </ComboboxItem>
              )}
            </ComboboxList>
          </>
        )}
      </ComboboxContent>
    </Combobox>
  );
}
