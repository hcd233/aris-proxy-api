"use client";

import type { CSSProperties } from "react";
import { useT } from "@/lib/i18n";
import { PORTAL_FEATURES, type PortalFeatureKey } from "@/lib/portal";
import { FEATURE_ART } from "./feature-art";

function FeatureCard({ k }: { k: PortalFeatureKey }) {
  const t = useT();
  const Art = FEATURE_ART[k];
  return (
    <article className="w-[min(23.25rem,85vw)] shrink-0 rounded-[1.375rem] bg-card p-2.5 pb-5 shadow-md ring-1 ring-border/60 transition-transform duration-500 hover:-translate-y-1.5">
      <div className="flex h-48 flex-col justify-center gap-2.5 overflow-hidden rounded-2xl bg-muted/60 p-4">
        <Art />
      </div>
      <h3 className="mx-3 mt-4 text-[1.0625rem] font-medium tracking-tight text-foreground">
        {t(`portal.feature.${k}.title`)}
      </h3>
      <p className="mx-3 mt-1 text-sm text-muted-foreground">{t(`portal.feature.${k}.desc`)}</p>
    </article>
  );
}

/** Hero 下方的功能卡跑马灯：内容两份首尾相接，第二份对读屏器与键盘隐藏。 */
export function PortalFeatureMarquee() {
  return (
    <div
      className="portal-marquee portal-rise py-7"
      style={{ "--delay": "700ms" } as CSSProperties}
    >
      <div
        className="portal-marquee-track gap-5 px-2.5"
        style={{ "--marquee-duration": "64s" } as CSSProperties}
      >
        {[0, 1].map((copy) => (
          <div
            key={copy}
            className="flex gap-5"
            aria-hidden={copy === 1 || undefined}
            inert={copy === 1 || undefined}
          >
            {PORTAL_FEATURES.map((k) => (
              <FeatureCard key={k} k={k} />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
