import { flushSync } from "react-dom";

/* Theme switch Signature Moment (spec §7.1): a View Transition that reveals
 * the new Motion World from the switch button — 日蚀 (eclipse, → moonshot)
 * or 墨染 (ink wash, → anthropic). The edge layer (.vt-edge, CSS in
 * styles/motion-signature.css) only exists in the new state and carries its
 * own view-transition-name; since the new view is rendered live, its CSS
 * animation drives the corona / wet-ink ring frame by frame. Locale
 * switching never goes through here (LocaleFade only). */

export interface Point {
  x: number;
  y: number;
}

/** Distance from the origin to the farthest viewport corner. */
export function revealRadius(x: number, y: number, w: number, h: number): number {
  return Math.hypot(Math.max(x, w - x), Math.max(y, h - y));
}

const VT_VARS = ["--vt-x", "--vt-y", "--vt-r"];
let active: ViewTransition | null = null;
let edge: HTMLElement | null = null;

function removeEdge(): void {
  edge?.remove();
  edge = null;
}

/**
 * Runs `apply` inside a themed View Transition. Returns false (without
 * calling `apply`) when the browser lacks View Transitions or the Motion
 * Tier is `still` — the caller then falls back to the plain color fade.
 */
export function startThemeTransition(
  next: "anthropic" | "moonshot",
  origin: Point | undefined,
  apply: () => void,
): boolean {
  const root = document.documentElement;
  const tier = root.dataset.motionTier;
  if (typeof document.startViewTransition !== "function" || tier === "still") return false;

  // Interruptible: a second click finishes the running transition at once.
  active?.skipTransition();
  removeEdge();

  const w = window.innerWidth;
  const h = window.innerHeight;
  const x = origin?.x ?? w / 2;
  const y = origin?.y ?? h / 2;
  root.style.setProperty("--vt-x", `${x}px`);
  root.style.setProperty("--vt-y", `${y}px`);
  root.style.setProperty("--vt-r", `${revealRadius(x, y, w, h)}px`);
  root.dataset.vt = next === "moonshot" ? "eclipse" : "ink";
  const withEdge = tier === "immersive";

  const reset = () => {
    active = null;
    removeEdge();
    delete root.dataset.vt;
    for (const name of VT_VARS) root.style.removeProperty(name);
  };
  // A skipped transition settles after its successor started: only the
  // transition that is still current may reset the shared state.
  const cleanup = (t: ViewTransition) => {
    if (active === t) reset();
  };

  try {
    const t = document.startViewTransition(() => {
      flushSync(apply);
      if (withEdge) {
        edge = document.createElement("div");
        edge.className = "vt-edge";
        edge.setAttribute("aria-hidden", "true");
        document.body.appendChild(edge);
      }
    });
    active = t;
    t.ready.catch(() => {});
    t.finished.finally(() => cleanup(t));
    return true;
  } catch {
    reset();
    return false;
  }
}
