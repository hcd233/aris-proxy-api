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
import { Type, Image as ImageIcon, SlidersHorizontal } from "lucide-react";
import { useT } from "@/lib/i18n";
import { formatTokens, type ModelForm } from "./shared";

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
                  placeholder="256000"
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
                  placeholder="65536"
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
            </div>
          </div>
          <div className="space-y-1">
            <Label>{t("models.capabilities")}</Label>
            <div className="grid grid-cols-2 gap-3">
              <div className="flex items-center justify-between rounded-lg border border-input px-3 py-2">
                <span className="flex items-center gap-1.5 text-sm">
                  <Type className="size-3.5 text-muted-foreground" />
                  {t("models.capability_text")}
                </span>
                <Switch
                  size="sm"
                  checked={form.supportText}
                  onCheckedChange={(v) => setForm((f) => ({ ...f, supportText: v }))}
                />
              </div>
              <div className="flex items-center justify-between rounded-lg border border-input px-3 py-2">
                <span className="flex items-center gap-1.5 text-sm">
                  <ImageIcon className="size-3.5 text-muted-foreground" />
                  {t("models.capability_image")}
                </span>
                <Switch
                  size="sm"
                  checked={form.supportImage}
                  onCheckedChange={(v) => setForm((f) => ({ ...f, supportImage: v }))}
                />
              </div>
            </div>
          </div>
        </div>
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
