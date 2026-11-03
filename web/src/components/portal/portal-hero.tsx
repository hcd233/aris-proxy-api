"use client";

import Link from "next/link";
import { ArrowRight, ArrowUpRight } from "lucide-react";
import type { ReactNode } from "react";
import { SplitText } from "@/components/motion/split-text";
import { PreferenceControls } from "@/components/preference-controls";
import { ProviderIcon } from "@/components/provider-icon";
import { Button } from "@/components/ui/button";
import { useT } from "@/lib/i18n";
import { GITHUB_REPO_URL, PORTAL_SECTION_IDS, type PortalCtaLabelKey } from "@/lib/portal";
import { cn } from "@/lib/utils";

type Cta = { href: string; labelKey: PortalCtaLabelKey };

export function PortalEyebrow({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <span className={cn("portal-eyebrow", className)}>{children}</span>;
}

/** 区块统一三层节奏：eyebrow → h2 → 副文案。 */
export function PortalSectionHead({
  eyebrowKey,
  titleKey,
  subtitleKey,
}: {
  eyebrowKey: string;
  titleKey: string;
  subtitleKey?: string;
}) {
  const t = useT();
  return (
    <div className="mb-14 flex flex-col items-center gap-4 text-center">
      <PortalEyebrow>{t(eyebrowKey)}</PortalEyebrow>
      <h2 className="font-display text-[clamp(1.9rem,4.2vw,3.1rem)] leading-[1.08] font-normal tracking-[-0.035em] text-balance text-foreground">
        {t(titleKey)}
      </h2>
      {subtitleKey && (
        <p className="max-w-[38rem] text-base leading-relaxed text-pretty text-muted-foreground sm:text-lg">
          {t(subtitleKey)}
        </p>
      )}
    </div>
  );
}

function BrandLink() {
  const t = useT();
  return (
    <Link href="/" className="flex items-center gap-2" aria-label={t("portal.nav.home")}>
      <span className="flex size-7 items-center justify-center rounded-full bg-primary font-display text-sm font-semibold text-primary-foreground">
        A
      </span>
      <span className="font-display text-base font-semibold tracking-tight text-foreground">
        Aris
      </span>
    </Link>
  );
}

const NAV_LINKS = [
  { id: PORTAL_SECTION_IDS.flow, key: "portal.nav.flow" },
  { id: PORTAL_SECTION_IDS.capabilities, key: "portal.nav.capabilities" },
  { id: PORTAL_SECTION_IDS.getStarted, key: "portal.nav.get_started" },
] as const;

export function PortalHeader({ cta }: { cta: Cta }) {
  const t = useT();
  return (
    <header className="sticky top-0 z-30 px-4 pt-3.5">
      {/* 渐变遮罩：页面内容是滚动到悬浮胶囊底下的，没有遮罩时卡片标题会在胶囊上下
       * 同时露面、被胶囊边缘直接切断。渐变把内容淡出到页面底色，胶囊边界不再“割”文字。 */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-24 bg-gradient-to-b from-background via-background/80 to-transparent"
      />
      <nav
        aria-label={t("portal.nav.primary")}
        className="mx-auto flex h-14 w-full max-w-[45rem] items-center gap-5 rounded-full border border-border/60 bg-card/80 pr-2 pl-4 shadow-lg backdrop-blur-md"
      >
        <BrandLink />
        <div className="hidden items-center gap-5 md:flex">
          {NAV_LINKS.map((l) => (
            <a
              key={l.id}
              href={`#${l.id}`}
              className="rounded-sm text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none"
            >
              {t(l.key)}
            </a>
          ))}
        </div>
        <div className="flex-1" />
        <PreferenceControls compact className="max-sm:gap-1" />
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t("portal.nav.github")}
          className="text-foreground/70 hover:text-foreground"
          nativeButton={false}
          render={<a href={GITHUB_REPO_URL} target="_blank" rel="noopener noreferrer" />}
        >
          <ProviderIcon decorative protocol="github" size={16} />
        </Button>
        <Button
          size="sm"
          className="rounded-full max-sm:hidden"
          nativeButton={false}
          render={<Link href={cta.href} />}
        >
          {t(cta.labelKey)}
        </Button>
      </nav>
    </header>
  );
}

export function PortalHero({ cta }: { cta: Cta }) {
  const t = useT();
  return (
    <section className="relative isolate px-5 pt-24 pb-10 text-center sm:px-8 sm:pt-28">
      <div aria-hidden="true" className="portal-glow" />
      <div className="mx-auto flex max-w-4xl flex-col items-center">
        <PortalEyebrow className="portal-rise">{t("portal.kicker")}</PortalEyebrow>
        <h1 className="mt-6 font-display text-[clamp(2.75rem,7vw,5.5rem)] leading-[1.04] font-normal tracking-[-0.045em] text-balance text-foreground">
          <SplitText text={t("portal.tagline")} className="portal-hero-title" />
        </h1>
        <p
          className="portal-rise mt-6 max-w-[38rem] text-base leading-relaxed text-pretty text-muted-foreground sm:text-lg"
          style={{ "--delay": "420ms" } as React.CSSProperties}
        >
          {t("portal.subtitle")}
        </p>
        <div
          className="portal-rise mt-9 flex flex-wrap items-center justify-center gap-3"
          style={{ "--delay": "560ms" } as React.CSSProperties}
        >
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
            className="rounded-full"
            nativeButton={false}
            render={<a href={GITHUB_REPO_URL} target="_blank" rel="noopener noreferrer" />}
          >
            <ProviderIcon decorative protocol="github" size={16} />
            {t("portal.cta.github")}
            <ArrowUpRight data-icon="inline-end" />
          </Button>
        </div>
      </div>
    </section>
  );
}
