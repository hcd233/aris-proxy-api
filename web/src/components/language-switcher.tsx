"use client";

import { useI18n, type Locale } from "@/lib/i18n";
import { ChevronDown, Check } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

/// Add entries here when adding a new language.
const LANGUAGE_LABELS: Record<Locale, { native: string }> = {
  en: { native: "English" },
  zh: { native: "中文" },
};

const LOCALE_LIST = Object.entries(LANGUAGE_LABELS) as [Locale, { native: string }][];

export function LanguageSwitcher() {
  const { locale, setLocale } = useI18n();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="flex cursor-pointer items-center gap-1 rounded-lg border border-sidebar-border/50 bg-sidebar px-2 py-1 text-[11px] font-medium leading-none tracking-wider transition-all duration-200 hover:bg-sidebar-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring data-popup-open:bg-sidebar-accent/50">
        <span className={locale === "en" ? "font-display" : ""}>
          {locale === "en" ? "EN" : "中文"}
        </span>
        <ChevronDown className="size-3 text-sidebar-foreground/30 transition-transform duration-200 data-popup-open:rotate-180" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="min-w-[130px] rounded-xl border-sidebar-border/50 bg-popover p-1 shadow-lg"
      >
        {LOCALE_LIST.map(([code, lang]) => {
          const isActive = locale === code;
          return (
            <DropdownMenuItem
              key={code}
              onClick={() => setLocale(code)}
              className={`flex items-center gap-2 rounded-lg px-2.5 py-2 text-xs transition-all duration-150 ${
                isActive
                  ? "bg-primary/10 text-primary font-medium"
                  : "text-popover-foreground/70 hover:text-popover-foreground"
              }`}
            >
              <span className={code === "en" ? "font-display" : ""}>
                {lang.native}
              </span>
              {isActive && <Check className="ml-auto size-3 shrink-0 text-primary" />}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
