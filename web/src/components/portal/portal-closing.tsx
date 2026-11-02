"use client";

import Link from "next/link";
import { ArrowRight, ArrowUpRight } from "lucide-react";
import { ProviderIcon } from "@/components/provider-icon";
import { Button } from "@/components/ui/button";
import { useT } from "@/lib/i18n";
import { GITHUB_REPO_URL, PORTAL_SECTION_IDS, type PortalCtaLabelKey } from "@/lib/portal";
import { cn } from "@/lib/utils";
import { Reveal } from "./reveal";

type Cta = { href: string; labelKey: PortalCtaLabelKey };

export function PortalCta({ cta }: { cta: Cta }) {
  const t = useT();
  return (
    <section className="px-5 pt-16 pb-24 sm:px-8">
      <Reveal className="portal-cta-surface mx-auto flex min-h-[21rem] max-w-6xl flex-col justify-center gap-5 overflow-hidden rounded-[1.75rem] p-8 sm:p-14">
        <h2 className="max-w-[38rem] font-display text-[clamp(1.9rem,4.2vw,3.1rem)] leading-[1.08] font-normal tracking-[-0.035em] text-balance text-foreground">
          {t("portal.closing.title")}
        </h2>
        <p className="max-w-[34rem] text-base text-foreground/70 sm:text-lg">
          {t("portal.closing.subtitle")}
        </p>
        <div className="mt-2 flex flex-wrap gap-3">
          <Button
            size="lg"
            className="rounded-full"
            nativeButton={false}
            render={<Link href={cta.href} />}
          >
            {t(cta.labelKey)}
            <ArrowRight data-icon="inline-end" />
          </Button>
          <Button
            size="lg"
            variant="outline"
            className="rounded-full bg-transparent"
            nativeButton={false}
            render={<a href={GITHUB_REPO_URL} target="_blank" rel="noopener noreferrer" />}
          >
            <ProviderIcon decorative protocol="github" size={16} />
            {t("portal.cta.github")}
            <ArrowUpRight data-icon="inline-end" />
          </Button>
        </div>
      </Reveal>
    </section>
  );
}

export function PortalFooter({ cta }: { cta: Cta }) {
  const t = useT();
  const linkCls = "w-fit text-muted-foreground transition-colors hover:text-foreground";
  return (
    <footer className="overflow-hidden border-t border-border/60 pt-16">
      <div className="mx-auto grid max-w-6xl grid-cols-2 gap-8 px-5 text-sm sm:px-8 md:grid-cols-[1.6fr_1fr_1fr_1fr]">
        <div className="col-span-2 flex flex-col gap-2 md:col-span-1">
          <span className="font-display text-base font-semibold text-foreground">Aris MaaS</span>
          <span className="text-muted-foreground">{t("portal.footer.tagline")}</span>
        </div>
        <nav aria-label={t("portal.footer.product")} className="flex flex-col gap-2">
          <span className="mb-1 font-medium text-foreground">{t("portal.footer.product")}</span>
          <Link href={cta.href} className={linkCls}>
            {t("portal.footer.console")}
          </Link>
          <a href={`#${PORTAL_SECTION_IDS.capabilities}`} className={linkCls}>
            {t("portal.nav.capabilities")}
          </a>
        </nav>
        <nav aria-label={t("portal.footer.explore")} className="flex flex-col gap-2">
          <span className="mb-1 font-medium text-foreground">{t("portal.footer.explore")}</span>
          <a href={`#${PORTAL_SECTION_IDS.flow}`} className={linkCls}>
            {t("portal.nav.flow")}
          </a>
          <a href={`#${PORTAL_SECTION_IDS.getStarted}`} className={linkCls}>
            {t("portal.nav.get_started")}
          </a>
        </nav>
        <nav aria-label={t("portal.footer.source")} className="flex flex-col gap-2">
          <span className="mb-1 font-medium text-foreground">{t("portal.footer.source")}</span>
          <a href={GITHUB_REPO_URL} target="_blank" rel="noopener noreferrer" className={linkCls}>
            GitHub
          </a>
          <a
            href={GITHUB_REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            className={cn(linkCls, "font-mono text-xs")}
          >
            hcd233/aris-mass
          </a>
        </nav>
      </div>
      <p
        aria-hidden="true"
        className="portal-wordmark mt-12 text-center font-display text-[clamp(5.5rem,19vw,16rem)] leading-[0.8] font-medium tracking-[-0.06em] select-none"
      >
        Aris
      </p>
    </footer>
  );
}
