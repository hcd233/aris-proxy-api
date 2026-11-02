"use client";

import type { CSSProperties } from "react";
import { ProviderIcon } from "@/components/provider-icon";
import { useT } from "@/lib/i18n";
import { PORTAL_CLIENTS, PORTAL_VENDORS, type PortalEntity } from "@/lib/portal";
import { Reveal } from "./reveal";

/** 一行图标卡：重复 n 次铺满宽屏，再整体两份首尾相接。 */
function LogoRow({
  items,
  label,
  reverse,
  repeat,
  duration,
}: {
  items: readonly PortalEntity[];
  label: string;
  reverse?: boolean;
  repeat: number;
  duration: string;
}) {
  const t = useT();
  const run = Array.from({ length: repeat }, () => items).flat();
  return (
    <div className="portal-marquee py-1.5" role="group" aria-label={label}>
      <div
        className="portal-marquee-track gap-3.5"
        data-reverse={reverse || undefined}
        style={{ "--marquee-duration": duration } as CSSProperties}
      >
        {[0, 1].map((copy) => (
          <ul
            key={copy}
            className="flex gap-3.5"
            aria-hidden={copy === 1 || undefined}
            inert={copy === 1 || undefined}
          >
            {run.map((e, i) => (
              <li
                key={`${e.id}-${i}`}
                aria-hidden={(copy === 0 && i >= items.length) || undefined}
                className="flex h-14 items-center gap-3 rounded-2xl bg-card pr-6 pl-4 text-base font-medium tracking-tight whitespace-nowrap text-foreground shadow-sm ring-1 ring-border/60"
              >
                <ProviderIcon decorative protocol={e.icon} size={22} />
                {e.labelKey ? t(e.labelKey) : e.label}
              </li>
            ))}
          </ul>
        ))}
      </div>
    </div>
  );
}

export function PortalEcosystem() {
  const t = useT();
  return (
    <Reveal as="section" className="pt-16 pb-10">
      <p className="mb-6 text-center text-sm text-muted-foreground">{t("portal.eco.title")}</p>
      <div className="flex flex-col gap-2">
        <LogoRow items={PORTAL_CLIENTS} label={t("portal.eco.clients")} repeat={4} duration="42s" />
        <LogoRow
          items={PORTAL_VENDORS}
          label={t("portal.eco.vendors")}
          repeat={3}
          duration="50s"
          reverse
        />
      </div>
    </Reveal>
  );
}
