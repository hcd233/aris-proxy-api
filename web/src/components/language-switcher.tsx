"use client";

import { useI18n, type Locale } from "@/lib/i18n";

const LOCALES: Locale[] = ["en", "zh"];

export function LanguageSwitcher() {
  const { locale, setLocale } = useI18n();

  return (
    <div
      className="relative flex items-center rounded-lg border border-sidebar-border/50 bg-sidebar p-0.5 shadow-inner"
      role="radiogroup"
      aria-label="Language"
    >
      <div
        className={`absolute top-0.5 bottom-0.5 w-[calc(50%-2px)] rounded-md bg-primary shadow-sm transition-all duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)] ${
          locale === "en" ? "left-0.5" : "right-0.5"
        }`}
      />
      {LOCALES.map((l) => (
        <button
          key={l}
          onClick={() => setLocale(l)}
          role="radio"
          aria-checked={locale === l}
          className={`relative z-10 flex-1 cursor-pointer rounded-md px-3 py-1 text-[11px] font-medium leading-none tracking-wider transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-sidebar ${
            l === "en" ? "font-display" : ""
          } ${
            locale === l
              ? "text-primary-foreground"
              : "text-sidebar-foreground/40 hover:text-sidebar-foreground/80"
          }`}
        >
          {l === "en" ? "EN" : "中文"}
        </button>
      ))}
    </div>
  );
}
