"use client";

import { useEffect, useRef, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
  PopoverHeader,
  PopoverTitle,
  PopoverDescription,
} from "@/components/ui/popover";
import {
  AudioLines,
  FileText,
  SlidersHorizontal,
  Type,
  Video,
  Image as ImageIcon,
  type LucideIcon,
} from "lucide-react";
import { PricingEditor } from "./pricing-editor";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api-client";
import { cn } from "@/lib/utils";
import type { ModelCapability } from "@/lib/types";
import { DEFAULT_CONTEXT_LENGTH, DEFAULT_MAX_OUTPUT, formatTokens, type ModelForm } from "./shared";

// 输入模态 chips：text 必选（保存校验的 UI 呈现），其余点击切换
const CAPABILITY_CHIPS: { value: ModelCapability; labelKey: string; Icon: LucideIcon }[] = [
  { value: "text", labelKey: "models.capability_text", Icon: Type },
  { value: "image", labelKey: "models.capability_image", Icon: ImageIcon },
  { value: "pdf", labelKey: "models.capability_pdf", Icon: FileText },
  { value: "video", labelKey: "models.capability_video", Icon: Video },
  { value: "audio", labelKey: "models.capability_audio", Icon: AudioLines },
];

// 常用 token 预设档位：点击即写入表单，替代上下箭头微调
const CONTEXT_LENGTH_PRESETS = [256_000, 512_000, 1_000_000];
const MAX_OUTPUT_PRESETS = [4_096, 8_192, 16_384, 32_768, 65_536, 131_072];

interface TokenPresetPopoverProps {
  label: string;
  description: string;
  value: number;
  presets: number[];
  onSelect: (v: number) => void;
}

