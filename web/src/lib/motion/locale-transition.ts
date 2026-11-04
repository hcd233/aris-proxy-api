import { flushSync } from "react-dom";

/* Locale switch: a View Transition that cross-fades the whole page from the
 * old language to the new one. Layout is Locale-Stable (same type scale,
 * reserved widths, reserved scrollbar gutters), so the snapshots line up and
 * only the glyphs morph — no blank frame in between. Styles live under
 * html[data-locale-vt] in styles/motion-signature.css, kept apart from the
 * theme transition's html[data-vt] so neither cleanup clobbers the other. */

let active: ViewTransition | null = null;

/**
 * Runs `apply` inside a cross-fade View Transition. Returns false (without
 * calling `apply`) when the browser lacks View Transitions or the Motion
 * Tier is `still` — the caller then switches instantly.
 */
export function startLocaleTransition(apply: () => void): boolean {
  const root = document.documentElement;
  if (typeof document.startViewTransition !== "function" || root.dataset.motionTier === "still") {
    return false;
  }

  // Interruptible: a second switch finishes the running cross-fade at once.
  active?.skipTransition();
  root.dataset.localeVt = "";

  try {
    const t = document.startViewTransition(() => flushSync(apply));
    active = t;
    t.ready.catch(() => {});
    t.finished.finally(() => {
      // A skipped transition settles after its successor started.
      if (active !== t) return;
      active = null;
      delete root.dataset.localeVt;
    });
    return true;
  } catch {
    delete root.dataset.localeVt;
    return false;
  }
}
