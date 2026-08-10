"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import en from "@/locales/en.json";
import zh from "@/locales/zh.json";
import ja from "@/locales/ja.json";

export type Locale = "en" | "zh" | "ja";

const translations: Record<Locale, Record<string, string>> = { en, zh, ja };

function detectBrowserLocale(): Locale {
  if (typeof window === "undefined") return "en";
  const stored = localStorage.getItem("locale");
  if (stored === "en" || stored === "zh" || stored === "ja") return stored;
  const navLang = navigator.language.toLowerCase();
  if (navLang.startsWith("zh")) return "zh";
  if (navLang.startsWith("ja")) return "ja";
  return "en";
}

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, fallback?: string) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en");

  /* eslint-disable react-hooks/set-state-in-effect -- 首次挂载时从 localStorage/navigator 恢复用户语言偏好，与 auth/theme 初始化同模式；惰性初始化会在 SSR/hydration 两侧产生不一致 */
  useEffect(() => {
    setLocaleState(detectBrowserLocale());
  }, []);
  /* eslint-enable react-hooks/set-state-in-effect */

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    localStorage.setItem("locale", next);
  }, []);

  const t = useCallback(
    (key: string, fallback?: string): string => {
      return translations[locale]?.[key] ?? fallback ?? key;
    },
    [locale],
  );

  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used within I18nProvider");
  }
  return ctx;
}

export function translate(key: string, fallback?: string): string {
  return translations[detectBrowserLocale()]?.[key] ?? fallback ?? key;
}

export function useT() {
  return useI18n().t;
}
