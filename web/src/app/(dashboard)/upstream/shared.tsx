"use client";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { TableCell } from "@/components/ui/table";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { DeleteButton } from "@/components/delete-button";
import { ProviderIcon } from "@/components/provider-icon";
import {
  ArrowLeftRight,
  ArrowUpFromLine,
  AudioLines,
  FileText,
  Type,
  Image as ImageIcon,
  Lock,
  Pencil,
  Video,
  type LucideIcon,
} from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { ModelCapability, PricingDTO, UpstreamUser } from "@/lib/types";

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

// 能力徽标：按模型输入模态渲染图标，未知模态回退为 Type 图标
export function CapabilityBadges({ capabilities }: { capabilities?: string[] }) {
  const caps = capabilities && capabilities.length > 0 ? capabilities : ["text"];
  return (
    <div className="flex items-center gap-1.5">
      {caps.map((cap) => {
        const Icon = CAPABILITY_ICONS[cap] ?? Type;
        return (
          <TooltipRoot key={cap}>
            <TooltipTrigger
              render={
                <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                  <Icon className="size-3 text-muted-foreground" />
                </span>
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
    <div className="flex items-center gap-1.5">
      <TooltipRoot>
        <TooltipTrigger
          render={
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
              <ArrowLeftRight className="size-3 text-muted-foreground" />
              {formatTokens(contextLength)}
            </span>
          }
        />
        <TooltipContent side="top" align="start" className="max-w-xs break-all">
          {`${t("models.context_length")}: ${contextLength.toLocaleString()}`}
        </TooltipContent>
      </TooltipRoot>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
              <ArrowUpFromLine className="size-3 text-muted-foreground" />
              {formatTokens(maxOutputTokens)}
            </span>
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
 * 模型行共享单元格：分组/平铺两视图的桌面表格行内容一致，抽到这里消除重复。
 * 停用行只降权内容列，操作列保持可点的视觉（与原实现一致）。
 */

/** 「模型别名」列：ProviderIcon + 可点击复制的 alias（停用行划线） */
export function ModelAliasCell({
  alias,
  enabled,
  onCopyAlias,
  className,
}: {
  alias: string;
  enabled: boolean;
  onCopyAlias: (alias: string) => void;
  /** 视图特有的单元格修饰（如分组视图的虚线树枝缩进） */
  className?: string;
}) {
  const t = useT();
  return (
    <TableCell className={cn(className, !enabled && "opacity-45")}>
      <div className="flex min-w-0 items-center gap-1.5">
        <ProviderIcon protocol={alias} size={14} className="shrink-0" />
        <TooltipRoot>
          <TooltipTrigger
            render={
              <span
                className={cn(
                  "max-w-[16ch] cursor-pointer truncate font-medium underline-offset-2 hover:underline",
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
    </TableCell>
  );
}

/** 「业务模型 ID」列：与 alias 相同时显示占位 —（恒定短占位不加 tooltip） */
export function ModelIdCell({
  modelId,
  alias,
  enabled,
}: {
  modelId: string;
  alias: string;
  enabled: boolean;
}) {
  return (
    <TableCell className={cn("font-mono text-xs", !enabled && "opacity-45")}>
      {modelId && modelId !== alias ? (
        <TooltipRoot>
          <TooltipTrigger render={<span className="block max-w-[20ch] truncate">{modelId}</span>} />
          <TooltipContent side="top" align="start" className="max-w-xs break-all">
            {modelId}
          </TooltipContent>
        </TooltipRoot>
      ) : (
        <span className="text-muted-foreground">—</span>
      )}
    </TableCell>
  );
}

/** 「上游模型名」列 */
export function UpstreamModelCell({
  upstreamModel,
  enabled,
}: {
  upstreamModel: string;
  enabled: boolean;
}) {
  return (
    <TableCell className={cn("font-mono text-xs", !enabled && "opacity-45")}>
      <TooltipRoot>
        <TooltipTrigger
          render={<span className="block max-w-[20ch] truncate">{upstreamModel}</span>}
        />
        <TooltipContent side="top" align="start" className="max-w-xs break-all">
          {upstreamModel}
        </TooltipContent>
      </TooltipRoot>
    </TableCell>
  );
}

/** 「操作」列：编辑 + 删除；demo 只读账户写入口统一锁定（DeleteButton 的 locked 模式） */
export function ModelActionsCell({
  isDemo,
  deleting,
  onEdit,
  onDelete,
}: {
  isDemo: boolean;
  deleting: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  return (
    <TableCell className="text-right">
      <div className="flex items-center justify-end gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onEdit}
          disabled={isDemo}
          aria-label={t("common.edit")}
          className="text-muted-foreground hover:text-foreground"
        >
          {isDemo ? <Lock className="size-3.5" /> : <Pencil className="size-3.5" />}
        </Button>
        <DeleteButton
          label={t("common.delete")}
          locked={isDemo}
          disabled={deleting}
          onClick={onDelete}
        />
      </div>
    </TableCell>
  );
}

export interface EndpointForm {
  name: string;
  openaiBaseURL: string;
  anthropicBaseURL: string;
  apiKey: string;
  supportOpenAIChatCompletion: boolean;
  supportOpenAIResponse: boolean;
  supportAnthropicMessage: boolean;
  ownerUserID?: number;
}

export interface ModelForm {
  alias: string;
  modelId: string;
  upstreamModel: string;
  contextLength: number;
  maxOutputTokens: number;
  capabilities: ModelCapability[];
  /** 定价（currency="" ⇔ rules 为空 ⇔ 未计价） */
  pricing: PricingDTO;
}

export const emptyEndpointForm: EndpointForm = {
  name: "",
  openaiBaseURL: "",
  anthropicBaseURL: "",
  apiKey: "",
  supportOpenAIChatCompletion: true,
  supportOpenAIResponse: false,
  supportAnthropicMessage: false,
};

export const emptyModelForm: ModelForm = {
  alias: "",
  modelId: "",
  upstreamModel: "",
  contextLength: DEFAULT_CONTEXT_LENGTH,
  maxOutputTokens: DEFAULT_MAX_OUTPUT,
  capabilities: ["text"],
  pricing: { currency: "", rules: [] },
};
