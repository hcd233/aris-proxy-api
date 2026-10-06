"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Download, Plus, Trash2, ArrowUp, ArrowDown } from "lucide-react";
import { toast } from "sonner";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api-client";
import type { PricingDTO, PricingRuleDTO, PricingCurrency, TimeWindowDTO } from "@/lib/types";

export interface PricingEditorProps {
  value?: PricingDTO;
  onChange: (p: PricingDTO | undefined) => void;
  /** 用于导入的上游模型名（表单 upstreamModel 字段） */
  upstreamModel: string;
}

const WEEKDAYS = [1, 2, 3, 4, 5, 6, 7];
// ponytail: 时区固定两选项（UTC/Asia/Shanghai），升级路径：接 IANA 全量列表选择器
const TIMEZONE_OPTIONS = ["UTC", "Asia/Shanghai"];

const DEFAULT_WINDOW = (): TimeWindowDTO => ({
  days: [],
  start: "00:30",
  end: "08:30",
  timezone: "UTC",
});

const emptyRule = (): PricingRuleDTO => ({
  time_windows: [],
  context_min: 0,
  context_max: 0,
  input_price: 0,
  output_price: 0,
  cache_creation_price: 0,
  cache_read_price: 0,
});

const isDefaultRule = (r: PricingRuleDTO): boolean =>
  (r.time_windows ?? []).length === 0 && !r.context_min && !r.context_max;

const timeEnabled = (r: PricingRuleDTO): boolean => (r.time_windows ?? []).length > 0;
const contextEnabled = (r: PricingRuleDTO): boolean =>
  (r.context_min ?? 0) > 0 || (r.context_max ?? 0) > 0;

function priceOrZero(v: string): number {
  const n = Number(v);
  return Number.isFinite(n) && n >= 0 ? n : 0;
}

/**
 * PricingEditor 定价规则编辑器。
 * - 币种下拉（无 / CNY / USD）；币种为空 = 未计价（规则区隐藏）
 * - 每条规则两个默认关闭的选项：「时段限制」勾选后编辑时段窗口；「上下文区间」勾选后编辑 min/max
 * - 行操作：增、删、上移/下移（数组顺序 = 匹配优先级）；无条件默认规则行标注「默认」
 * - 「从 models.dev 导入」整体填入定价（含上下文区间规则；时段窗口无数据需手填）
 */
