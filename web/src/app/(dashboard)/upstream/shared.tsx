"use client";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { ArrowLeftRight, ArrowUpFromLine, Type, Image as ImageIcon } from "lucide-react";
import { useT } from "@/lib/i18n";
import type { UpstreamUser } from "@/lib/types";

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

// 能力徽标：按模型输入模态渲染图标（text / image），未知模态回退为 Type 图标
export function CapabilityBadges({ capabilities }: { capabilities?: string[] }) {
  const caps = capabilities && capabilities.length > 0 ? capabilities : ["text"];
  return (
    <div className="flex items-center gap-1.5">
      {caps.map((cap) => (
        <TooltipRoot key={cap}>
          <TooltipTrigger
            render={
              <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                {cap === "image" ? (
                  <ImageIcon className="size-3 text-muted-foreground" />
                ) : (
                  <Type className="size-3 text-muted-foreground" />
                )}
              </span>
            }
          />
          <TooltipContent side="top">{cap}</TooltipContent>
        </TooltipRoot>
      ))}
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
  supportText: boolean;
  supportImage: boolean;
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
  contextLength: 256000,
  maxOutputTokens: 65536,
  supportText: true,
  supportImage: false,
};
