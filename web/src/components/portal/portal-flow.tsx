"use client";

import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { ProviderIcon } from "@/components/provider-icon";
import { useI18n, useT } from "@/lib/i18n";
import { useMotionTier } from "@/lib/motion/tier-store";
import {
  PORTAL_CLIENTS,
  PORTAL_GATEWAY_STAGES,
  PORTAL_SECTION_IDS,
  PORTAL_UPSTREAMS,
  type PortalEntity,
} from "@/lib/portal";
import { curvePath, curvePathVertical } from "@/lib/portal-motion";
import { cn } from "@/lib/utils";
import { PortalSectionHead } from "./portal-hero";
import { Reveal } from "./reveal";
import { usePortalLoop } from "./use-portal-loop";

type Paths = { inbound: string[]; outbound: string[]; w: number; h: number };
type Phase =
  | { kind: "idle" }
  | { kind: "in"; client: number; upstream: number }
  | { kind: "stage"; client: number; upstream: number; stage: number }
  | { kind: "out"; client: number; upstream: number };

const TRAVEL_MS = 720;
const STAGE_MS = 150;

function FlowNode({
  entity,
  kind,
  hot,
  ref,
}: {
  entity: PortalEntity;
  kind: "client" | "upstream";
  hot: boolean;
  ref: React.Ref<HTMLLIElement>;
}) {
  const t = useT();
  return (
    <li
      ref={ref}
      data-flow-node={kind}
      data-hot={hot || undefined}
      className={cn(
        "flex h-10 items-center gap-2.5 rounded-xl bg-background px-3.5 text-sm text-foreground ring-1 ring-border transition-[box-shadow,transform] duration-300",
        hot && "scale-[1.03] ring-[1.5px] ring-primary",
      )}
    >
      <ProviderIcon decorative protocol={entity.icon} size={18} />
      <span className="whitespace-nowrap">
        {entity.labelKey ? t(entity.labelKey) : entity.label}
      </span>
    </li>
  );
}

