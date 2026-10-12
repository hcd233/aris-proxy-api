"use client";

import { useEffect } from "react";
import { useMotionTier } from "@/lib/motion/tier-store";
import { emitAmbientPulse } from "@/lib/motion/pulse";
import { lampShadow, magnetTarget, tiltFor, type Box } from "@/lib/motion/interaction";
import { springAtRest, springStep, type SpringState } from "@/lib/motion/spring";
import { MAGNETIC_SPRING } from "@/lib/motion/tokens";

const PRIMARY_SELECTOR = '[data-slot="button"][data-variant="default"], [data-magnetic]';
const SPOT_SELECTOR =
  '[data-slot="button"][data-variant="ghost"], [data-slot="button"][data-variant="outline"]';
const CARD_SELECTOR = '[data-slot="card"]';
const DISABLED_SELECTOR = ':disabled, [aria-disabled="true"], [data-disabled]';
const DIALOG_OPEN_SELECTOR =
  '[data-slot="dialog-content"][data-open], [data-slot="alert-dialog-content"][data-open]';
const CARD_VARS = ["--mx", "--my", "--rx", "--ry"];
const SPOT_VARS = ["--mx", "--my"];
const MAG_VARS = ["--mag-x", "--mag-y"];
/** The lamp only moves once the pointer rests (keeps style recalcs rare). */
const LAMP_SETTLE_MS = 120;
const SHOCKWAVE_TTL_MS = 1000;
const MAX_SPRING_DT_S = 0.05;

interface Magnet {
  x: SpringState;
  y: SpringState;
  tx: number;
  ty: number;
}

function isMoonshot(): boolean {
  return document.documentElement.dataset.theme === "moonshot";
}

function clearVars(el: HTMLElement, names: readonly string[]): void {
  for (const name of names) el.style.removeProperty(name);
}

function setPointerVars(el: HTMLElement, r: Box, px: number, py: number): void {
  el.style.setProperty("--mx", `${(px - r.left).toFixed(1)}px`);
  el.style.setProperty("--my", `${(py - r.top).toFixed(1)}px`);
}

function spawnShockwave(x: number, y: number): void {
  const ring = document.createElement("span");
  ring.className = "motion-shockwave";
  ring.setAttribute("aria-hidden", "true");
  ring.style.left = `${x}px`;
  ring.style.top = `${y}px`;
  ring.addEventListener("animationend", () => ring.remove(), { once: true });
  document.body.appendChild(ring);
  window.setTimeout(() => ring.remove(), SHOCKWAVE_TTL_MS);
}

/**
 * Interaction Layer (spec §4.1 ④): one set of document-level listeners that
 * drives every pointer-reactive effect of the active Motion World by writing
 * CSS variables — no per-component wiring, no React re-renders.
 * - all worlds: primary presses → `primary` Ambient Pulse; dialogs → focus pulses
 * - anthropic (immersive): lamp shadow direction (--light-x / --light-y on <html>)
 * - moonshot (fine pointer): card rim light + tilt, ghost-button light spot,
 *   magnetic primary buttons, click shockwave
 */
