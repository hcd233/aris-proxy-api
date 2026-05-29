"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export interface UseInfiniteListOptions<T> {
  fetcher: (offset: number, limit: number) => Promise<{ items: T[]; total: number }>;
  pageSize: number;
  enabled: boolean;
}

export interface UseInfiniteListResult<T> {
  items: T[];
  total: number;
  loading: boolean;
  hasMore: boolean;
  loadMore: () => Promise<void>;
  reset: () => void;
}

/**
 * 通用"向下滚动加载更多"hook，复用于 messages / tools 两个列表。
 *
 * 行为契约：
 * - enabled=false 时不发起任何请求
 * - reset() 清空 state 并重新从 offset=0 拉首页
 * - loadMore() 内部用 inFlight ref 保证并发安全
 * - 请求失败不抛错，console.warn 后保持现状（与项目现有 try/catch 静默风格一致）
 */
export function useInfiniteList<T>(
  opts: UseInfiniteListOptions<T>
): UseInfiniteListResult<T> {
  const { fetcher, pageSize, enabled } = opts;
  const [items, setItems] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  // inFlightRef 仅用于并发去重（写入和读取都在 loadMore 中，不参与 render）
  const inFlightRef = useRef(false);

  const loadMore = useCallback(async () => {
    if (!enabled) return;
    if (inFlightRef.current) return;

    inFlightRef.current = true;
    setLoading(true);
    try {
      // 用闭包读取最新 offset 不可靠，loadMore 已经被 setOffset 触发的 deps 重算
      const { items: newItems, total: newTotal } = await fetcher(
        offset,
        pageSize,
      );
      // 已 loaded 且没有新条目 → 不再触发 setItems 以避免新引用
      if (newItems.length > 0) {
        setItems((prev) => [...prev, ...newItems]);
        setOffset((prev) => prev + newItems.length);
      }
      setTotal(newTotal);
      setLoaded(true);
    } catch (e) {
      console.warn("[useInfiniteList] load failed", e);
    } finally {
      setLoading(false);
      inFlightRef.current = false;
    }
  }, [enabled, fetcher, offset, pageSize]);

  const reset = useCallback(() => {
    setItems([]);
    setTotal(0);
    setOffset(0);
    setLoaded(false);
  }, []);

  // enabled 切换为 true 或刚 reset 后自动拉首页
  /* eslint-disable react-hooks/set-state-in-effect -- intentional: trigger first-page fetch on enable; loadMore awaits then setState */
  useEffect(() => {
    if (enabled && !loaded) {
      void loadMore();
    }
  }, [enabled, loaded, loadMore]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const hasMore = !loaded ? enabled : offset < total;

  return { items, total, loading, hasMore, loadMore, reset };
}
