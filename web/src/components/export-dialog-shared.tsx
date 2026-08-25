"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode, type Ref } from "react";
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
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { useT } from "@/lib/i18n";
import { Check, Copy, Search, Terminal, X } from "lucide-react";

/* ─── 通用工具 ─── */

// 将 token 数格式化为紧凑可读形式：128000 -> 128K
export function formatTokens(n: number): string {
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

// 解析模型输入模态集合（能力）；空值/缺省兜底为 ["text"]，与服务端 DB 默认值一致
export function modelCapabilities(model: ModelItem): string[] {
  return model.capabilities && model.capabilities.length > 0 ? model.capabilities : ["text"];
}

// 按 alias / upstreamModel 过滤模型列表
export function useFilteredModels(models: ModelItem[], search: string): ModelItem[] {
  return useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return models;
    return models.filter(
      (m) => m.alias.toLowerCase().includes(q) || m.upstreamModel.toLowerCase().includes(q),
    );
  }, [models, search]);
}

/* ─── 脚本高亮 ─── */

hljs.registerLanguage("bash", bash);

function highlightBash(code: string): string {
  return hljs.highlight(code, { language: "bash" }).value;
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

/* ─── 对话框外壳：header + 左配置栏 + 右脚本预览 ─── */

interface ExportDialogShellProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** header 品牌图标 */
  icon: ReactNode;
  title: string;
  description: string;
  /** 预览窗口工具栏中的脚本文件名 */
  fileName: string;
  /** 生成的脚本；空字符串表示暂无可预览内容 */
  script: string;
  emptyIcon: ReactNode;
  emptyTitle: string;
  emptyHint: string;
  /** 阻塞预览的错误（如重复 alias）；存在时优先于脚本显示错误态 */
  errorMessage?: string | null;
  /** 左侧配置栏内容 */
  children: ReactNode;
}

