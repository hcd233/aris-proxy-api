"use client";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TableCell } from "@/components/ui/table";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { DeleteButton } from "@/components/delete-button";
import { ProviderIcon } from "@/components/provider-icon";
import {
  ArrowLeftRight,
  ArrowUpFromLine,
  AudioLines,
  Coins,
  FileText,
  Type,
  Image as ImageIcon,
  Lock,
  Pencil,
  Video,
  type LucideIcon,
} from "lucide-react";
import { useT } from "@/lib/i18n";
import { formatCost } from "@/lib/money";
import { cn } from "@/lib/utils";
import type {
  ConfigMissing,
  ModelCapability,
  PricingDTO,
  PricingRuleDTO,
  UpstreamUser,
} from "@/lib/types";
import { isDefaultRule } from "./pricing-rule";

// 模型表单默认规格：新建表单初值、编辑回填空值兜底、输入框占位共用同一口径
export const DEFAULT_CONTEXT_LENGTH = 256000;
export const DEFAULT_MAX_OUTPUT = 65536;

// 将 token 数格式化为紧凑可读形式：128000 -> 128K，1048576 -> 1M
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

// 未计价表单/展示的初值：currency 为空 ⇔ rules 为空 ⇔ 未计价
export const emptyPricing: PricingDTO = { currency: "", rules: [] };

// 无条件默认规则判定见 ./pricing-rule（isDefaultRule）

/**
 * 配置缺失徽标：只呈现「未填规格」。
 *
 * 未计价不再出徽标——定价列的 — 已经把这件事说完了（后端 !IsPriced() 与前端
 * rules 为空等价，两者永远同现同隐），徽标只是把同一句话再说一遍。无缺失不渲染。
 */
export function ConfigMissingBadges({ missing }: { missing?: ConfigMissing[] }) {
  const t = useT();
  if (!missing?.includes("spec")) return null;
  return (
    <div className="flex items-center gap-1">
      <Badge
        variant="outline"
        className="border-amber-500/30 bg-amber-500/15 font-mono text-amber-600 dark:text-amber-400"
      >
        {t("upstream.missing.spec")}
      </Badge>
    </div>
  );
}

// 归属用户展示单元：头像 + 用户名；user 缺省显示占位 —（恒定短占位不加 tooltip）
export function OwnerCell({ user }: { user?: UpstreamUser }) {
  if (!user) {
    return <span className="text-muted-foreground">—</span>;
  }
  return (
    <TooltipRoot>
      <TooltipTrigger
        render={
          <span className="flex max-w-[14ch] items-center gap-1.5">
            <Avatar size="sm">
              {user.avatar && <AvatarImage src={user.avatar} alt={user.name} />}
              <AvatarFallback className="text-[10px]">
                {user.name.charAt(0).toUpperCase() || "?"}
              </AvatarFallback>
            </Avatar>
            <span className="truncate text-xs text-muted-foreground">{user.name}</span>
          </span>
        }
      />
      <TooltipContent side="top" align="start" className="max-w-xs break-all">
        {user.name}
      </TooltipContent>
    </TooltipRoot>
  );
}

// 输入模态全集（枚举序，与后端 enum.InputModalities 一致）
export const MODEL_CAPABILITIES: ModelCapability[] = ["text", "image", "pdf", "video", "audio"];

const CAPABILITY_ICONS: Record<string, LucideIcon> = {
  text: Type,
  image: ImageIcon,
  pdf: FileText,
  video: Video,
  audio: AudioLines,
};

// 能力徽标：按模型输入模态渲染图标，未知模态回退为 Type 图标。
// 图标徽标按 px-1.5/gap-1 收紧：固定列宽下五个模态也能排成一行，不会把列撑宽或折行。
export function CapabilityBadges({ capabilities }: { capabilities?: string[] }) {
  const caps = capabilities && capabilities.length > 0 ? capabilities : ["text"];
  return (
    <div className="flex flex-wrap items-center gap-1">
      {caps.map((cap) => {
        const Icon = CAPABILITY_ICONS[cap] ?? Type;
        return (
          <TooltipRoot key={cap}>
            <TooltipTrigger
              render={
                <Badge variant="secondary" className="px-1.5 font-mono tabular-nums">
                  <Icon />
                </Badge>
              }
            />
            <TooltipContent side="top">{cap}</TooltipContent>
          </TooltipRoot>
        );
      })}
    </div>
  );
}

