"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

/**
 * Fades content out then in on locale change to mask residual reflow in
 * free-flowing text areas after layout stabilization (see ADR 0005).
 * Uses a ref + direct DOM opacity manipulation so no setState-in-effect.
 */
export function LocaleFade({
  children,
  className,
}: {
  children: ReactNode;
  /** 透传布局类：需要撑满高度的页面（如调试台）靠它把 flex 高度链传下去 */
  className?: string;
}) {
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
      className={cn("transition-opacity duration-100 ease-out", className)}
      style={{ opacity: 1 }}
    >
      {children}
    </div>
  );
}