export function ExportDialogShell({
  open,
  onOpenChange,
  icon,
  title,
  description,
  fileName,
  script,
  emptyIcon,
  emptyTitle,
  emptyHint,
  errorMessage,
  children,
}: ExportDialogShellProps) {
  const t = useT();
  const [copied, setCopied] = useState(false);

  const highlighted = useMemo(() => (script ? highlightBash(script) : ""), [script]);
  const lineCount = useMemo(() => (script ? script.split("\n").length : 0), [script]);

  // 统一拦截所有关闭路径（Esc / 外部点击 / 关闭按钮），关闭时重置复制态
  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      if (!nextOpen) setCopied(false);
      onOpenChange(nextOpen);
    },
    [onOpenChange],
  );

  const handleCopy = useCallback(async () => {
    if (!script) return;
    await navigator.clipboard.writeText(script);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [script]);

  const handleClose = useCallback(() => {
    handleOpenChange(false);
  }, [handleOpenChange]);

  const hasError = Boolean(errorMessage);
  const showPreview = script.length > 0 && !hasError;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        showCloseButton={false}
        className="!max-w-[1040px] w-[calc(100vw-1.5rem)] h-[min(86vh,720px)] p-0 gap-0 overflow-hidden flex flex-col sm:!max-w-[1040px]"
      >
        {/* ─── Header ─── */}
        <DialogHeader className="shrink-0 flex-row items-center gap-3 px-6 py-4 border-b border-border">
          <span className="flex size-9 items-center justify-center rounded-xl border border-border bg-gradient-to-b from-secondary to-muted shadow-sm">
            {icon}
          </span>
          <div className="flex flex-col gap-0.5 min-w-0">
            <DialogTitle className="font-display text-base leading-tight">{title}</DialogTitle>
            <DialogDescription className="min-h-[2.5rem] text-xs leading-snug">
              {description}
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
            {children}
          </div>

          {/* ── Right: Claude-style code preview ── */}
          <div className="flex flex-col md:min-h-0 md:overflow-hidden bg-[#262624]">
            {/* Window toolbar */}
            <div className="shrink-0 flex items-center justify-between gap-3 border-b border-white/[0.07] bg-[#30302E] px-4 py-2.5">
              <div className="flex items-center gap-2 min-w-0">
                <Terminal className="size-3.5 shrink-0 text-white/35" />
                <TooltipRoot>
                  <TooltipTrigger
                    render={
                      <span className="truncate font-mono text-xs text-white/50">{fileName}</span>
                    }
                  />
                  <TooltipContent side="top" className="max-w-xs break-all">
                    {fileName}
                  </TooltipContent>
                </TooltipRoot>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                {showPreview && (
                  <span className="hidden font-mono text-[10px] tabular-nums text-white/25 sm:inline">
                    {lineCount} {t("models.export_lines")}
                  </span>
                )}
                <button
                  type="button"
                  onClick={handleCopy}
                  disabled={!showPreview}
                  className="inline-flex h-7 min-w-20 items-center justify-center gap-1.5 rounded-md px-2.5 text-[11px] font-medium text-white/55 transition-colors hover:bg-white/[0.08] hover:text-white disabled:pointer-events-none disabled:opacity-30"
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
              {showPreview ? (
                <pre className="px-5 py-4 text-[12.5px] leading-[1.65] text-[#E5E0D6]">
                  <code
                    className={`block font-mono whitespace-pre ${CLAUDE_SYNTAX}`}
                    dangerouslySetInnerHTML={{ __html: highlighted }}
                  />
                </pre>
              ) : hasError ? (
                <div className="flex h-full min-h-[280px] flex-col items-center justify-center gap-4 px-6 py-16 text-center">
                  <span className="flex size-14 items-center justify-center rounded-2xl border border-destructive/20 bg-destructive/[0.06]">
                    <X className="size-7 text-destructive/60" />
                  </span>
                  <p className="max-w-sm text-sm font-medium text-white/55">{errorMessage}</p>
                </div>
              ) : (
                <div className="flex h-full min-h-[280px] flex-col items-center justify-center gap-4 px-6 py-16 text-center">
                  <span className="flex size-14 items-center justify-center rounded-2xl border border-white/[0.07] bg-white/[0.03]">
                    {emptyIcon}
                  </span>
                  <div className="space-y-1">
                    <p className="text-sm font-medium text-white/45">{emptyTitle}</p>
                    <p className="text-xs text-white/25">{emptyHint}</p>
                  </div>
                </div>
              )}
            </div>

            {/* Run hint footer */}
            {showPreview && (
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

/* ─── 左侧配置栏通用件 ─── */

// 表单字段：Label + 等宽字体 Input
interface ExportFieldProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

export function ExportField({ id, label, value, onChange, placeholder }: ExportFieldProps) {
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id} className="text-xs font-medium text-foreground/80">
        {label}
      </Label>
      <Input
        id={id}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="font-mono text-sm"
      />
    </div>
  );
}

// section 标题（大写小字）
export function ExportSectionTitle({ children }: { children: ReactNode }) {
  return (
    <h3 className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">
      {children}
    </h3>
  );
}

// section 标题内的选中计数徽标
export function ExportSelectionBadge({ count }: { count: number }) {
  return (
    <span className="ml-1.5 inline-flex items-center rounded-full bg-primary/10 px-1.5 py-px text-[10px] font-semibold text-primary tabular-nums normal-case tracking-normal">
      {count}
    </span>
  );
}

// section 标题右侧的轻量文字按钮（全选 / 清除）
interface ExportTextButtonProps {
  onClick: () => void;
  children: ReactNode;
}

export function ExportTextButton({ onClick, children }: ExportTextButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="min-w-14 text-[11px] font-medium text-primary/80 transition-colors hover:text-primary"
    >
      {children}
    </button>
  );
}

