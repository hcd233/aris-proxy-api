"use client";

import { PortalFlow } from "@/components/portal/portal-flow";
import { PortalHeader, PortalHero } from "@/components/portal/portal-hero";
import { PortalFooter, PortalPillars } from "@/components/portal/portal-pillars";
import { useAuth } from "@/lib/auth-context";
import { portalPrimaryCta } from "@/lib/portal";

/** 公开门户：介绍产品、引导登录或前往 GitHub。已登录不自动跳转，按钮改为进入控制台。 */
export default function PortalPage() {
  const { user, isLoading } = useAuth();
  const cta = portalPrimaryCta({ isLoading, hasUser: user !== null });

  return (
    <div className="page-surface flex min-h-dvh flex-col bg-background">
      <PortalHeader cta={cta} />
      <main className="flex-1">
        <PortalHero cta={cta} />
        <PortalFlow />
        <PortalPillars />
      </main>
      <PortalFooter />
    </div>
  );
}
