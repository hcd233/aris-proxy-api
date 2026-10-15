"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Input } from "@/components/ui/input";
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
  FieldTitle,
} from "@/components/ui/field";
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
  Download,
  FileText,
  SlidersHorizontal,
  Type,
  Video,
  Image as ImageIcon,
  type LucideIcon,
} from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api-client";
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

// text 为必选模态：models.dev 命中结果缺 text 时补上，避免把表单推进保存校验失败的态
function ensureText(caps: ModelCapability[]): ModelCapability[] {
  return caps.includes("text") ? caps : [...caps, "text"];
}

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
        <ToggleGroup
          variant="outline"
          size="sm"
          spacing={1.5}
          className="grid w-full grid-cols-3"
          value={[String(value)]}
          onValueChange={(next) => {
            if (next[0]) onSelect(Number(next[0]));
            setOpen(false);
          }}
        >
          {presets.map((preset) => (
            <ToggleGroupItem key={preset} value={String(preset)} className="font-mono tabular-nums">
              {formatTokens(preset)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
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

/** 模型新建/编辑弹窗：无 endpoint 选择器（绑定不可移动）；定价已拆分为同级操作，不在此弹窗内 */
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
  const [fetching, setFetching] = useState(false);
  // 提示绑定触发时的模型名：改名后提示自然失效，无需在 effect 里清理状态
  const [autofillHint, setAutofillHint] = useState<{
    kind: "filled" | "miss";
    name: string;
  } | null>(null);

  /**
   * 显式触发规格导入：命中即覆盖上下文 / 最大输出 / 模态三件套。
   * 按钮是用户主动点击，不存在「自动覆盖手填值」的顾虑，故不做 dirty 保护；
   * 未命中或上游不可达只给内联提示，表单不动（不弹全局错误）。
   */
  const fetchSpec = async () => {
    const name = form.upstreamModel.trim();
    if (!name || fetching) return;
    setFetching(true);
    try {
      const rsp = await api.prefillModelSpec(name);
      if (!rsp.found) {
        setAutofillHint({ kind: "miss", name });
        return;
      }
      setForm((f) => ({
        ...f,
        contextLength: rsp.contextLength || f.contextLength,
        maxOutputTokens: rsp.maxOutputTokens || f.maxOutputTokens,
        capabilities: rsp.capabilities?.length ? ensureText([...rsp.capabilities]) : f.capabilities,
      }));
      setAutofillHint({ kind: "filled", name });
    } catch {
      setAutofillHint({ kind: "miss", name });
    } finally {
      setFetching(false);
    }
  };

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
        <FieldGroup className="gap-4">
          <Field>
            <FieldLabel htmlFor="model-alias">{t("models.alias")}</FieldLabel>
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
          </Field>
          <Field>
            <FieldLabel htmlFor="model-id">{t("models.model_id")}</FieldLabel>
            <Input
              id="model-id"
              placeholder={t("models.model_id_placeholder")}
              value={form.modelId}
              onChange={(e) => {
                onModelIdTouched();
                setForm((f) => ({ ...f, modelId: e.target.value }));
              }}
            />
            <FieldDescription>{t("models.model_id_hint")}</FieldDescription>
          </Field>
          {showSyncHistory && (
            <FieldLabel htmlFor="model-sync-history">
              <Field orientation="horizontal">
                <FieldContent>
                  <FieldTitle>{t("models.sync_history")}</FieldTitle>
                  <FieldDescription>{t("models.sync_history_desc")}</FieldDescription>
                </FieldContent>
                <Switch
                  id="model-sync-history"
                  size="sm"
                  checked={syncHistory}
                  onCheckedChange={onSyncHistoryChange}
                />
              </Field>
            </FieldLabel>
          )}
          <Field>
            <FieldLabel htmlFor="model-upstream">{t("models.upstream_model")}</FieldLabel>
            <Input
              id="model-upstream"
              placeholder={t("models.upstream_model_placeholder")}
              value={form.upstreamModel}
              onChange={(e) => setForm((f) => ({ ...f, upstreamModel: e.target.value }))}
            />
          </Field>
          {/* 规格导入入口 + 结果提示同一行（按钮产出的是下方规格三件套，故紧邻其上方） */}
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={!form.upstreamModel.trim() || fetching}
              onClick={fetchSpec}
            >
              {fetching ? (
                <Spinner data-icon="inline-start" />
              ) : (
                <Download data-icon="inline-start" />
              )}
              {t("models.spec_fetch")}
            </Button>
            {hint === "filled" && (
              <span className="text-[11px] text-muted-foreground">
                {t("models.autofill_filled")}
              </span>
            )}
            {hint === "miss" && (
              <span className="text-[11px] text-muted-foreground">{t("models.autofill_miss")}</span>
            )}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Field>
              <FieldLabel htmlFor="model-context-length">{t("models.context_length")}</FieldLabel>
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
                  onChange={(e) =>
                    setForm((f) => ({ ...f, contextLength: Number(e.target.value) || 0 }))
                  }
                />
                <TokenPresetPopover
                  label={t("models.context_length_presets")}
                  description={t("models.preset_desc")}
                  value={form.contextLength}
                  presets={CONTEXT_LENGTH_PRESETS}
                  onSelect={(v) => setForm((f) => ({ ...f, contextLength: v }))}
                />
              </div>
            </Field>
            <Field>
              <FieldLabel htmlFor="model-max-output">{t("models.max_output")}</FieldLabel>
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
                  onChange={(e) =>
                    setForm((f) => ({ ...f, maxOutputTokens: Number(e.target.value) || 0 }))
                  }
                />
                <TokenPresetPopover
                  label={t("models.max_output_presets")}
                  description={t("models.preset_desc")}
                  value={form.maxOutputTokens}
                  presets={MAX_OUTPUT_PRESETS}
                  onSelect={(v) => setForm((f) => ({ ...f, maxOutputTokens: v }))}
                />
              </div>
            </Field>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Field>
              <FieldLabel htmlFor="model-priority">{t("models.priority")}</FieldLabel>
              <Input
                id="model-priority"
                type="number"
                min={-100}
                max={100}
                inputMode="numeric"
                className="[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                value={form.priority}
                onChange={(e) => setForm((f) => ({ ...f, priority: Number(e.target.value) || 0 }))}
              />
              <FieldDescription>{t("models.priority_hint")}</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="model-weight">{t("models.weight")}</FieldLabel>
              <Input
                id="model-weight"
                type="number"
                min={1}
                max={1000}
                inputMode="numeric"
                className="[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                value={form.weight}
                onChange={(e) => setForm((f) => ({ ...f, weight: Number(e.target.value) || 1 }))}
              />
              <FieldDescription>{t("models.weight_hint")}</FieldDescription>
            </Field>
          </div>
          <FieldSet>
            <FieldLegend variant="label">{t("models.capabilities")}</FieldLegend>
            <ToggleGroup
              multiple
              variant="outline"
              size="sm"
              spacing={1.5}
              className="flex-wrap"
              value={form.capabilities}
              onValueChange={(next) =>
                setForm((f) => ({ ...f, capabilities: next as typeof f.capabilities }))
              }
            >
              {CAPABILITY_CHIPS.map(({ value, labelKey, Icon }) => (
                // text 为必选模态（保存校验的 UI 呈现），不可取消
                <ToggleGroupItem key={value} value={value} disabled={value === "text"}>
                  <Icon data-icon="inline-start" />
                  {t(labelKey)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </FieldSet>
        </FieldGroup>

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
