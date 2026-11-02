"use client";

import { Check } from "lucide-react";
import { useT } from "@/lib/i18n";
import { GITHUB_REPO_URL, PORTAL_PILLARS, type PortalPillarKey } from "@/lib/portal";
import { ProviderIcon } from "@/components/provider-icon";

/** 三支柱色带：主色 + 两个与之区分明显的图表色，均为主题 token。 */
const PILLAR_ACCENT: Record<PortalPillarKey, string> = {
  access: "bg-primary",
  curation: "bg-chart-4",
  governance: "bg-chart-3",
};

export function PortalPillars() {
  const t = useT();
  return (
    <section className="mx-auto max-w-6xl px-5 pb-20 pt-6 sm:px-8">
      <h2 className="mb-8 text-center font-display text-3xl font-semibold tracking-tight text-foreground">
        {t("portal.pillars.title")}
      </h2>
      <div className="stagger-rise grid grid-cols-1 gap-5 md:grid-cols-3">
        {PORTAL_PILLARS.map((p) => (
          <article
            key={p.key}
            className="relative overflow-hidden rounded-xl border border-border bg-card p-6"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-x-0 top-0 h-[3px] ${PILLAR_ACCENT[p.key]}`}
            />
            <h3 className="font-display text-xl font-semibold tracking-tight text-foreground">
              {t(`portal.pillar.${p.key}.title`)}
            </h3>
            <p className="mt-1.5 text-sm text-muted-foreground">
              {t(`portal.pillar.${p.key}.desc`)}
            </p>
            <ul className="mt-5 flex flex-col gap-2.5">
              {p.items.map((k) => (
                <li key={k} className="flex gap-2.5 text-sm leading-6 text-foreground">
                  <Check className="mt-1 size-4 shrink-0 text-primary" aria-hidden="true" />
                  <span className="min-w-0">{t(k)}</span>
                </li>
              ))}
            </ul>
          </article>
        ))}
      </div>
    </section>
  );
}

export function PortalFooter() {
  const t = useT();
  return (
    <footer className="mt-auto border-t border-border/60">
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-5 py-6 text-sm text-muted-foreground sm:px-8">
        <span>
          <span className="font-display font-semibold text-foreground">Aris Proxy</span>
          {" · "}
          {t("portal.footer.tagline")}
        </span>
        <a
          href={GITHUB_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 transition-colors hover:text-foreground"
        >
          <ProviderIcon protocol="github" size={14} className="size-3.5" />
          hcd233/aris-proxy-api
        </a>
      </div>
    </footer>
  );
}