// 模型搜索框
interface ExportModelSearchProps {
  value: string;
  onChange: (value: string) => void;
  inputRef?: Ref<HTMLInputElement>;
}

export function ExportModelSearch({ value, onChange, inputRef }: ExportModelSearchProps) {
  const t = useT();
  return (
    <div className="relative">
      <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground/60" />
      <Input
        ref={inputRef}
        placeholder={t("models.search_placeholder")}
        aria-label={t("models.search_placeholder")}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="h-8 pl-8 text-sm"
      />
    </div>
  );
}

// 模型列表空态（无模型 / 搜索无匹配）
export function ExportModelEmpty({ searching }: { searching: boolean }) {
  const t = useT();
  return (
    <p className="rounded-lg border border-dashed border-border py-8 text-center text-xs text-muted-foreground">
      {searching ? t("models.export_no_matches") : t("models.no_models")}
    </p>
  );
}

/* ─── Connection section：Label + 等宽 Input 字段组 ─── */

interface ExportConnectionField {
  id: string;
  label: string;
  placeholder?: string;
  value: string;
  onChange: (value: string) => void;
}

// 连接信息字段组（providerId/baseUrl/apiKey 或 baseUrl/authToken 等，各对话框字段数量与 id 前缀不同）
export function ExportConnectionFields({ fields }: { fields: ExportConnectionField[] }) {
  const t = useT();
  return (
    <section className="space-y-4">
      <ExportSectionTitle>{t("models.export_connection")}</ExportSectionTitle>
      {fields.map((field) => (
        <ExportField
          key={field.id}
          id={field.id}
          label={field.label}
          placeholder={field.placeholder}
          value={field.value}
          onChange={field.onChange}
        />
      ))}
    </section>
  );
}

/* ─── 模型选择区（single / multi 通用） ─── */

interface ExportModelPickerBaseProps {
  models: ModelItem[];
  /** 对话框当前打开状态：关闭时重置搜索，打开时自动聚焦搜索框 */
  open: boolean;
  /** 模型区标题（已翻译文本） */
  title: string;
  /** ExportModelRow 的 maxOutputTokens 兜底值（Pi 为 16384） */
  outputFallback?: number;
  /** 列表下方附加内容（如 Pi 的重复 alias 错误提示） */
  children?: ReactNode;
}

interface ExportModelPickerSingleProps extends ExportModelPickerBaseProps {
  mode: "single";
  selectedId: number | null;
  onSelectedIdChange: (id: number | null) => void;
  /** 清除按钮文案（有选中时显示） */
  clearLabel: string;
}

interface ExportModelPickerMultiProps extends ExportModelPickerBaseProps {
  mode: "multi";
  selectedIds: Set<number>;
  onSelectedIdsChange: (ids: Set<number>) => void;
  selectAllLabel: string;
  clearAllLabel: string;
}

type ExportModelPickerProps = ExportModelPickerSingleProps | ExportModelPickerMultiProps;

