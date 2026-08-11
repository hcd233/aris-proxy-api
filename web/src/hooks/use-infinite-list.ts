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
export function useInfiniteList<T>(opts: UseInfiniteListOptions<T>): UseInfiniteListResult<T> {
  const { fetcher, pageSize, enabled } = opts;
  const [items, setItems] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  // inFlightRef 仅用于并发去重（写入和读取都在 loadMore 中，不参与 render）
  const inFlightRef = useRef(false);
  // generationRef：reset 作废在途请求的凭据，避免旧响应污染新列表
  const generationRef = useRef(0);
  const [generation, setGeneration] = useState(0);

  const loadMore = useCallback(async () => {
    if (!enabled) return;
    if (inFlightRef.current) return;

    const gen = generationRef.current;
    inFlightRef.current = true;
    setLoading(true);
    try {
      // 用闭包读取最新 offset 不可靠，loadMore 已经被 setOffset 触发的 deps 重算
      const { items: newItems, total: newTotal } = await fetcher(offset, pageSize);
      // reset 已发生 → 丢弃本次响应
      if (gen !== generationRef.current) return;
      // 已 loaded 且没有新条目 → 不再触发 setItems 以避免新引用
      if (newItems.length > 0) {
        setItems((prev) => [...prev, ...newItems]);
        setOffset((prev) => prev + newItems.length);
      }
      setTotal(newTotal);
      setLoaded(true);
    } catch (e) {
      if (gen !== generationRef.current) return;
      console.warn("[useInfiniteList] load failed", e);
    } finally {
      // 仅当没有发生 reset 时才复位 loading/inFlight，避免清掉新一轮请求的状态
      if (gen === generationRef.current) {
        setLoading(false);
        inFlightRef.current = false;
      }
    }
  }, [enabled, fetcher, offset, pageSize]);

  const reset = useCallback(() => {
    generationRef.current += 1;
    inFlightRef.current = false;
    setLoading(false);
    setItems([]);
    setTotal(0);
    setOffset(0);
    setLoaded(false);
    // 强制 effect 重跑：补偿“reset 时旧请求仍 in-flight 而被作废”的首屏拉取
    setGeneration((g) => g + 1);
  }, []);

  // enabled 切换为 true 或刚 reset 后自动拉首页
  /* eslint-disable react-hooks/set-state-in-effect -- intentional: trigger first-page fetch on enable; loadMore awaits then setState */
  useEffect(() => {
    if (enabled && !loaded) {
      void loadMore();
    }
  }, [enabled, loaded, loadMore, generation]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const hasMore = !loaded ? enabled : offset < total;

  return { items, total, loading, hasMore, loadMore, reset };
}
