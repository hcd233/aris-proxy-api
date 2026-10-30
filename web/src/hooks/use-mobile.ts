import { useCallback, useSyncExternalStore } from "react";

const MOBILE_BREAKPOINT = 768;
/** 平板及以下（<1024px，Tailwind `lg` 之前）：侧栏默认收起、表格开启行详情 */
const COMPACT_BREAKPOINT = 1024;

function useMediaQuery(query: string) {
  const subscribe = useCallback(
    (callback: () => void) => {
      const mql = window.matchMedia(query);
      mql.addEventListener("change", callback);
      return () => mql.removeEventListener("change", callback);
    },
    [query],
  );
  return useSyncExternalStore(
    subscribe,
    () => window.matchMedia(query).matches,
    () => false,
  );
}

export function useIsMobile() {
  return useMediaQuery(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`);
}

/** 小于 1280px：触屏横屏平板与小屏笔记本；只给「布局本来就放不下」的表格降级用 */
export function useIsNarrow() {
  return useMediaQuery("(max-width: 1279px)");
}

export function useIsCompact() {
  return useMediaQuery(`(max-width: ${COMPACT_BREAKPOINT - 1}px)`);
}