export function PricingEditor({ value, onChange, upstreamModel }: PricingEditorProps) {
  const t = useT();
  const [importing, setImporting] = useState(false);
  const currency = value?.currency ?? "";
  const rules = value?.rules ?? [];

  const emit = (nextCurrency: PricingCurrency, nextRules: PricingRuleDTO[]) => {
    if (nextCurrency === "" && nextRules.length === 0) {
      onChange(undefined);
      return;
    }
    onChange({ currency: nextCurrency, rules: nextRules });
  };

  const patchRule = (idx: number, patch: Partial<PricingRuleDTO>) => {
    const next = rules.map((r, i) => (i === idx ? { ...r, ...patch } : r));
    emit(currency, next);
  };

  const moveRule = (idx: number, delta: number) => {
    const j = idx + delta;
    if (j < 0 || j >= rules.length) return;
    const next = [...rules];
    [next[idx], next[j]] = [next[j], next[idx]];
    emit(currency, next);
  };

  const handlePrefill = async () => {
    if (!upstreamModel.trim()) {
      toast.error(t("upstream.pricing.prefill.miss"));
      return;
    }
    setImporting(true);
    try {
      const rsp = await api.prefillModelPricing(upstreamModel.trim());
      if (!rsp.found || !rsp.pricing) {
        toast.error(t("upstream.pricing.prefill.miss"));
        return;
      }
      emit((rsp.currency as PricingCurrency) ?? "USD", rsp.pricing.rules ?? []);
      toast.success(t("upstream.pricing.prefill.hint"));
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Label className="w-28 shrink-0">{t("upstream.pricing.currency")}</Label>
        <Select
          value={currency}
          onValueChange={(v) => {
            const nextCurrency = (v ?? "") as PricingCurrency;
            if (nextCurrency === "") {
              emit("", []);
              return;
            }
            const next = rules.length > 0 ? rules : [{ ...emptyRule() }];
            emit(nextCurrency, next);
          }}
        >
          <SelectTrigger className="w-40">
            <SelectValue placeholder={t("upstream.pricing.currency.none")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">{t("upstream.pricing.currency.none")}</SelectItem>
            <SelectItem value="CNY">CNY</SelectItem>
            <SelectItem value="USD">USD</SelectItem>
          </SelectContent>
        </Select>
        {currency !== "" && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handlePrefill}
            disabled={importing}
          >
            <Download className="size-4" />
            {t("upstream.pricing.prefill")}
          </Button>
        )}
      </div>

      {currency !== "" && (
        <div className="space-y-2">
          {rules.map((rule, idx) => (
            <div key={idx} className="rounded-md border p-2.5 space-y-2">
              <div className="flex items-center gap-1.5">
                <span className="text-xs font-medium text-muted-foreground">#{idx + 1}</span>
                {isDefaultRule(rule) && (
                  <span className="rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary">
                    {t("upstream.pricing.rule.default")}
                  </span>
                )}
                <div className="ml-auto flex items-center gap-1">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={t("upstream.pricing.rule.moveUp")}
                    onClick={() => moveRule(idx, -1)}
                  >
                    <ArrowUp className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={t("upstream.pricing.rule.moveDown")}
                    onClick={() => moveRule(idx, 1)}
                  >
                    <ArrowDown className="size-4" />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={t("common.delete")}
                    onClick={() =>
                      emit(
                        currency,
                        rules.filter((_, i) => i !== idx),
                      )
                    }
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-4">
                <label className="flex items-center gap-2 text-xs">
                  <Switch
                    checked={timeEnabled(rule)}
                    onCheckedChange={(checked) =>
                      patchRule(idx, {
                        time_windows: checked ? [DEFAULT_WINDOW()] : [],
                      })
                    }
                  />
                  {t("upstream.pricing.opt.timeWindow")}
                </label>
                <label className="flex items-center gap-2 text-xs">
                  <Switch
                    checked={contextEnabled(rule)}
                    onCheckedChange={(checked) =>
                      patchRule(idx, {
                        context_min: 0,
                        context_max: checked ? 200_000 : 0,
                      })
                    }
                  />
                  {t("upstream.pricing.opt.context")}
                </label>
              </div>

              {timeEnabled(rule) &&
                (rule.time_windows ?? []).map((w, wi) => (
                  <div key={wi} className="flex flex-wrap items-center gap-1.5">
                    <div className="flex gap-1">
                      {WEEKDAYS.map((d) => {
                        const active = (w.days ?? []).length === 0 || (w.days ?? []).includes(d);
                        return (
                          <Button
                            key={d}
                            type="button"
                            size="sm"
                            variant={active ? "default" : "outline"}
                            className="h-7 w-7 p-0"
                            onClick={() => {
                              const cur = w.days ?? [];
                              // 空=每天；点选后进入显式选择模式
                              const base = cur.length === 0 ? WEEKDAYS : cur;
                              const nextDays = base.includes(d)
                                ? base.filter((x) => x !== d)
                                : [...base, d].sort((a, b) => a - b);
                              const windows = (rule.time_windows ?? []).map((x, xi) =>
                                xi === wi ? { ...x, days: nextDays } : x,
                              );
                              patchRule(idx, { time_windows: windows });
                            }}
                          >
                            {d}
                          </Button>
                        );
                      })}
                    </div>
                    <Input
                      className="w-24"
                      value={w.start}
                      placeholder="00:30"
                      onChange={(e) => {
                        const windows = (rule.time_windows ?? []).map((x, xi) =>
                          xi === wi ? { ...x, start: e.target.value } : x,
                        );
                        patchRule(idx, { time_windows: windows });
                      }}
                    />
                    <span className="text-muted-foreground">–</span>
                    <Input
                      className="w-24"
                      value={w.end}
                      placeholder="08:30"
                      onChange={(e) => {
                        const windows = (rule.time_windows ?? []).map((x, xi) =>
                          xi === wi ? { ...x, end: e.target.value } : x,
                        );
                        patchRule(idx, { time_windows: windows });
                      }}
                    />
                    <Select
                      value={w.timezone ?? "UTC"}
                      onValueChange={(v) => {
                        const tz = String(v ?? "UTC");
                        const windows = (rule.time_windows ?? []).map((x, xi) =>
                          xi === wi ? { ...x, timezone: tz } : x,
                        );
                        patchRule(idx, { time_windows: windows });
                      }}
                    >
                      <SelectTrigger className="w-36">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {TIMEZONE_OPTIONS.map((tz) => (
                          <SelectItem key={tz} value={tz}>
                            {tz}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={t("common.delete")}
                      onClick={() => {
                        const windows = (rule.time_windows ?? []).filter((_, xi) => xi !== wi);
                        patchRule(idx, { time_windows: windows });
                      }}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                ))}
              {timeEnabled(rule) && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() =>
                    patchRule(idx, {
                      time_windows: [...(rule.time_windows ?? []), DEFAULT_WINDOW()],
                    })
                  }
                >
                  <Plus className="size-4" />
                  {t("upstream.pricing.window.add")}
                </Button>
              )}

              {contextEnabled(rule) && (
                <div className="flex flex-wrap items-center gap-1.5">
                  <Label className="w-20 shrink-0">{t("upstream.pricing.context")}</Label>
                  <Input
                    className="w-24"
                    type="number"
                    min={0}
                    value={rule.context_min ?? 0}
                    onChange={(e) => patchRule(idx, { context_min: priceOrZero(e.target.value) })}
                  />
                  <span className="text-muted-foreground">–</span>
                  <Input
                    className="w-24"
                    type="number"
                    min={0}
                    placeholder={t("upstream.pricing.context.max.placeholder")}
                    value={rule.context_max ? String(rule.context_max) : ""}
                    onChange={(e) => patchRule(idx, { context_max: priceOrZero(e.target.value) })}
                  />
                </div>
              )}

              <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-4">
                {(
                  [
                    ["upstream.pricing.inputPrice", "input_price"],
                    ["upstream.pricing.outputPrice", "output_price"],
                    ["upstream.pricing.cacheCreationPrice", "cache_creation_price"],
                    ["upstream.pricing.cacheReadPrice", "cache_read_price"],
                  ] as const
                ).map(([labelKey, field]) => (
                  <div key={field} className="space-y-1">
                    <Label className="text-xs">{t(labelKey)}</Label>
                    <Input
                      type="number"
                      min={0}
                      step={0.000001}
                      value={String(rule[field] ?? 0)}
                      onChange={(e) => patchRule(idx, { [field]: priceOrZero(e.target.value) })}
                    />
                  </div>
                ))}
              </div>
            </div>
          ))}
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => emit(currency, [...rules, emptyRule()])}
          >
            <Plus className="size-4" />
            {t("upstream.pricing.rule.add")}
          </Button>
        </div>
      )}
    </div>
  );
}
