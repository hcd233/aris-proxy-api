"use client";

import { PortalCapabilities } from "@/components/portal/portal-capabilities";
import { PortalEcosystem } from "@/components/portal/portal-ecosystem";
import { PortalFeatureMarquee } from "@/components/portal/portal-feature-marquee";
import { PortalFlow } from "@/components/portal/portal-flow";
import { PortalHeader, PortalHero } from "@/components/portal/portal-hero";
import { useAuth } from "@/lib/auth-context";
import { portalPrimaryCta } from "@/lib/portal";

/** 公开门户：介绍产品、引导登录或前往 GitHub。已登录不自动跳转，按钮改为进入控制台。 */
export default function PortalPage() {
  const { user, isLoading } = useAuth();
  const cta = portalPrimaryCta({ isLoading, hasUser: user !== null });

  return (
    <div className="page-surface portal-root flex min-h-dvh flex-col overflow-x-clip bg-background">
      <div aria-hidden="true" className="portal-stars" />
      <PortalHeader cta={cta} />
      <main className="flex-1">
        <PortalHero cta={cta} />
        <PortalFeatureMarquee />
        <PortalEcosystem />
        <PortalFlow />
        <PortalCapabilities />
      </main>
    </div>
  );
}