// 预设值 Popover：锚定在输入框旁，点选预设即写入表单并关闭；当前值高亮为主色，其余为 outline
function TokenPresetPopover({
  label,
  description,
  value,
  presets,
  onSelect,
}: TokenPresetPopoverProps) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={<Button type="button" variant="outline" size="icon" aria-label={label} />}
      >
        <SlidersHorizontal className="size-4" />
      </PopoverTrigger>
      <PopoverContent align="end" sideOffset={8} className="w-64 p-2.5">
        <PopoverHeader className="px-0.5 pb-1">
          <PopoverTitle className="text-xs">{label}</PopoverTitle>
          <PopoverDescription className="text-[11px]">{description}</PopoverDescription>
        </PopoverHeader>
        <div className="grid grid-cols-3 gap-1.5">
          {presets.map((preset) => {
            const active = preset === value;
            return (
              <Button
                key={preset}
                type="button"
                size="sm"
                variant={active ? "default" : "outline"}
                aria-pressed={active}
                className="font-mono tabular-nums"
                onClick={() => {
                  onSelect(preset);
                  setOpen(false);
                }}
              >
                {formatTokens(preset)}
              </Button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export interface ModelDialogProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  editing: boolean;
  form: ModelForm;
  setForm: React.Dispatch<React.SetStateAction<ModelForm>>;
  onModelIdTouched: () => void;
  modelIdTouched: boolean;
  /** 仅编辑模式且 modelId 相对原值变化时为 true（此时展示同步历史开关） */
  showSyncHistory: boolean;
  syncHistory: boolean;
  onSyncHistoryChange: (v: boolean) => void;
  saving: boolean;
  onSave: () => void;
}

/** 模型新建/编辑弹窗：无 endpoint 选择器（绑定不可移动） */
export function ModelDialog({
  open,
  onOpenChange,
  editing,
  form,
  setForm,
  onModelIdTouched,
  modelIdTouched,
  showSyncHistory,
  syncHistory,
  onSyncHistoryChange,
  saving,
  onSave,
}: ModelDialogProps) {
  const t = useT();
  const [dirty, setDirty] = useState<ReadonlySet<string>>(new Set());
  // 提示绑定触发时的模型名：改名后提示自然失效，无需在 effect 里清理状态
  const [autofillHint, setAutofillHint] = useState<{
    kind: "filled" | "miss";
    name: string;
  } | null>(null);
  const [pricingOpen, setPricingOpen] = useState(false);
  const dirtyRef = useRef<ReadonlySet<string>>(new Set());
  const formRef = useRef(form);
  const lastQueried = useRef("");

  const markDirty = (field: string) => setDirty((d) => new Set(d).add(field));

  // ref 同步放 effect 内（render 期禁止读写 ref）；本地状态由父组件 key 重挂载在每次打开时重置
  useEffect(() => {
    dirtyRef.current = dirty;
    formRef.current = form;
  });

  // 上游模型名防抖 600ms 自动填充：只填未手改字段；同值短路；旧响应丢弃；零 toast
  useEffect(() => {
    const name = form.upstreamModel.trim();
    if (!name || name === lastQueried.current) return;
    let cancelled = false;
    const timer = setTimeout(async () => {
      lastQueried.current = name;
      try {
        const rsp = await api.prefillModelSpec(name, formRef.current.contextLength || undefined);
        if (cancelled || formRef.current.upstreamModel.trim() !== name) return; // 竞态守卫
        if (!rsp.found) {
          setAutofillHint({ kind: "miss", name });
          return;
        }
        const willFillPricing = !dirtyRef.current.has("pricing") && !!rsp.pricing?.rules?.length;
        setForm((f) => {
          const next = { ...f };
          const d = dirtyRef.current;
          if (!d.has("contextLength") && rsp.contextLength) next.contextLength = rsp.contextLength;
          if (!d.has("maxOutputTokens") && rsp.maxOutputTokens)
            next.maxOutputTokens = rsp.maxOutputTokens;
          if (!d.has("capabilities") && rsp.capabilities?.length)
            next.capabilities = rsp.capabilities;
          if (!d.has("pricing") && rsp.pricing?.rules?.length)
            next.pricing = { currency: "USD", rules: rsp.pricing.rules };
          return next;
        });
        setAutofillHint({ kind: "filled", name });
        if (willFillPricing) setPricingOpen(true); // 定价自动回填后展开折叠卡供核对
      } catch {
        // 自动触发路径静默降级：接口报错按未命中处理
        if (!cancelled && formRef.current.upstreamModel.trim() === name) {
          setAutofillHint({ kind: "miss", name });
        }
      }
    }, 600);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [form.upstreamModel, setForm]);

  const hint =
    autofillHint && autofillHint.name === form.upstreamModel.trim() ? autofillHint.kind : "";
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{editing ? t("models.edit") : t("models.create")}</DialogTitle>
          <DialogDescription className="min-h-[2.5rem]">
            {editing ? t("models.edit_desc") : t("upstream.create_model_desc")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-1">
            <Label htmlFor="model-alias">{t("models.alias")}</Label>
            <Input
              id="model-alias"
              placeholder={t("models.alias_placeholder")}
              value={form.alias}
              onChange={(e) => {
                const alias = e.target.value;
                setForm((f) => ({
                  ...f,
                  alias,
                  // 未手动改过 modelId 时新建表单跟随 alias 同步输入
                  ...(modelIdTouched ? {} : { modelId: alias }),
                }));
              }}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="model-id">{t("models.model_id")}</Label>
            <Input
              id="model-id"
              placeholder={t("models.model_id_placeholder")}
              value={form.modelId}
              onChange={(e) => {
                onModelIdTouched();
                setForm((f) => ({ ...f, modelId: e.target.value }));
              }}
            />
            <p className="text-[11px] text-muted-foreground">{t("models.model_id_hint")}</p>
          </div>
          {showSyncHistory && (
            <div className="space-y-1 rounded-lg border border-input px-3 py-2">
              <div className="flex items-center justify-between">
                <span className="text-sm">{t("models.sync_history")}</span>
                <Switch size="sm" checked={syncHistory} onCheckedChange={onSyncHistoryChange} />
              </div>
              <p className="text-[11px] text-muted-foreground">{t("models.sync_history_desc")}</p>
            </div>
          )}
          <div className="space-y-1">
            <Label htmlFor="model-upstream">{t("models.upstream_model")}</Label>
            <Input
              id="model-upstream"
              placeholder={t("models.upstream_model_placeholder")}
              value={form.upstreamModel}
              onChange={(e) => setForm((f) => ({ ...f, upstreamModel: e.target.value }))}
            />
            {hint === "filled" && (
              <p className="text-[11px] text-muted-foreground">{t("models.autofill_filled")}</p>
            )}
            {hint === "miss" && (
              <p className="text-[11px] text-muted-foreground">{t("models.autofill_miss")}</p>
            )}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1">
              <Label htmlFor="model-context-length">{t("models.context_length")}</Label>
              <div className="flex gap-2">
                <Input
                  id="model-context-length"
                  type="number"
                  min={0}
                  step={1000}
                  inputMode="numeric"
                  placeholder={String(DEFAULT_CONTEXT_LENGTH)}
                  className="[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                  value={form.contextLength || ""}
                  onChange={(e) => {
                    markDirty("contextLength");
                    setForm((f) => ({ ...f, contextLength: Number(e.target.value) || 0 }));
                  }}
                />
                <TokenPresetPopover
                  label={t("models.context_length_presets")}
                  description={t("models.preset_desc")}
                  value={form.contextLength}
                  presets={CONTEXT_LENGTH_PRESETS}
                  onSelect={(v) => {
                    markDirty("contextLength");
                    setForm((f) => ({ ...f, contextLength: v }));
                  }}
                />
              </div>
            </div>
            <div className="space-y-1">
              <Label htmlFor="model-max-output">{t("models.max_output")}</Label>
              <div className="flex gap-2">
                <Input
                  id="model-max-output"
                  type="number"
                  min={0}
                  step={1000}
                  inputMode="numeric"
                  placeholder={String(DEFAULT_MAX_OUTPUT)}
                  className="[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                  value={form.maxOutputTokens || ""}
                  onChange={(e) => {
                    markDirty("maxOutputTokens");
                    setForm((f) => ({ ...f, maxOutputTokens: Number(e.target.value) || 0 }));
                  }}
                />
                <TokenPresetPopover
                  label={t("models.max_output_presets")}
                  description={t("models.preset_desc")}
                  value={form.maxOutputTokens}
                  presets={MAX_OUTPUT_PRESETS}
                  onSelect={(v) => {
                    markDirty("maxOutputTokens");
                    setForm((f) => ({ ...f, maxOutputTokens: v }));
                  }}
                />
              </div>
            </div>
          </div>
          <div className="space-y-1">
            <Label>{t("models.capabilities")}</Label>
            <div className="flex flex-wrap gap-1.5">
              {CAPABILITY_CHIPS.map(({ value, labelKey, Icon }) => {
                const checked = form.capabilities.includes(value);
                // text 为必选模态（保存校验的 UI 呈现），不可取消
                const locked = value === "text";
                return (
                  <Button
                    key={value}
                    type="button"
                    size="sm"
                    variant="outline"
                    aria-pressed={checked}
                    disabled={locked}
                    className={cn(
                      checked && "border-primary/40 bg-primary/10 text-primary hover:bg-primary/15",
                    )}
                    onClick={() => {
                      markDirty("capabilities");
                      setForm((f) => ({
                        ...f,
                        capabilities: checked
                          ? f.capabilities.filter((c) => c !== value)
                          : [...f.capabilities, value],
                      }));
                    }}
                  >
                    <Icon className="size-3.5" />
                    {t(labelKey)}
                  </Button>
                );
              })}
            </div>
          </div>
        </div>
        <PricingEditor
          value={form.pricing}
          open={pricingOpen}
          onOpenChange={setPricingOpen}
          onChange={(p) => {
            markDirty("pricing");
            setForm((f) => ({ ...f, pricing: p }));
          }}
        />

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button
            onClick={onSave}
            disabled={!form.alias.trim() || !form.upstreamModel.trim() || saving}
          >
            {saving ? t("common.saving") : editing ? t("common.update") : t("common.create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
