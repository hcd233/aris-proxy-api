"use client";

import { useState, type CSSProperties } from "react";
import { Badge } from "@/components/ui/badge";
import { useT } from "@/lib/i18n";
import { PORTAL_CLIENTS, PORTAL_SECTION_IDS, PORTAL_STEPS, type PortalStepKey } from "@/lib/portal";
import { cn } from "@/lib/utils";
import { PortalSectionHead } from "./portal-hero";
import { Reveal } from "./reveal";
import { ApiKeyScene, SessionScene, TerminalScene, UpstreamScene } from "./step-scenes";
import { usePortalLoop } from "./use-portal-loop";

const STEP_MS = 7000;
const STEP_CHIP_COUNT: Record<PortalStepKey, number> = {
  login: 3,
  upstream: 4,
  install: 0,
  use: 4,
};
const SCENES: Record<PortalStepKey, () => React.ReactNode> = {
  login: ApiKeyScene,
  upstream: UpstreamScene,
  install: TerminalScene,
  use: SessionScene,
};

function StepChips({ k }: { k: PortalStepKey }) {
  const t = useT();
  const labels =
    k === "install"
      ? PORTAL_CLIENTS.map((c) => c.label)
      : Array.from({ length: STEP_CHIP_COUNT[k] }, (_, i) => t(`portal.step.${k}.chip_${i + 1}`));
  return (
    <ul className="mt-3 flex flex-wrap gap-1.5">
      {labels.map((l) => (
        <li key={l}>
          <Badge variant="outline">{l}</Badge>
        </li>
      ))}
    </ul>
  );
}

export function PortalGetStarted() {
  const t = useT();
  const [current, setCurrent] = useState(0);
  const [userPicked, setUserPicked] = useState(false);
  usePortalLoop(() => {
    if (!userPicked) setCurrent((c) => (c + 1) % PORTAL_STEPS.length);
  }, STEP_MS);
  const key = PORTAL_STEPS[current];
  const Scene = SCENES[key];

  return (
    <section id={PORTAL_SECTION_IDS.getStarted} className="scroll-mt-24 px-5 pt-24 pb-10 sm:px-8">
      <div className="mx-auto max-w-6xl">
        <Reveal>
          <PortalSectionHead
            eyebrowKey="portal.steps.eyebrow"
            titleKey="portal.steps.heading"
            subtitleKey="portal.steps.subtitle"
          />
        </Reveal>
        <Reveal className="grid items-start gap-10 lg:grid-cols-[1fr_1.05fr] lg:gap-12">
          <ol className="flex flex-col">
            {PORTAL_STEPS.map((k, i) => {
              const on = i === current;
              return (
                <li key={k} className="relative border-t border-border last:border-b">
                  <button
                    type="button"
                    aria-expanded={on}
                    aria-controls="portal-step-window"
                    onClick={() => {
                      setUserPicked(true);
                      setCurrent(i);
                    }}
                    className={cn(
                      "grid w-full grid-cols-[2.75rem_1fr] gap-1 py-5 text-left transition-opacity duration-300 focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none",
                      !on && "opacity-55 hover:opacity-100",
                    )}
                  >
                    <span
                      className={cn(
                        "pt-1 font-mono text-sm",
                        on ? "text-primary" : "text-muted-foreground",
                      )}
                    >
                      {String(i + 1).padStart(2, "0")}
                    </span>
                    <span>
                      <span className="block text-xl font-medium tracking-tight text-foreground">
                        {t(`portal.step.${k}.title`)}
                      </span>
                    </span>
                  </button>
                  {on && (
                    <div className="grid grid-cols-[2.75rem_1fr] gap-1 pb-5">
                      <span />
                      <div>
                        <p className="text-[0.9375rem] leading-relaxed text-muted-foreground">
                          {t(`portal.step.${k}.desc`)}
                        </p>
                        <StepChips k={k} />
                      </div>
                    </div>
                  )}
                  {on && !userPicked && (
                    <span
                      key={current}
                      aria-hidden="true"
                      className="portal-step-bar absolute inset-x-0 -bottom-px h-0.5 bg-primary"
                      style={{ "--step-ms": `${STEP_MS}ms` } as CSSProperties}
                    />
                  )}
                </li>
              );
            })}
          </ol>

          <div
            id="portal-step-window"
            className="overflow-hidden rounded-[1.375rem] bg-card shadow-md ring-1 ring-border/60 lg:sticky lg:top-24"
          >
            <div className="flex h-11 items-center gap-2 border-b border-border px-4">
              {[0, 1, 2].map((d) => (
                <span key={d} aria-hidden="true" className="size-2.5 rounded-full bg-border" />
              ))}
              <span className="ml-2.5 text-xs text-muted-foreground">
                {t(`portal.step.${key}.window`)}
              </span>
            </div>
            <div key={key} className="portal-rise flex min-h-[26rem] flex-col gap-2.5 p-5 sm:p-6">
              <Scene />
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
