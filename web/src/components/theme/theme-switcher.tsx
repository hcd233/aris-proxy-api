"use client";

import { usePathname } from "next/navigation";
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

// The FAB mounts only on public pages (they have no UserBar); dashboard
// pages render the inline variant inside UserBar instead.
const FAB_PATHS = ["/login", "/callback", "/share"];

export function ThemeSwitcher({ variant = "fab" }: { variant?: "fab" | "inline" }) {
  const t = useT();
  const pathname = usePathname();
  const { theme, setTheme } = useTheme();

  const next = theme === "moonshot" ? "anthropic" : "moonshot";
  const label = theme === "moonshot" ? t("theme.to_anthropic") : t("theme.to_moonshot");
  const icon =
    theme === "moonshot" ? <Feather className="size-4" /> : <Sparkles className="size-4" />;

  if (variant === "inline") {
    return (
      <TooltipProvider>
        <TooltipRoot>
          <TooltipTrigger
            render={
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => setTheme(next)}
                aria-label={label}
                className="text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-foreground"
              >
                {icon}
              </Button>
            }
          />
          <TooltipContent side="top">{label}</TooltipContent>
        </TooltipRoot>
      </TooltipProvider>
    );
  }

  const showFab = FAB_PATHS.some((p) => pathname === p || pathname.startsWith(`${p}/`));
  if (!showFab) return null;

  return (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <button
              type="button"
              onClick={() => setTheme(next)}
              aria-label={label}
              className="fixed bottom-6 left-6 z-50 flex h-9 w-9 items-center justify-center rounded-full border border-border bg-popover/70 text-foreground/70 opacity-60 backdrop-blur-md transition-opacity hover:text-foreground hover:opacity-100"
            >
              {icon}
            </button>
          }
        />
        <TooltipContent side="right">{label}</TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );
}
