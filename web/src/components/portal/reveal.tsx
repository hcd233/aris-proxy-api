"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * 首次进入视口时揭示一次（data-revealed），之后不再观察。
 * 视觉由 styles/portal.css 的 [data-reveal] 规则决定；still 档 CSS 直接显示终态。
 */
export function Reveal({
  as: Tag = "div",
  className,
  children,
  id,
}: {
  as?: "div" | "section";
  className?: string;
  children: ReactNode;
  id?: string;
}) {
  const ref = useRef<HTMLElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (typeof IntersectionObserver === "undefined") {
      el.dataset.revealed = "";
      return;
    }
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (!e.isIntersecting) continue;
          (e.target as HTMLElement).dataset.revealed = "";
          io.unobserve(e.target);
        }
      },
      { threshold: 0.12 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);

  return (
    <Tag ref={ref as React.Ref<HTMLDivElement>} id={id} data-reveal="" className={cn(className)}>
      {children}
    </Tag>
  );
}
