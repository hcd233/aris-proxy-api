"use client";

import { useEffect, useState } from "react";
import { Feather, Sparkles } from "lucide-react";
import { useT } from "@/lib/i18n";

type Theme = "anthropic" | "moonshot";

export function ThemeSwitcher() {
  const t = useT();
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

  return (
    <button
      type="button"
      onClick={toggle}
      title={label}
      aria-label={label}
      className="fixed bottom-6 left-6 z-50 flex h-9 w-9 items-center justify-center rounded-full border border-border bg-popover/70 text-foreground/70 opacity-60 backdrop-blur-md transition-opacity hover:text-foreground hover:opacity-100"
    >
      {theme === null ? (
        <span className="size-4" />
      ) : theme === "moonshot" ? (
        <Feather className="size-4" />
      ) : (
        <Sparkles className="size-4" />
      )}
    </button>
  );
}
