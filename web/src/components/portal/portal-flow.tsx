"use client";

import { ArrowRight } from "lucide-react";
import { useT } from "@/lib/i18n";
import { PORTAL_FLOW } from "@/lib/portal";

/** 专名原样展示，portal.* 开头的视为 i18n key。 */
function useNodeLabel() {
  const t = useT();
  return (s: string) => (s.startsWith("portal.") ? t(s) : s);
}

function FlowColumn({ title, nodes }: { title: string; nodes: readonly string[] }) {
  const label = useNodeLabel();
  return (
    <div className="flex flex-col gap-2">
      <p className="text-center text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
        {title}
      </p>
      {nodes.map((n) => (
        <div
          key={n}
          className="rounded-lg border border-border bg-card/70 px-4 py-2.5 text-center text-sm text-foreground"
        >
          {label(n)}
        </div>
      ))}
    </div>
  );
}

function FlowArrow() {
  return (
    <div className="flex items-center justify-center text-muted-foreground" aria-hidden="true">
      <ArrowRight className="size-5 rotate-90 md:rotate-0" />
    </div>
  );
}

export function PortalFlow() {
  const t = useT();
  return (
    <section className="mx-auto max-w-5xl px-5 py-10 sm:px-8">
      <h2 className="sr-only">{t("portal.flow.title")}</h2>
      <div className="animate-rise motion-delay-4 grid grid-cols-1 items-center gap-4 rounded-2xl border border-border/70 bg-card/40 p-6 md:grid-cols-[1fr_auto_1.1fr_auto_1fr] md:gap-6 md:p-8">
        <FlowColumn title={t("portal.flow.clients")} nodes={PORTAL_FLOW.clients} />
        <FlowArrow />
        <div className="rounded-xl border-2 border-primary bg-primary/[0.07] px-5 py-6 text-center moonshot:bg-white/[0.04]">
          <p className="font-display text-2xl font-semibold tracking-tight text-foreground">Aris</p>
          <p className="mt-2 text-xs leading-5 text-muted-foreground">
            {t("portal.flow.gateway_desc")}
          </p>
        </div>
        <FlowArrow />
        <FlowColumn title={t("portal.flow.upstreams")} nodes={PORTAL_FLOW.upstreams} />
      </div>
    </section>
  );
}
