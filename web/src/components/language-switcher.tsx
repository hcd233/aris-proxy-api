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

/// Add entries here when adding a new language.
const LANGUAGE_LABELS: Record<Locale, { native: string }> = {
  en: { native: "English" },
  zh: { native: "中文" },
  ja: { native: "日本語" },
};

const LOCALE_LIST = Object.entries(LANGUAGE_LABELS) as [Locale, { native: string }][];

export function LanguageSwitcher() {
  const { locale, setLocale } = useI18n();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="outline" size="xs" className="min-w-0" />}>
        {LANGUAGE_LABELS[locale].native}
        <ChevronDown data-icon="inline-end" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-32">
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
