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
  const [loading, setLoading] = useState(false);
  const inFlightRef = useRef(false);
  const offsetRef = useRef(0);
  const totalLoadedRef = useRef(false);

  const loadMore = useCallback(async () => {
    if (!enabled) return;
    if (inFlightRef.current) return;
    if (totalLoadedRef.current && offsetRef.current >= total) return;

    inFlightRef.current = true;
    setLoading(true);
    try {
      const { items: newItems, total: newTotal } = await fetcher(
        offsetRef.current,
        pageSize
      );
      setItems((prev) => [...prev, ...newItems]);
      setTotal(newTotal);
      totalLoadedRef.current = true;
      offsetRef.current += newItems.length;
    } catch (e) {
      console.warn("[useInfiniteList] load failed", e);
    } finally {
      setLoading(false);
      inFlightRef.current = false;
    }
  }, [enabled, fetcher, pageSize, total]);

  const reset = useCallback(() => {
    setItems([]);
    setTotal(0);
    offsetRef.current = 0;
    totalLoadedRef.current = false;
  }, []);

  // enabled 切换为 true 或刚 reset 后自动拉首页
  useEffect(() => {
    if (enabled && !totalLoadedRef.current && !inFlightRef.current) {
      void loadMore();
    }
  }, [enabled, loadMore]);

  const hasMore = !totalLoadedRef.current ? enabled : offsetRef.current < total;

  return { items, total, loading, hasMore, loadMore, reset };
}