export function PortalFlow() {
  const t = useT();
  const { locale } = useI18n();
  const tier = useMotionTier();
  const boxRef = useRef<HTMLDivElement>(null);
  const coreRef = useRef<HTMLDivElement>(null);
  const clientRefs = useRef<(HTMLLIElement | null)[]>([]);
  const upstreamRefs = useRef<(HTMLLIElement | null)[]>([]);
  const inRefs = useRef<(SVGPathElement | null)[]>([]);
  const outRefs = useRef<(SVGPathElement | null)[]>([]);
  const dotRef = useRef<SVGCircleElement>(null);
  const [paths, setPaths] = useState<Paths>({ inbound: [], outbound: [], w: 0, h: 0 });
  const [phase, setPhase] = useState<Phase>({ kind: "idle" });

  const measure = useCallback(() => {
    const box = boxRef.current?.getBoundingClientRect();
    const core = coreRef.current?.getBoundingClientRect();
    if (!box || !core) return;
    const stacked = core.top > (clientRefs.current[0]?.getBoundingClientRect().bottom ?? 0) + 4;
    const rel = (r: DOMRect) => ({
      l: r.left - box.left,
      r: r.right - box.left,
      t: r.top - box.top,
      b: r.bottom - box.top,
      cx: r.left - box.left + r.width / 2,
      cy: r.top - box.top + r.height / 2,
    });
    const c = rel(core);
    const spread = (i: number, n: number, from: number, to: number) =>
      from + ((i + 1) * (to - from)) / (n + 1);
    const nodes = (refs: (HTMLLIElement | null)[]) =>
      refs.flatMap((el) => (el ? [rel(el.getBoundingClientRect())] : []));
    const cl = nodes(clientRefs.current);
    const up = nodes(upstreamRefs.current);
    const inbound = cl.map((n, i) =>
      stacked
        ? curvePathVertical(n.cx, n.b, spread(i, cl.length, c.l + 16, c.r - 16), c.t)
        : curvePath(n.r, n.cy, c.l, spread(i, cl.length, c.t + 16, c.b - 16)),
    );
    const outbound = up.map((n, i) =>
      stacked
        ? curvePathVertical(spread(i, up.length, c.l + 16, c.r - 16), c.b, n.cx, n.t)
        : curvePath(c.r, spread(i, up.length, c.t + 16, c.b - 16), n.l, n.cy),
    );
    setPaths({ inbound, outbound, w: box.width, h: box.height });
  }, []);

  useLayoutEffect(() => {
    measure();
    const ro = new ResizeObserver(measure);
    if (boxRef.current) ro.observe(boxRef.current);
    for (const el of [...clientRefs.current, ...upstreamRefs.current, coreRef.current])
      if (el) ro.observe(el);
    void document.fonts?.ready.then(measure);
    return () => ro.disconnect();
  }, [measure, locale]);

  const travel = useCallback((path: SVGPathElement | null) => {
    const dot = dotRef.current;
    if (!path || !dot) return;
    const len = path.getTotalLength();
    const start = performance.now();
    dot.style.opacity = "1";
    const frame = (now: number) => {
      const k = Math.min(1, (now - start) / TRAVEL_MS);
      const e = k * k * (3 - 2 * k);
      const p = path.getPointAtLength(len * e);
      dot.setAttribute("cx", String(p.x));
      dot.setAttribute("cy", String(p.y));
      if (k < 1) requestAnimationFrame(frame);
      else dot.style.opacity = "0";
    };
    requestAnimationFrame(frame);
  }, []);

  const phaseRef = useRef<Phase>(phase);
  const advance = useCallback(() => {
    const p = phaseRef.current;
    let next: Phase;
    if (p.kind === "idle") {
      next = {
        kind: "in",
        client: Math.floor(Math.random() * PORTAL_CLIENTS.length),
        upstream: Math.floor(Math.random() * PORTAL_UPSTREAMS.length),
      };
      if (tier === "immersive") travel(inRefs.current[next.client]);
    } else if (p.kind === "in") {
      next = { ...p, kind: "stage", stage: 0 };
    } else if (p.kind === "stage" && p.stage < PORTAL_GATEWAY_STAGES.length - 1) {
      next = { ...p, stage: p.stage + 1 };
    } else if (p.kind === "stage") {
      next = { kind: "out", client: p.client, upstream: p.upstream };
      if (tier === "immersive") travel(outRefs.current[p.upstream]);
    } else {
      next = { kind: "idle" };
    }
    phaseRef.current = next;
    setPhase(next);
  }, [tier, travel]);

  usePortalLoop(advance, () => {
    if (phase.kind === "stage") return STAGE_MS;
    if (phase.kind === "idle") return 260;
    if (phase.kind === "out") return TRAVEL_MS + 460;
    return TRAVEL_MS;
  });

  const active = phase.kind === "idle" ? null : phase;
  const clientHot = (i: number) => active?.client === i;
  const upstreamHot = (i: number) => active?.kind === "out" && active.upstream === i;
  const coreHot = active?.kind === "stage";

  return (
    <section id={PORTAL_SECTION_IDS.flow} className="scroll-mt-24 px-5 pt-24 pb-10 sm:px-8">
      <div className="mx-auto max-w-6xl">
        <Reveal>
          <PortalSectionHead
            eyebrowKey="portal.flow.eyebrow"
            titleKey="portal.flow.heading"
            subtitleKey="portal.flow.subtitle"
          />
        </Reveal>
        <Reveal>
          <div
            ref={boxRef}
            className="relative grid items-center gap-10 overflow-hidden rounded-[1.75rem] bg-card p-6 shadow-md ring-1 ring-border/60 sm:p-10 lg:grid-cols-[14rem_1fr_14rem] lg:gap-0"
          >
            <svg
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 size-full"
              viewBox={`0 0 ${paths.w || 1} ${paths.h || 1}`}
            >
              {paths.inbound.map((d, i) => (
                <path
                  key={`in-${i}`}
                  ref={(el) => {
                    inRefs.current[i] = el;
                  }}
                  d={d}
                  data-flow-path="in"
                  data-hot={clientHot(i) || undefined}
                  className="portal-dash"
                />
              ))}
              {paths.outbound.map((d, i) => (
                <path
                  key={`out-${i}`}
                  ref={(el) => {
                    outRefs.current[i] = el;
                  }}
                  d={d}
                  data-flow-path="out"
                  data-hot={upstreamHot(i) || undefined}
                  className="portal-dash"
                />
              ))}
              <circle ref={dotRef} r={4} className="fill-primary opacity-0" />
            </svg>

            <div className="relative flex flex-col gap-2">
              <p className="mb-1 text-center text-xs text-muted-foreground">
                {t("portal.flow.clients")}
              </p>
              <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
                {PORTAL_CLIENTS.map((c, i) => (
                  <FlowNode
                    key={c.id}
                    ref={(el) => {
                      clientRefs.current[i] = el;
                    }}
                    entity={c}
                    kind="client"
                    hot={clientHot(i)}
                  />
                ))}
              </ul>
            </div>

            <div
              ref={coreRef}
              data-flow-core=""
              className={cn(
                "relative mx-auto w-full max-w-[15rem] rounded-[1.375rem] bg-background px-6 py-8 text-center ring-[1.5px] ring-primary transition-shadow duration-300 lg:w-60",
                coreHot
                  ? "shadow-[0_0_0_16px_color-mix(in_oklab,var(--primary)_12%,transparent)]"
                  : "shadow-[0_0_0_8px_color-mix(in_oklab,var(--primary)_7%,transparent)]",
              )}
            >
              <p className="font-display text-3xl font-normal tracking-tight text-foreground">
                Aris
              </p>
              <ul className="mt-4 grid grid-cols-2 gap-1.5">
                {PORTAL_GATEWAY_STAGES.map((k, i) => (
                  <li
                    key={k}
                    className={cn(
                      "rounded-lg py-1 text-xs text-muted-foreground ring-1 ring-border transition-colors duration-200",
                      active?.kind === "stage" &&
                        active.stage === i &&
                        "bg-primary text-primary-foreground ring-primary",
                    )}
                  >
                    {t(k)}
                  </li>
                ))}
              </ul>
            </div>

            <div className="relative flex flex-col gap-2">
              <p className="mb-1 text-center text-xs text-muted-foreground">
                {t("portal.flow.upstreams")}
              </p>
              <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-1">
                {PORTAL_UPSTREAMS.map((u, i) => (
                  <FlowNode
                    key={u.id}
                    ref={(el) => {
                      upstreamRefs.current[i] = el;
                    }}
                    entity={u}
                    kind="upstream"
                    hot={upstreamHot(i)}
                  />
                ))}
              </ul>
            </div>
          </div>
        </Reveal>
      </div>
    </section>
  );
}
