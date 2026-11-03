"use client";

import { Feather, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { useT } from "@/lib/i18n";
import { useTheme } from "@/lib/theme";
import { cn } from "@/lib/utils";

/** 偏好控件所处底色：普通页面背景，或侧栏（使用 sidebar-* token）。 */
export type PreferenceTone = "page" | "sidebar";

export const PREFERENCE_TONE_CLASS: Record<PreferenceTone, string> = {
  page: "text-foreground/70 hover:bg-secondary hover:text-foreground",
  sidebar: "text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-foreground",
};

/** Viewport center of the clicked switch: the theme transition's origin. */
function centerOf(el: Element) {
  const r = el.getBoundingClientRect();
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
}

export function ThemeSwitcher({
  tone = "page",
  tooltipSide = "bottom",
  className,
}: {
  tone?: PreferenceTone;
  tooltipSide?: "top" | "bottom" | "right";
  className?: string;
}) {
  const t = useT();
  const { theme, setTheme } = useTheme();

  const next = theme === "moonshot" ? "anthropic" : "moonshot";
  const label = theme === "moonshot" ? t("theme.to_anthropic") : t("theme.to_moonshot");
  const icon =
    theme === "moonshot" ? <Feather className="size-4" /> : <Sparkles className="size-4" />;

  return (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={(e) => setTheme(next, centerOf(e.currentTarget))}
              aria-label={label}
              className={cn(PREFERENCE_TONE_CLASS[tone], className)}
            >
              {icon}
            </Button>
          }
        />
        <TooltipContent side={tooltipSide}>{label}</TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );
}