/**
 * 规格徽标：上下文窗口 + 最大输出。
 *
 * 原先桌面端与移动端各实现一遍（移动端还漏了 maxOutputTokens），这里统一为
 * 同一个组件，两个视图共用。
 */
export function SpecBadges({
  contextLength,
  maxOutputTokens,
}: {
  contextLength: number;
  maxOutputTokens: number;
}) {
  const t = useT();
  return (
    <div className="flex flex-wrap items-center gap-1">
      <TooltipRoot>
        <TooltipTrigger
          render={
            <Badge variant="secondary" className="px-2 font-mono tabular-nums">
              <ArrowLeftRight />
              {formatTokens(contextLength)}
            </Badge>
          }
        />
        <TooltipContent side="top" align="start" className="max-w-xs break-all">
          {`${t("models.context_length")}: ${contextLength.toLocaleString()}`}
        </TooltipContent>
      </TooltipRoot>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <Badge variant="secondary" className="px-2 font-mono tabular-nums">
              <ArrowUpFromLine />
              {formatTokens(maxOutputTokens)}
            </Badge>
          }
        />
        <TooltipContent side="top" align="start" className="max-w-xs break-all">
          {`${t("models.max_output")}: ${maxOutputTokens.toLocaleString()}`}
        </TooltipContent>
      </TooltipRoot>
    </div>
  );
}

/**
 * 「定价」单元格：已计价显示代表档的输入/输出单价（`$1 / $5`，多档时追加 +N），
 * 未计价显示占位 —（恒定短占位不加 tooltip）。完整档位表收进 Tooltip：
 * 列表列只能承担一对数字，分档结构展开看。
 */
export function PricingInline({ pricing }: { pricing?: PricingDTO }) {
  const t = useT();
  const rules = pricing?.rules ?? [];
  if (rules.length === 0) {
    return <span className="text-muted-foreground">—</span>;
  }
  const base = rules.find(isDefaultRule) ?? rules[0];
  const extra = rules.length - 1;
  // 档位标签：无条件档 → 「默认」，时段档 → 「时段限制」，其余按区间上下界显示
  const tierLabel = (r: PricingRuleDTO): string => {
    if (isDefaultRule(r)) return t("upstream.pricing.rule.default");
    if ((r.time_windows ?? []).length > 0) return t("upstream.pricing.opt.timeWindow");
    if ((r.context_max ?? 0) > 0) return `≤ ${formatTokens(r.context_max ?? 0)}`;
    return `≥ ${formatTokens(r.context_min ?? 0)}`;
  };
  return (
    <TooltipRoot>
      <TooltipTrigger
        render={
          <span className="inline-flex cursor-default items-center gap-1 font-mono text-xs tabular-nums">
            {formatCost(base.input_price, "USD")} / {formatCost(base.output_price, "USD")}
            {extra > 0 && <span className="text-muted-foreground">+{extra}</span>}
          </span>
        }
      />
      <TooltipContent side="top" align="start" className="max-w-xs">
        <div className="space-y-1">
          <p className="text-[11px] text-muted-foreground">{t("upstream.pricing.unit")}</p>
          <div className="flex items-center justify-between gap-3 text-[10px] tracking-[0.08em] text-muted-foreground uppercase">
            <span>{t("upstream.pricing.cell_tier")}</span>
            <span>{t("upstream.pricing.cell_inout")}</span>
          </div>
          {rules.map((r, i) => (
            <div key={i} className="flex items-center justify-between gap-3">
              <span className="text-muted-foreground">{tierLabel(r)}</span>
              <span className="font-mono tabular-nums">
                {formatCost(r.input_price, "USD")} / {formatCost(r.output_price, "USD")}
              </span>
            </div>
          ))}
        </div>
      </TooltipContent>
    </TooltipRoot>
  );
}

/**
 * 模型行共享单元格：分组/平铺两视图的桌面表格行内容一致，抽到这里消除重复。
 * 停用行只降权内容列，操作列保持可点的视觉（与原实现一致）。
 */

