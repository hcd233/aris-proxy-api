"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

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
  anthropic: ["#D97757", "#5F8A8B", "#B8654A", "#7A8450", "#8C7E72", "#C98A5F", "#4F6F52", "#5B7B8C"],
  moonshot: ["#7C8CFF", "#6FE3E0", "#9DB4FF", "#B98BC9", "#5E6EE0", "#8AB4D6", "#78C3A3", "#E79AB4"],
};

export interface TokenLayerColors {
  cacheRead: string;
  input: string;
  cacheCreated: string;
  output: string;
}

const TOKEN_LAYER_COLORS: Record<ThemeName, TokenLayerColors> = {
  anthropic: { cacheRead: "#F2D0B8", input: "#E6733F", cacheCreated: "#F2D5BE", output: "#D46A3E" },
  moonshot: { cacheRead: "#A9B8FF", input: "#7C8CFF", cacheCreated: "#9CF0EC", output: "#5E6EE0" },
};

function chartSeriesColors(theme: ThemeName): readonly string[] {
  return CHART_SERIES_COLORS[theme];
}

function tokenLayerColors(theme: ThemeName): TokenLayerColors {
  return TOKEN_LAYER_COLORS[theme];
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
  setTheme: (next: ThemeName) => void;
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

  const setTheme = (next: ThemeName) => {
    const root = document.documentElement;
    root.classList.add("theme-transition");
    root.dataset.theme = next;
    try {
      localStorage.setItem("theme", next);
    } catch {
      // private mode etc.: theme still applies for this session
    }
    setThemeState(next);
    window.setTimeout(
      () => root.classList.remove("theme-transition"),
      TRANSITION_MS
    );
  };

  return (
    <ThemeContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
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
