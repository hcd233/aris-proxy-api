"use client";

import { useI18n, type Locale } from "@/lib/i18n";
import { ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import type { PreferenceTone } from "@/components/theme/theme-switcher";

/// Add entries here when adding a new language.
const LANGUAGE_LABELS: Record<Locale, { native: string; short: string }> = {
  en: { native: "English", short: "EN" },
  zh: { native: "中文", short: "中" },
  ja: { native: "日本語", short: "日" },
};

const LOCALE_LIST = Object.entries(LANGUAGE_LABELS) as [
  Locale,
  { native: string; short: string },
][];

const TRIGGER_TONE_CLASS: Record<PreferenceTone, string> = {
  page: "",
  sidebar:
    "border-sidebar-border bg-transparent text-sidebar-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-foreground",
};

export function LanguageSwitcher({
  tone = "page",
  compact = false,
  className,
}: {
  tone?: PreferenceTone;
  /** 仅显示语言短码（折叠侧栏等窄位） */
  compact?: boolean;
  className?: string;
}) {
  const { locale, setLocale } = useI18n();
  const label = LANGUAGE_LABELS[locale];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="outline"
            size="xs"
            aria-label={label.native}
            // 定宽：三种语言名长度不同（English / 中文 / 日本語），自适应宽度会让
            // 触发器在切换后伸缩，连带挤动同一行的主题、登出按钮。
            className={cn(
              compact ? "w-8 min-w-0 px-0" : "w-22 justify-between",
              TRIGGER_TONE_CLASS[tone],
              className,
            )}
          />
        }
      >
        {compact ? label.short : label.native}
        {!compact && <ChevronDown data-icon="inline-end" />}
      </DropdownMenuTrigger>
      <DropdownMenuContent align={compact ? "start" : "end"} className="min-w-32">
        <DropdownMenuGroup>
          <DropdownMenuRadioGroup value={locale} onValueChange={(v) => setLocale(v as Locale)}>
            {LOCALE_LIST.map(([code, lang]) => (
              <DropdownMenuRadioItem key={code} value={code}>
                {lang.native}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