export function InteractionLayer() {
  const tier = useMotionTier();

  useEffect(() => {
    if (tier === "still") return;
    const root = document.documentElement;
    const finePointer = window.matchMedia("(hover: hover) and (pointer: fine)").matches;
    const magnets = new Map<HTMLElement, Magnet>();
    let pointer: { x: number; y: number; target: Element | null } | null = null;
    let hoverCard: HTMLElement | null = null;
    let hoverSpot: HTMLElement | null = null;
    let moveRaf = 0;
    let springRaf = 0;
    let lastSpring = 0;
    let lampTimer = 0;
    let dialogRaf = 0;
    let dialogOpen = false;

    const stepSprings = (now: number) => {
      springRaf = 0;
      const dt = Math.min((now - lastSpring) / 1000, MAX_SPRING_DT_S);
      lastSpring = now;
      for (const [el, m] of magnets) {
        m.x = springStep(m.x, m.tx, dt, MAGNETIC_SPRING);
        m.y = springStep(m.y, m.ty, dt, MAGNETIC_SPRING);
        const idle = m.tx === 0 && m.ty === 0 && springAtRest(m.x, 0) && springAtRest(m.y, 0);
        if (!el.isConnected || idle) {
          el.removeAttribute("data-magnetic-active");
          clearVars(el, MAG_VARS);
          magnets.delete(el);
          continue;
        }
        el.setAttribute("data-magnetic-active", "");
        el.style.setProperty("--mag-x", m.x.x.toFixed(2));
        el.style.setProperty("--mag-y", m.y.x.toFixed(2));
      }
      if (magnets.size > 0) springRaf = requestAnimationFrame(stepSprings);
    };

    const kickSprings = () => {
      if (springRaf || magnets.size === 0) return;
      lastSpring = performance.now();
      springRaf = requestAnimationFrame(stepSprings);
    };

    const releaseAll = () => {
      for (const m of magnets.values()) {
        m.tx = 0;
        m.ty = 0;
      }
      kickSprings();
      if (hoverCard) clearVars(hoverCard, CARD_VARS);
      if (hoverSpot) clearVars(hoverSpot, SPOT_VARS);
      hoverCard = null;
      hoverSpot = null;
    };

    const flushMove = () => {
      moveRaf = 0;
      if (!pointer) return;
      const { x, y, target } = pointer;
      if (!isMoonshot() || !finePointer) {
        releaseAll();
        return;
      }

      // Reads first (rects), then writes (CSS vars) — no layout thrash.
      const card = target?.closest<HTMLElement>(CARD_SELECTOR) ?? null;
      const spot = target?.closest<HTMLElement>(SPOT_SELECTOR) ?? null;
      const cardRect = card?.getBoundingClientRect();
      const spotRect = spot?.getBoundingClientRect();
      const candidates = Array.from(document.querySelectorAll<HTMLElement>(PRIMARY_SELECTOR));
      const pulls = candidates.map((el) => {
        if (el.matches(DISABLED_SELECTOR)) return { x: 0, y: 0 };
        const r = el.getBoundingClientRect();
        if (r.width === 0 || r.height === 0) return { x: 0, y: 0 };
        // Measure against the rest position, not the displaced one.
        const m = magnets.get(el);
        const rest = m ? { ...r.toJSON(), left: r.left - m.x.x, top: r.top - m.y.x } : r;
        return magnetTarget(rest, x, y);
      });

      if (card !== hoverCard && hoverCard) clearVars(hoverCard, CARD_VARS);
      hoverCard = card;
      if (card && cardRect) {
        setPointerVars(card, cardRect, x, y);
        if (card.hasAttribute("data-interactive")) {
          const t = tiltFor(cardRect, x, y);
          card.style.setProperty("--rx", `${t.rx.toFixed(2)}deg`);
          card.style.setProperty("--ry", `${t.ry.toFixed(2)}deg`);
        }
      }

      if (spot !== hoverSpot && hoverSpot) clearVars(hoverSpot, SPOT_VARS);
      hoverSpot = spot;
      if (spot && spotRect) setPointerVars(spot, spotRect, x, y);

      candidates.forEach((el, i) => {
        const pull = pulls[i];
        const m = magnets.get(el);
        if (m) {
          m.tx = pull.x;
          m.ty = pull.y;
        } else if (pull.x !== 0 || pull.y !== 0) {
          magnets.set(el, { x: { x: 0, v: 0 }, y: { x: 0, v: 0 }, tx: pull.x, ty: pull.y });
        }
      });
      for (const [el, m] of magnets) {
        if (!candidates.includes(el)) {
          m.tx = 0;
          m.ty = 0;
        }
      }
      kickSprings();
    };

    const settleLamp = () => {
      lampTimer = 0;
      if (!pointer || isMoonshot() || root.dataset.motionTier !== "immersive") return;
      const s = lampShadow(pointer.x, pointer.y, window.innerWidth, window.innerHeight);
      root.style.setProperty("--light-x", String(s.x));
      root.style.setProperty("--light-y", String(s.y));
    };

    const onMove = (e: PointerEvent) => {
      pointer = {
        x: e.clientX,
        y: e.clientY,
        target: e.target instanceof Element ? e.target : null,
      };
      if (!moveRaf) moveRaf = requestAnimationFrame(flushMove);
      window.clearTimeout(lampTimer);
      lampTimer = window.setTimeout(settleLamp, LAMP_SETTLE_MS);
    };

    const onOut = (e: PointerEvent) => {
      if (e.relatedTarget === null) {
        pointer = null;
        releaseAll();
      }
    };

    const onDown = (e: PointerEvent) => {
      if (e.button !== 0 || !(e.target instanceof Element)) return;
      const el = e.target.closest(PRIMARY_SELECTOR);
      if (!el || el.matches(DISABLED_SELECTOR)) return;
      emitAmbientPulse({ kind: "primary", x: e.clientX, y: e.clientY });
      if (isMoonshot()) spawnShockwave(e.clientX, e.clientY);
    };

    const checkDialog = () => {
      dialogRaf = 0;
      const open = document.querySelector(DIALOG_OPEN_SELECTOR) !== null;
      if (open === dialogOpen) return;
      dialogOpen = open;
      emitAmbientPulse({ kind: open ? "focus-on" : "focus-off" });
    };
    const observer = new MutationObserver(() => {
      if (!dialogRaf) dialogRaf = requestAnimationFrame(checkDialog);
    });
    observer.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["data-open", "data-closed"],
    });

    window.addEventListener("pointermove", onMove, { passive: true });
    window.addEventListener("pointerdown", onDown, { capture: true, passive: true });
    document.addEventListener("pointerout", onOut);

    return () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerdown", onDown, { capture: true });
      document.removeEventListener("pointerout", onOut);
      observer.disconnect();
      cancelAnimationFrame(moveRaf);
      cancelAnimationFrame(springRaf);
      cancelAnimationFrame(dialogRaf);
      window.clearTimeout(lampTimer);
      if (hoverCard) clearVars(hoverCard, CARD_VARS);
      if (hoverSpot) clearVars(hoverSpot, SPOT_VARS);
      for (const el of magnets.keys()) {
        el.removeAttribute("data-magnetic-active");
        clearVars(el, MAG_VARS);
      }
      magnets.clear();
      root.style.removeProperty("--light-x");
      root.style.removeProperty("--light-y");
      if (dialogOpen) emitAmbientPulse({ kind: "focus-off" });
    };
  }, [tier]);

  return null;
}
