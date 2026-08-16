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

  // t 的引用必须永久稳定：全库 30+ 处 hook 把 t 写进依赖数组，若 t 随 locale
  // 变化（挂载后 en→检测值必然发生一次），登录回调与各列表页的数据加载
  // effect 会被整体重放，造成 /user/current、/demo/config 等接口重复请求。
  // 语言响应性由 context value（含 locale）变化触发重渲染承担；client 侧与
  // translate() 一致，每次调用实时读取当前 locale，不缓存旧翻译。
  const t = useCallback((key: string, fallback?: string): string => {
    const active: Locale = typeof window === "undefined" ? "en" : detectBrowserLocale();
    return translations[active]?.[key] ?? fallback ?? key;
  }, []);

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
