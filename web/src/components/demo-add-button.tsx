"use client";

import { BadgePlus, Check } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";

interface DemoAddButtonProps {
  sessionId: number;
  /** 该 session 是否已在 demo 白名单 */
  inDemo: boolean;
  /** 添加/移除请求进行中 */
  pending: boolean;
  /** demo 登录开关（false 时不渲染按钮） */
  loginEnabled: boolean;
  onToggle: (id: number) => void;
  /** 覆盖尺寸/颜色（与 DeleteIconButton 同构，供详情页 header 放大到 size-10） */
  className?: string;
  iconClassName?: string;
}

/** sessions 页「添加到 demo」按钮：admin 且 demo 登录开启时显示，点击 toggle 白名单 */
export function DemoAddButton({
  sessionId,
  inDemo,
  pending,
  loginEnabled,
  onToggle,
  className,
  iconClassName,
}: DemoAddButtonProps) {
  const t = useT();
  const { isAdmin } = useAuth();

  if (!isAdmin() || !loginEnabled) return null;

  return (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <Button
              variant={inDemo ? "secondary" : "ghost"}
              size="icon-sm"
              disabled={pending}
              onClick={(e) => {
                e.stopPropagation();
                onToggle(sessionId);
              }}
              className={cn(
                inDemo ? "text-primary" : "text-foreground/70 hover:text-foreground",
                className,
              )}
              aria-label={inDemo ? t("demo.in_demo_tooltip") : t("demo.add_tooltip")}
            >
              {inDemo ? (
                <Check className={cn("size-4", iconClassName)} />
              ) : (
                <BadgePlus className={cn("size-4", iconClassName)} />
              )}
            </Button>
          }
        />
        <TooltipContent side="top">
          {inDemo ? t("demo.in_demo_tooltip") : t("demo.add_tooltip")}
        </TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );
}
