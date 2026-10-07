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
import { Download, Loader2 } from "lucide-react";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api-client";
import type { PricingDTO } from "@/lib/types";
import { PricingEditor } from "./pricing-editor";

export interface PricingDialogProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  /** 目标模型别名（仅说明文案用） */
  alias: string;
  /** 目标模型上游真名：models.dev 导入的查询键 */
  upstreamModel: string;
  /** 目标模型上下文窗口：导入时用于展开末档区间上界 */
  contextLength: number;
  value: PricingDTO;
  onChange: (p: PricingDTO) => void;
  saving: boolean;
  onSave: () => void;
}

/**
 * 定价弹窗：与「编辑模型配置」平级的独立操作，只改 PATCH /model 的 pricing 字段
 * （缺省=不修改），因此不会把规格字段一并回写。
 *
 * 「从 models.dev 导入」是显式按钮：命中后整体替换规则表并自动进入已计价；
 * 未命中/无 cost/上游不可达只给内联提示，不动表单，也不弹全局错误。
 */
export function PricingDialog({
  open,
  onOpenChange,
  alias,
  upstreamModel,
  contextLength,
  value,
  onChange,
  saving,
  onSave,
}: PricingDialogProps) {
  const t = useT();
  const [importing, setImporting] = useState(false);
  const [importHint, setImportHint] = useState<"" | "filled" | "miss">("");

  const importPricing = async () => {
    const name = upstreamModel.trim();
    if (!name || importing) return;
    setImporting(true);
    try {
      const rsp = await api.prefillModelSpec(name, contextLength);
      const rules = rsp.pricing?.rules ?? [];
      if (!rsp.found || rules.length === 0) {
        setImportHint("miss");
        return;
      }
      onChange({ currency: "USD", rules });
      setImportHint("filled");
    } catch {
      setImportHint("miss");
    } finally {
      setImporting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {/* 规则条数多时弹窗可能超出视口：正文整体可滚，勿依赖 DialogContent 默认居中裁切 */}
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("upstream.pricing.title")}</DialogTitle>
          <DialogDescription className="min-h-[2.5rem]">
            {t("upstream.pricing.dialog_desc").replace("{alias}", alias)}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={!upstreamModel.trim() || importing}
            onClick={importPricing}
          >
            {importing ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Download className="size-4" />
            )}
            {t("upstream.pricing.fetch")}
          </Button>
          {importHint === "filled" && (
            <span className="text-[11px] text-muted-foreground">
              {t("upstream.pricing.fetch_filled")}
            </span>
          )}
          {importHint === "miss" && (
            <span className="text-[11px] text-muted-foreground">
              {t("upstream.pricing.fetch_miss")}
            </span>
          )}
        </div>

        <PricingEditor value={value} onChange={onChange} />

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button onClick={onSave} disabled={saving}>
            {saving ? t("common.saving") : t("common.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
