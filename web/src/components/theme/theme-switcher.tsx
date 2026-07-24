"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { Feather, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useT } from "@/lib/i18n";

type Theme = "anthropic" | "moonshot";

// The FAB mounts only on public pages (they have no UserBar); dashboard
// pages render the inline variant inside UserBar instead.
const FAB_PATHS = ["/login", "/callback", "/share"];

export function ThemeSwitcher({ variant = "fab" }: { variant?: "fab" | "inline" }) {
  const t = useT();
  const pathname = usePathname();
  const [theme, setTheme] = useState<Theme | null>(null);

  /* eslint-disable react-hooks/set-state-in-effect -- Reading <html data-theme> requires setting state in effect on mount */
  useEffect(() => {
    setTheme(document.documentElement.dataset.theme === "moonshot" ? "moonshot" : "anthropic");
  }, []);
  /* eslint-enable react-hooks/set-state-in-effect */

  const toggle = () => {
    if (theme === null) return;
    const next: Theme = theme === "moonshot" ? "anthropic" : "moonshot";
    const root = document.documentElement;
    root.classList.add("theme-transition");
    root.dataset.theme = next;
    try {
      localStorage.setItem("theme", next);
    } catch {
      // private mode etc.: theme still applies for this session
    }
    setTheme(next);
    window.setTimeout(() => root.classList.remove("theme-transition"), 400);
  };

  const label = theme === "moonshot" ? t("theme.to_anthropic") : t("theme.to_moonshot");
  const icon =
    theme === null ? (
      <span className="size-4" />
    ) : theme === "moonshot" ? (
      <Feather className="size-4" />
    ) : (
      <Sparkles className="size-4" />
    );

  if (variant === "inline") {
    return (
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={toggle}
        title={label}
        aria-label={label}
        className="text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-foreground"
      >
        {icon}
      </Button>
    );
  }

  const showFab = FAB_PATHS.some((p) => pathname === p || pathname.startsWith(`${p}/`));
  if (!showFab) return null;

  return (
    <button
      type="button"
      onClick={toggle}
      title={label}
      aria-label={label}
      className="fixed bottom-6 left-6 z-50 flex h-9 w-9 items-center justify-center rounded-full border border-border bg-popover/70 text-foreground/70 opacity-60 backdrop-blur-md transition-opacity hover:text-foreground hover:opacity-100"
    >
      {icon}
    </button>
  );
}
