"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
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
  // t 通过 ref 实时读取当前 locale：ref 在 state 变化的同时更新，t 引用保持
  // 永久稳定（全库 30+ 处 hook 把 t 写进依赖数组，若 t 随 locale 变化，登录
  // 回调与各列表页的数据加载 effect 会被整体重放，造成接口重复请求）。
  // 不能让 t 直接调用 detectBrowserLocale()——静态导出页面 hydration 时
  // 服务端 HTML 是英文、客户端首帧却会检测出 zh/ja，触发 React #418 文本
  // 不匹配；hydration 安全的取值只能来自 state（首帧恒为 "en"，挂载后由
  // effect 修正，与 theme/auth 初始化同模式）。语言响应性由 context value
  // （含 locale）变化触发重渲染承担。
  const activeLocaleRef = useRef<Locale>(locale);

  /* eslint-disable react-hooks/set-state-in-effect -- 首次挂载时从 localStorage/navigator 恢复用户语言偏好，与 auth/theme 初始化同模式；惰性初始化会在 SSR/hydration 两侧产生不一致 */
  useEffect(() => {
    const detected = detectBrowserLocale();
    activeLocaleRef.current = detected;
    setLocaleState(detected);
  }, []);
  /* eslint-enable react-hooks/set-state-in-effect */

  const setLocale = useCallback((next: Locale) => {
    activeLocaleRef.current = next;
    setLocaleState(next);
    localStorage.setItem("locale", next);
  }, []);

  const t = useCallback((key: string, fallback?: string): string => {
    return translations[activeLocaleRef.current]?.[key] ?? fallback ?? key;
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