export function ExportModelPicker(props: ExportModelPickerProps) {
  const searchInputRef = useRef<HTMLInputElement>(null);
  const [search, setSearch] = useState("");

  /* eslint-disable react-hooks/set-state-in-effect -- 关闭时重置搜索、打开时聚焦，与既有对话框行为一致 */
  useEffect(() => {
    if (!props.open) {
      setSearch("");
      return;
    }
    const timer = setTimeout(() => searchInputRef.current?.focus(), 120);
    return () => clearTimeout(timer);
  }, [props.open]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const filteredModels = useFilteredModels(props.models, search);

  if (props.mode === "single") {
    const { title, selectedId, onSelectedIdChange, clearLabel, outputFallback, children } = props;
    return (
      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <ExportSectionTitle>{title}</ExportSectionTitle>
          {selectedId !== null && (
            <ExportTextButton onClick={() => onSelectedIdChange(null)}>
              {clearLabel}
            </ExportTextButton>
          )}
        </div>
        <ExportModelSearch value={search} onChange={setSearch} inputRef={searchInputRef} />
        <div className="space-y-1">
          {filteredModels.length === 0 ? (
            <ExportModelEmpty searching={search.trim().length > 0} />
          ) : (
            filteredModels.map((model) => (
              <ExportModelRow
                key={model.id}
                model={model}
                selected={selectedId === model.id}
                onSelect={() => onSelectedIdChange(model.id)}
                outputFallback={outputFallback}
              />
            ))
          )}
        </div>
        {children}
      </section>
    );
  }

  const {
    title,
    selectedIds,
    onSelectedIdsChange,
    selectAllLabel,
    clearAllLabel,
    outputFallback,
    children,
  } = props;

  const allFilteredSelected =
    filteredModels.length > 0 && filteredModels.every((model) => selectedIds.has(model.id));

  const handleToggle = (id: number) => {
    const next = new Set(selectedIds);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    onSelectedIdsChange(next);
  };

  const handleToggleAll = () => {
    const next = new Set(selectedIds);
    const everySelected = filteredModels.every((model) => next.has(model.id));
    filteredModels.forEach((model) => (everySelected ? next.delete(model.id) : next.add(model.id)));
    onSelectedIdsChange(next);
  };

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between">
        <ExportSectionTitle>
          {title}
          {selectedIds.size > 0 && <ExportSelectionBadge count={selectedIds.size} />}
        </ExportSectionTitle>
        {filteredModels.length > 0 && (
          <ExportTextButton onClick={handleToggleAll}>
            {allFilteredSelected ? clearAllLabel : selectAllLabel}
          </ExportTextButton>
        )}
      </div>
      <ExportModelSearch value={search} onChange={setSearch} inputRef={searchInputRef} />
      <div className="space-y-1">
        {filteredModels.length === 0 ? (
          <ExportModelEmpty searching={search.trim().length > 0} />
        ) : (
          filteredModels.map((model) => (
            <ExportModelRow
              key={model.id}
              model={model}
              selected={selectedIds.has(model.id)}
              onSelect={() => handleToggle(model.id)}
              outputFallback={outputFallback}
            />
          ))
        )}
      </div>
      {children}
    </section>
  );
}

// 统一的模型选择行（单选 / 多选通用）
interface ExportModelRowProps {
  model: ModelItem;
  selected: boolean;
  onSelect: () => void;
  /** alias 旁的附加徽标（如 1M） */
  badge?: ReactNode;
  /** contextLength 缺失时的兜底值 */
  contextFallback?: number;
  /** maxOutputTokens 缺失时的兜底值 */
  outputFallback?: number;
}

export function ExportModelRow({
  model,
  selected,
  onSelect,
  badge,
  contextFallback = 128000,
  outputFallback = 64000,
}: ExportModelRowProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={`group flex w-full items-center gap-3 rounded-lg border px-3 py-2 text-left transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40 ${
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
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="flex items-center gap-1.5">
          <TooltipRoot>
            <TooltipTrigger
              render={
                <span className="truncate text-sm font-medium text-foreground">
                  {model.alias}
                </span>
              }
            />
            <TooltipContent side="top" className="max-w-xs break-all">
              {model.alias}
            </TooltipContent>
          </TooltipRoot>
          {badge}
        </span>
        <TooltipRoot>
          <TooltipTrigger
            render={
              <span className="truncate font-mono text-[11px] text-muted-foreground/70">
                {model.upstreamModel}
              </span>
            }
          />
          <TooltipContent side="top" className="max-w-xs break-all">
            {model.upstreamModel}
          </TooltipContent>
        </TooltipRoot>
      </span>
      <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted-foreground/60">
        {formatTokens(model.contextLength || contextFallback)}
        <span className="mx-0.5 opacity-50">/</span>
        {formatTokens(model.maxOutputTokens || outputFallback)}
      </span>
    </button>
  );
}