/**
 * 「模型」列：ProviderIcon + 可点击复制的 alias（停用行划线），次行收敛身份信息。
 *
 * 原先独立的「业务模型 ID」「上游真名」两列并到这里：与 alias 同值的项不再重复展示，
 * 只有真正有差异的（或平铺视图需要点明所属端点时）才出现在次行，因此行高不因并列而增加，
 * 表格也少两列、不必横向滚动。次行同样配 Tooltip，截断后仍可读全文。
 */
export function ModelIdentityCell({
  alias,
  modelId,
  upstreamModel,
  endpointName,
  enabled,
  onCopyAlias,
  className,
}: {
  alias: string;
  modelId?: string;
  upstreamModel?: string;
  /** 平铺视图专用：端点名（分组视图由组头承担，不传） */
  endpointName?: string;
  enabled: boolean;
  onCopyAlias: (alias: string) => void;
  /** 视图特有的单元格修饰（如分组视图的虚线树枝缩进） */
  className?: string;
}) {
  const t = useT();
  // 去重且剔除与 alias 重复的值：同一句话不在一行里说两遍
  const meta = [upstreamModel, modelId, endpointName].filter(
    (v, i, arr): v is string => !!v && v !== alias && arr.indexOf(v) === i,
  );
  const metaText = meta.join(" · ");
  return (
    <TableCell className={cn(className, !enabled && "opacity-45")}>
      <div className="flex min-w-0 flex-col gap-0.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <ProviderIcon protocol={alias} size={14} className="shrink-0" />
          <TooltipRoot>
            <TooltipTrigger
              render={
                <span
                  className={cn(
                    "min-w-0 cursor-pointer truncate font-medium underline-offset-2 hover:underline",
                    !enabled && "line-through",
                  )}
                  onClick={() => onCopyAlias(alias)}
                >
                  {alias}
                </span>
              }
            />
            <TooltipContent side="top" align="start" className="max-w-xs break-all">
              {`${t("models.click_to_copy")}: ${alias}`}
            </TooltipContent>
          </TooltipRoot>
        </div>
        {metaText && (
          <TooltipRoot>
            <TooltipTrigger
              render={
                <span className="min-w-0 truncate font-mono text-xs text-muted-foreground">
                  {metaText}
                </span>
              }
            />
            <TooltipContent side="top" align="start" className="max-w-xs break-all">
              {metaText}
            </TooltipContent>
          </TooltipRoot>
        )}
      </div>
    </TableCell>
  );
}

/**
 * 「操作」列：编辑模型配置 + 定价配置 + 删除。
 *
 * demo 只读账户与「他人公开端点下的模型」都锁定写入口（DeleteButton 的 locked 模式）：
 * 前者是全局只读，后者是普通用户对公开端点无维护权。
 */
export function ModelActionsCell({
  isDemo,
  readOnly = false,
  deleting,
  onEdit,
  onPricing,
  onDelete,
}: {
  isDemo: boolean;
  readOnly?: boolean;
  deleting: boolean;
  onEdit: () => void;
  onPricing: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  const locked = isDemo || readOnly;
  return (
    <TableCell className="text-right">
      <div className="flex items-center justify-end gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onEdit}
          disabled={locked}
          aria-label={t("common.edit")}
          className="text-muted-foreground hover:text-foreground"
        >
          {locked ? <Lock className="size-3.5" /> : <Pencil className="size-3.5" />}
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onPricing}
          disabled={locked}
          aria-label={t("upstream.pricing.title")}
          className="text-muted-foreground hover:text-foreground"
        >
          {locked ? <Lock className="size-3.5" /> : <Coins className="size-3.5" />}
        </Button>
        <DeleteButton
          label={t("common.delete")}
          locked={locked}
          disabled={deleting}
          onClick={onDelete}
        />
      </div>
    </TableCell>
  );
}

export { emptyEndpointForm, type EndpointForm } from "./endpoint-form";

export interface ModelForm {
  alias: string;
  modelId: string;
  upstreamModel: string;
  contextLength: number;
  maxOutputTokens: number;
  capabilities: ModelCapability[];
  /** 调度优先级（数字小=优先级高） */
  priority: number;
  /** 同优先级加权随机权重 */
  weight: number;
}

export const emptyModelForm: ModelForm = {
  alias: "",
  modelId: "",
  upstreamModel: "",
  contextLength: DEFAULT_CONTEXT_LENGTH,
  maxOutputTokens: DEFAULT_MAX_OUTPUT,
  capabilities: ["text"],
  priority: 0,
  weight: 1,
};
