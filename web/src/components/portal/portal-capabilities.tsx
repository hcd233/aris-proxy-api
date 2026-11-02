"use client";

import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { useT } from "@/lib/i18n";
import { PORTAL_CAPABILITIES, PORTAL_SECTION_IDS, type PortalCapabilityKey } from "@/lib/portal";
import { cn } from "@/lib/utils";
import {
  AuditStreamDemo,
  BreakerDemo,
  BulkheadDemo,
  DatasetDemo,
  DemoPane,
  PermissionLadder,
  ProtocolDemo,
  RouteDemo,
  SessionDemo,
  TriggerActions,
} from "./capability-demos";
import { PortalSectionHead } from "./portal-hero";
import { Reveal } from "./reveal";

function CapabilityCard({
  k,
  className,
  children,
}: {
  k: PortalCapabilityKey;
  className?: string;
  children: ReactNode;
}) {
  const t = useT();
  const cap = PORTAL_CAPABILITIES.find((c) => c.key === k)!;
  return (
    <Reveal
      className={cn(
        "col-span-12 flex min-w-0 flex-col gap-5 rounded-[1.625rem] bg-card p-6 shadow-md ring-1 ring-border/60 sm:p-7",
        className,
      )}
    >
      <header>
        <p className="text-xs text-primary-ink">{t(`portal.cap.${k}.eyebrow`)}</p>
        <h3 className="mt-1 font-display text-2xl leading-tight font-normal tracking-[-0.025em] text-balance text-foreground">
          {t(`portal.cap.${k}.title`)}
        </h3>
        <p className="mt-1.5 text-sm text-muted-foreground">{t(`portal.cap.${k}.desc`)}</p>
      </header>
      {children}
      <ul className="mt-auto flex flex-wrap gap-1.5">
        {cap.chips.map((c) => (
          <li key={c}>
            <Badge variant="outline">{t(c)}</Badge>
          </li>
        ))}
      </ul>
    </Reveal>
  );
}

export function PortalCapabilities() {
  const t = useT();
  return (
    <section id={PORTAL_SECTION_IDS.capabilities} className="scroll-mt-24 px-5 pt-24 pb-10 sm:px-8">
      <div className="mx-auto max-w-6xl">
        <Reveal>
          <PortalSectionHead
            eyebrowKey="portal.caps.eyebrow"
            titleKey="portal.caps.heading"
            subtitleKey="portal.caps.subtitle"
          />
        </Reveal>
        <div className="grid grid-cols-12 gap-4 lg:gap-5">
          <CapabilityCard k="access" className="lg:col-span-7">
            <DemoPane label={t("portal.cap.access.route_label")}>
              <RouteDemo />
              <p className="mt-3 text-xs text-muted-foreground">
                {t("portal.cap.access.affinity_note")}
              </p>
            </DemoPane>
            <DemoPane label={t("portal.cap.access.protocol_label")}>
              <ProtocolDemo />
            </DemoPane>
          </CapabilityCard>

          <CapabilityCard k="curation" className="lg:col-span-5">
            <DemoPane label={t("portal.cap.curation.session_label")}>
              <SessionDemo />
            </DemoPane>
            <DemoPane label={t("portal.cap.curation.export_label")}>
              <DatasetDemo />
            </DemoPane>
          </CapabilityCard>

          <CapabilityCard k="governance" className="lg:col-span-5">
            <DemoPane label={t("portal.cap.governance.audit_label")}>
              <AuditStreamDemo />
            </DemoPane>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <DemoPane label={t("portal.cap.governance.ladder_label")}>
                <PermissionLadder />
              </DemoPane>
              <DemoPane label={t("portal.cap.governance.trigger_label")}>
                <TriggerActions />
              </DemoPane>
            </div>
          </CapabilityCard>

          <CapabilityCard k="resilience" className="lg:col-span-7">
            <DemoPane label={t("portal.cap.resilience.breaker_label")}>
              <BreakerDemo />
            </DemoPane>
            <DemoPane label={t("portal.cap.resilience.bulkhead_label")}>
              <BulkheadDemo />
            </DemoPane>
          </CapabilityCard>
        </div>
      </div>
    </section>
  );
}
