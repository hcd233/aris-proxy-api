import type { ReactNode } from "react";

/**
 * Route-change entrance for every dashboard page: templates remount on
 * navigation, so each page rises in once (480ms, transform/opacity only).
 * The final keyframe is `transform: none`, so after the animation the
 * wrapper imposes no containing-block/stacking side effects (fixed
 * tooltips, sticky headers keep working).
 */
export default function DashboardTemplate({ children }: { children: ReactNode }) {
  return <div className="animate-rise">{children}</div>;
}
