import type { ReactNode } from "react";

/**
 * Route-change entrance for every dashboard page: templates remount on
 * navigation, so each page enters once in the active Motion World's style
 * (--animate-page, styles/motion-*.css). Fill-mode is `backwards`, so after
 * the animation the wrapper imposes no containing-block/stacking side
 * effects (fixed tooltips, sticky headers keep working).
 */
export default function DashboardTemplate({ children }: { children: ReactNode }) {
  return <div className="animate-page">{children}</div>;
}
