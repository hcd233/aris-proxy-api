"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { startThemeTransition, type Point } from "@/lib/motion/theme-transition";

/* ============================================================
 * Theme registry
 * ------------------------------------------------------------
 * Add a new skin: append a ThemeName, register it in THEMES,
 * add a CSS `[data-theme="<name>"]` block in globals.css, and
 * extend the color palettes below. No other code needs to change.
 * ========================================================== */

export type ThemeName = "anthropic" | "moonshot";

const DEFAULT_THEME: ThemeName = "anthropic";

function isThemeName(x: string | null | undefined): x is ThemeName {
  return x === "anthropic" || x === "moonshot";
}

/** Per-theme display metadata consumed by non-CSS consumers (e.g. sonner). */
export interface ThemeMeta {
  /** Maps our skin onto sonner's light/dark/system theming. */
  sonner: "light" | "dark" | "system";
}

export const THEMES: Record<ThemeName, ThemeMeta> = {
  anthropic: { sonner: "light" },
  moonshot: { sonner: "dark" },
};

/* ============================================================
 * Color palettes
 * recharts needs raw color strings, so palettes are plain data
 * keyed by theme — no CSS variables, no runtime resolution.
 * ========================================================== */

const CHART_SERIES_COLORS: Record<ThemeName, readonly string[]> = {
  anthropic: [
    "#D97757",
    "#5B8DB8",
    "#7C6BA5",
    "#4A9E7D",
    "#C76B8A",
    "#8B7355",
    "#6B8BA4",
    "#A0522D",
  ],
  /* Moonlit neutrals: achromatic greys lead; desaturated cool tints
   * (moonlight cream, ice blue) only where series must diverge. */
  moonshot: [
    "#FFFFFF",
    "#BFBFBF",
    "#FFEED9",
    "#8C8C8C",
    "#A8BDD8",
    "#595959",
    "#D9CBB8",
    "#6E7B8C",
  ],
};

export interface TokenLayerColors {
  cacheRead: string;
  input: string;
  cacheCreated: string;
  output: string;
}

/* Token four-dimensional palette, shared by the composition ratio bars and
 * the token throughput time-series. Input/output carry the theme's two deep
 * hues, and each cache layer is the pale tint of its counterpart
 * (cacheRead = light input, cacheWrite = light output), so the four segments
 * read as two dark/light families instead of four unrelated hues.
 *
 * Charts stack these by luminance (dark at the bottom, light on top), so the
 * four values must stay distinguishable both from each other and from the
 * card background. */
const RATIO_SEGMENT_COLORS: Record<ThemeName, TokenLayerColors> = {
  anthropic: { input: "#B8654A", output: "#5C4A3D", cacheRead: "#EFC5AC", cacheCreated: "#CDBBA6" },
  /* Moonshot stays inside its own token set: moon-rock grey and moon-shadow
   * slate as the two darks, their pale forms being the warm moonlight cream
   * and the ice-blue haze already used by the theme's charts. */
  moonshot: { input: "#737373", output: "#6E7B8C", cacheRead: "#FFEED9", cacheCreated: "#A8BDD8" },
};

export interface StatusColors {
  positive: string;
  warning: string;
  negative: string;
  muted: string;
}

/* Status palette for recharts (mirrors --positive/--warning/--negative in
 * globals.css; recharts needs raw strings). Used by the dashboard reliability
 * bars and health dots. */
const STATUS_COLORS: Record<ThemeName, StatusColors> = {
  anthropic: { positive: "#4F7F6F", warning: "#B9852F", negative: "#B3564F", muted: "#8C7E72" },
  moonshot: { positive: "#A8BDD8", warning: "#FFEED9", negative: "#E07A80", muted: "#595959" },
};

function chartSeriesColors(theme: ThemeName): readonly string[] {
  return CHART_SERIES_COLORS[theme];
}

function tokenLayerColors(theme: ThemeName): TokenLayerColors {
  return RATIO_SEGMENT_COLORS[theme];
}

/* ============================================================
 * Inline init script (FOUC prevention)
 * ------------------------------------------------------------
 * Rendered as a raw synchronous <script> in <head> by the root
 * layout. It runs DURING HTML parsing, before <body> is parsed
 * or painted, so the correct [data-theme] is on <html> before
 * first paint. Must not depend on React or any chunk.
 * ========================================================== */

export const THEME_INIT_SCRIPT =
  "(function(){try{var t=localStorage.getItem('theme');document.documentElement.dataset.theme=(t==='moonshot')?'moonshot':'anthropic';}catch(e){document.documentElement.dataset.theme='anthropic';}})();";

/* ============================================================
 * ThemeProvider + useTheme
 * ------------------------------------------------------------
 * Single source of truth for the active theme. Components read
 * via useTheme(); mutations go through setTheme() which updates
 * <html data-theme>, localStorage and context state atomically.
 * Replaces the three separate MutationObservers that previously
 * watched <html data-theme> from theme.ts, theme-switcher and
 * particle-background.
 * ========================================================== */

interface ThemeContextValue {
  theme: ThemeName;
  /** `origin` (viewport px) anchors the theme-switch View Transition. */
  setTheme: (next: ThemeName, origin?: Point) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

const TRANSITION_MS = 400;

export function ThemeProvider({ children }: { children: ReactNode }) {
  // The inline init script guarantees <html data-theme> is set
  // before hydration, so this is safe. We keep DEFAULT_THEME on
  // both server and first client render to stay hydration-safe;
  // the effect syncs the real value right after mount.
  const [theme, setThemeState] = useState<ThemeName>(DEFAULT_THEME);

  /* eslint-disable react-hooks/set-state-in-effect -- Reading <html data-theme> (set by inline script) on mount requires setting state in effect */
  useEffect(() => {
    const current = document.documentElement.dataset.theme;
    setThemeState(isThemeName(current) ? current : DEFAULT_THEME);
  }, []);
  /* eslint-enable react-hooks/set-state-in-effect */

  const setTheme = (next: ThemeName, origin?: Point) => {
    const root = document.documentElement;
    const apply = () => {
      root.dataset.theme = next;
      try {
        localStorage.setItem("theme", next);
      } catch {
        // private mode etc.: theme still applies for this session
      }
      setThemeState(next);
    };
    // Signature Moment: eclipse / ink-wash View Transition between worlds.
    if (startThemeTransition(next, origin, apply)) return;
    // Fallback (no View Transitions, or Motion Tier `still`): color fade.
    root.classList.add("theme-transition");
    apply();
    window.setTimeout(() => root.classList.remove("theme-transition"), TRANSITION_MS);
  };

  return <ThemeContext.Provider value={{ theme, setTheme }}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return ctx;
}

/* ============================================================
 * Chart color hooks (backward-compatible with the old API)
 * Components call these and re-render when the theme changes.
 * ========================================================== */

export function useChartSeriesColors(): readonly string[] {
  return chartSeriesColors(useTheme().theme);
}

export function useTokenLayerColors(): TokenLayerColors {
  return tokenLayerColors(useTheme().theme);
}

export function useStatusColors(): StatusColors {
  return STATUS_COLORS[useTheme().theme];
}

export function useRatioSegmentColors(): TokenLayerColors {
  return RATIO_SEGMENT_COLORS[useTheme().theme];
}
