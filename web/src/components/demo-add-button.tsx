"use client";

import { BadgePlus, Check } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
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
}

/** sessions 页「添加到 demo」按钮：admin 且 demo 登录开启时显示，点击 toggle 白名单 */
export function DemoAddButton({
  sessionId,
  inDemo,
  pending,
  loginEnabled,
  onToggle,
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
              className={inDemo ? "text-primary" : "text-foreground/70 hover:text-foreground"}
              aria-label={inDemo ? t("demo.in_demo_tooltip") : t("demo.add_tooltip")}
            >
              {inDemo ? <Check className="size-4" /> : <BadgePlus className="size-4" />}
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
