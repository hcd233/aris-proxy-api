"use client";

import { useEffect, useRef } from "react";

export interface UseAutoRefreshOptions {
  enabled: boolean;
  intervalMs: number;
  onRefresh: () => void;
}

/**
 * 定时刷新开关：关闭时不留定时器；回调走 ref，调用方每次渲染重建闭包
 * 不会重置计时。开关打开时的首次刷新由调用方在切换处立即执行。
 */
export function useAutoRefresh({ enabled, intervalMs, onRefresh }: UseAutoRefreshOptions) {
  const onRefreshRef = useRef(onRefresh);

  useEffect(() => {
    onRefreshRef.current = onRefresh;
  }, [onRefresh]);

  useEffect(() => {
    if (!enabled) return;
    const timer = window.setInterval(() => onRefreshRef.current(), intervalMs);
    return () => window.clearInterval(timer);
  }, [enabled, intervalMs]);
}
