"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
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
import type { PricingDTO, PricingRuleDTO, TimeWindowDTO } from "@/lib/types";
import { cn } from "@/lib/utils";

export interface PricingEditorProps {
  value?: PricingDTO;
  onChange: (p: PricingDTO) => void;
  /** 用于导入的上游模型名（表单 upstreamModel 字段） */
  upstreamModel: string;
  /** 模型最大上下文长度（tokens）；导入时作为末档区间上界 */
  contextLength?: number;
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
 * - 「启用计价」开关（关 = 未计价，币种固定 USD，由规则表推导）；关时规则区隐藏
 * - 每条规则两个默认关闭的选项：「时段限制」勾选后编辑时段窗口；「上下文区间」勾选后编辑 min/max
 * - 时段窗口星期选择：days 空=每天（7 键全亮 + 「每天」徽标）；显式选满 7 天归一化回空数组；
 *   最后一天禁点（取消会静默翻回「每天」，与直觉相反）
 * - 行操作：增、删、上移/下移（数组顺序 = 匹配优先级）；无条件默认规则行标注「默认」
 * - 「从 models.dev 导入」两种状态下都可点，命中后整体替换规则表并自动进入已计价（含上下文区间规则；时段窗口无数据需手填）
 */
export function PricingEditor({
  value,
  onChange,
  upstreamModel,
  contextLength,
}: PricingEditorProps) {
  const t = useT();
  const [importing, setImporting] = useState(false);
  const rules = value?.rules ?? [];
  // 币种固定 USD：规则表非空 ⇔ 已计价
  const priced = rules.length > 0;

  const emit = (nextRules: PricingRuleDTO[]) => {
    const next: PricingDTO =
      nextRules.length > 0 ? { currency: "USD", rules: nextRules } : { currency: "", rules: [] };
    onChange(next);
  };

  const patchRule = (idx: number, patch: Partial<PricingRuleDTO>) => {
    emit(rules.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
  };

  const moveRule = (idx: number, delta: number) => {
    const j = idx + delta;
    if (j < 0 || j >= rules.length) return;
    const next = [...rules];
    [next[idx], next[j]] = [next[j], next[idx]];
    emit(next);
  };

  const handlePrefill = async () => {
    if (!upstreamModel.trim()) {
      toast.error(t("upstream.pricing.prefill.miss"));
      return;
    }
    setImporting(true);
    try {
      const rsp = await api.prefillModelPricing(upstreamModel.trim(), contextLength);
      const imported = rsp.pricing?.rules ?? [];
      if (!rsp.found || imported.length === 0) {
        toast.error(t("upstream.pricing.prefill.miss"));
        return;
      }
      // 非空规则 ⇒ emit 推导为 USD，等价于「自动打开计价开关」
      emit(imported);
      toast.success(t("upstream.pricing.prefill.hint"));
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="space-y-3">
      {/* flex-wrap：计价开关 + 单位说明 + 导入按钮的 min-content 之和超过弹窗正文宽度时
          换行；否则会把弹窗 grid 轨道撑宽，定价卡片整体溢出弹窗右边界 */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex items-center gap-2">
          <Switch
            id="pricing-enabled"
            size="sm"
            checked={priced}
            onCheckedChange={(checked) => emit(checked ? [emptyRule()] : [])}
          />
          <Label htmlFor="pricing-enabled">{t("upstream.pricing.enabled")}</Label>
        </div>
        {priced && (
          <span className="text-xs text-muted-foreground">{t("upstream.pricing.unit")}</span>
        )}
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={handlePrefill}
          disabled={importing}
        >
          <Download className="size-4" />
          {t("upstream.pricing.prefill")}
        </Button>
      </div>

      {!priced && (
        <p className="text-[11px] text-muted-foreground">{t("upstream.pricing.disabled_hint")}</p>
      )}

      {priced && (
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
                    onClick={() => emit(rules.filter((_, i) => i !== idx))}
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
                    <div
                      className="flex gap-1"
                      role="group"
                      aria-label={t("upstream.pricing.window.days")}
                    >
                      {WEEKDAYS.map((d) => {
                        const cur = w.days ?? [];
                        // 空=每天：7 键全亮；点选后进入显式选择模式
                        const active = cur.length === 0 || cur.includes(d);
                        // 最后一天禁点：取消会静默翻回「每天」，与「仅剩一天」直觉相反
                        const lastDay = cur.length === 1 && cur.includes(d);
                        return (
                          <Button
                            key={d}
                            type="button"
                            size="xs"
                            variant="outline"
                            // min-w-0 抵消 size xs 内置 min-w-14；w-9 固定宽保证跨语言不位移
                            className={cn(
                              "w-9 min-w-0 p-0",
                              active
                                ? "border-primary/40 bg-primary/10 text-primary hover:bg-primary/15"
                                : "text-muted-foreground",
                            )}
                            aria-pressed={active}
                            disabled={lastDay}
                            onClick={() => {
                              const base = cur.length === 0 ? WEEKDAYS : cur;
                              let nextDays = base.includes(d)
                                ? base.filter((x) => x !== d)
                                : [...base, d].sort((a, b) => a - b);
                              // 显式选满 7 天与「每天」后端语义等价，归一化回空数组
                              if (nextDays.length === WEEKDAYS.length) nextDays = [];
                              const windows = (rule.time_windows ?? []).map((x, xi) =>
                                xi === wi ? { ...x, days: nextDays } : x,
                              );
                              patchRule(idx, { time_windows: windows });
                            }}
                          >
                            {t(`upstream.pricing.window.day.${d}`)}
                          </Button>
                        );
                      })}
                    </div>
                    {(w.days ?? []).length === 0 && (
                      <Badge variant="secondary">{t("upstream.pricing.window.everyday")}</Badge>
                    )}
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
            onClick={() => emit([...rules, emptyRule()])}
          >
            <Plus className="size-4" />
            {t("upstream.pricing.rule.add")}
          </Button>
        </div>
      )}
    </div>
  );
}
