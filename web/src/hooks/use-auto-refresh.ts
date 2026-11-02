"use client";

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";

export interface AutoRefresh {
  /** 每次重新开始计时自增；倒计时环以它作为 key 重放动画 */
  cycle: number;
  /** 立即刷新并重新开始计时 */
  refreshNow: () => void;
}

function subscribeVisibility(onChange: () => void) {
  document.addEventListener("visibilitychange", onChange);
  return () => document.removeEventListener("visibilitychange", onChange);
}

const isVisible = () => document.visibilityState !== "hidden";
const isVisibleOnServer = () => true;

/**
 * 定时刷新：intervalMs ≤ 0 表示关闭。用 setTimeout 链而非 setInterval，
 * 每轮都从「上一次刷新」起算，手动刷新会重置倒计时。标签页隐藏时停表，
 * 回到前台立即刷新一次再继续。
 */
export function useAutoRefresh(intervalMs: number, onRefresh: () => void): AutoRefresh {
  const onRefreshRef = useRef(onRefresh);
  useEffect(() => {
    onRefreshRef.current = onRefresh;
  }, [onRefresh]);

  const [cycle, setCycle] = useState(0);
  const visible = useSyncExternalStore(subscribeVisibility, isVisible, isVisibleOnServer);
  const enabled = intervalMs > 0;

  const refreshNow = useCallback(() => {
    onRefreshRef.current();
    setCycle((c) => c + 1);
  }, []);

  useEffect(() => {
    if (!enabled || !visible) return;
    const timer = window.setTimeout(refreshNow, intervalMs);
    return () => window.clearTimeout(timer);
  }, [enabled, visible, intervalMs, cycle, refreshNow]);

  const wasVisible = useRef(visible);
  useEffect(() => {
    // 从后台回到前台：数据可能已过期很久，先补一次再恢复计时
    if (enabled && visible && !wasVisible.current) refreshNow();
    wasVisible.current = visible;
  }, [enabled, visible, refreshNow]);

  return { cycle, refreshNow };
}
