"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";

/**
 * Fades content out then in on locale change to mask residual reflow in
 * free-flowing text areas after layout stabilization (see ADR 0005).
 * Uses a ref + direct DOM opacity manipulation so no setState-in-effect.
 */
export function LocaleFade({ children }: { children: ReactNode }) {
  const { locale } = useI18n();
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.opacity = "0";
    const timer = setTimeout(() => {
      el.style.opacity = "1";
    }, 120);
    return () => clearTimeout(timer);
  }, [locale]);

  return (
    <div
      ref={ref}
      className="transition-opacity duration-100 ease-out"
      style={{ opacity: 1 }}
    >
      {children}
    </div>
  );
}
