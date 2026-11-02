"use client";

import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { ProviderIcon } from "@/components/provider-icon";
import { PreferenceControls } from "@/components/preference-controls";
import { Button } from "@/components/ui/button";
import { useT } from "@/lib/i18n";
import { GITHUB_REPO_URL, type PortalCtaLabelKey } from "@/lib/portal";

export function PortalHeader({ cta }: { cta: { href: string; labelKey: PortalCtaLabelKey } }) {
  const t = useT();
  return (
    <header className="sticky top-0 z-30 border-b border-border/60 bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-3 px-5 sm:px-8">
        <Link href="/" className="flex items-center gap-2.5" aria-label="Aris Proxy">
          <span className="flex size-8 items-center justify-center rounded-[0.55rem] bg-primary font-display text-sm font-semibold text-primary-foreground">
            A
          </span>
          <span className="font-display text-lg font-semibold tracking-tight text-foreground">
            Aris
          </span>
        </Link>
        <div className="flex-1" />
        <PreferenceControls compact className="max-sm:gap-1" />
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("portal.nav.github")}
          className="text-foreground/70 hover:bg-secondary hover:text-foreground"
          nativeButton={false}
          render={<a href={GITHUB_REPO_URL} target="_blank" rel="noopener noreferrer" />}
        >
          <ProviderIcon protocol="github" size={16} className="size-4" />
        </Button>
        <Button
          size="sm"
          className="bg-foreground text-background hover:bg-primary hover:text-primary-foreground max-sm:hidden"
          nativeButton={false}
          render={<Link href={cta.href} />}
        >
          {t(cta.labelKey)}
        </Button>
      </div>
    </header>
  );
}

export function PortalHero({ cta }: { cta: { href: string; labelKey: PortalCtaLabelKey } }) {
  const t = useT();
  return (
    <section className="mx-auto max-w-4xl px-5 pb-10 pt-16 text-center sm:px-8 sm:pt-24">
      <p className="animate-rise text-xs font-medium uppercase tracking-[0.18em] text-primary">
        {t("portal.kicker")}
      </p>
      <h1 className="animate-rise motion-delay-1 mt-5 font-display text-[clamp(2.25rem,6vw,4.5rem)] font-semibold leading-[1.05] tracking-[-0.025em] text-foreground text-balance">
        {t("portal.tagline")}
      </h1>
      <p className="animate-rise motion-delay-2 mx-auto mt-6 max-w-2xl text-base leading-[1.8] text-muted-foreground text-pretty sm:text-lg">
        {t("portal.subtitle")}
      </p>
      <div className="animate-rise motion-delay-3 mt-9 flex flex-wrap items-center justify-center gap-3">
        <Button
          size="lg"
          className="bg-foreground text-background hover:bg-primary hover:text-primary-foreground"
          nativeButton={false}
          render={<Link href={cta.href} />}
        >
          {t(cta.labelKey)}
        </Button>
        <Button
          size="lg"
          variant="outline"
          nativeButton={false}
          render={<a href={GITHUB_REPO_URL} target="_blank" rel="noopener noreferrer" />}
        >
          <ProviderIcon protocol="github" size={16} className="size-4" />
          {t("portal.cta.github")}
          <ArrowUpRight data-icon="inline-end" />
        </Button>
      </div>
    </section>
  );
}
