"use client";

import { useEffect, useState } from "react";

export type ThemeName = "anthropic" | "moonshot";

function readTheme(): ThemeName {
  if (typeof document === "undefined") return "anthropic";
  return document.documentElement.dataset.theme === "moonshot" ? "moonshot" : "anthropic";
}

/**
 * Current theme from <html data-theme>, kept in sync via MutationObserver so
 * components re-render when ThemeSwitcher toggles without a page reload.
 * Initial value is "anthropic" on both server and client to stay
 * hydration-safe; the effect syncs the real value on mount.
 */
export function useThemeName(): ThemeName {
  const [theme, setTheme] = useState<ThemeName>("anthropic");

  useEffect(() => {
    const root = document.documentElement;
    const sync = () => setTheme(readTheme());
    sync();
    const observer = new MutationObserver(sync);
    observer.observe(root, { attributes: true, attributeFilter: ["data-theme"] });
    return () => observer.disconnect();
  }, []);

  return theme;
}

/** Series palette for multi-line/bar charts (recharts needs raw color strings). */
const CHART_SERIES_COLORS: Record<ThemeName, readonly string[]> = {
  anthropic: ["#D97757", "#5B8DB8", "#7C6BA5", "#4A9E7D", "#C76B8A", "#8B7355", "#6B8BA4", "#A0522D"],
  moonshot: ["#7C8CFF", "#6FE3E0", "#9DB4FF", "#B98BC9", "#5E6EE0", "#8AB4D6", "#78C3A3", "#E79AB4"],
};

export function useChartSeriesColors(): readonly string[] {
  return CHART_SERIES_COLORS[useThemeName()];
}

/** Stacked token-volume layers: cache layers are light tints, real I/O is saturated. */
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

export function useTokenLayerColors(): TokenLayerColors {
  return TOKEN_LAYER_COLORS[useThemeName()];
}
