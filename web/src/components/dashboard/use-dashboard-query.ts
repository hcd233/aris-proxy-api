"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export interface DashboardQuery<T> {
  data: T | undefined;
  loading: boolean;
  error: boolean;
  retry: () => void;
}

/**
 * 面板级数据拉取：竞态保护（只认最后一次请求）+ 独立 loading/error，
 * 单个面板失败不影响其他面板。key 变化即重拉；fetcher 用 ref 引用最新闭包。
 */
export function useDashboardQuery<T>(key: string, fetcher: () => Promise<T>): DashboardQuery<T> {
  const fetcherRef = useRef(fetcher);
  useEffect(() => {
    fetcherRef.current = fetcher;
  }, [fetcher]);

  const seqRef = useRef(0);
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<{ data?: T; loading: boolean; error: boolean }>({
    loading: true,
    error: false,
  });

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects */
  useEffect(() => {
    const seq = ++seqRef.current;
    setState((s) => ({ data: s.data, loading: true, error: false }));
    fetcherRef
      .current()
      .then((data) => {
        if (seq === seqRef.current) setState({ data, loading: false, error: false });
      })
      .catch(() => {
        if (seq === seqRef.current)
          setState((s) => ({ data: s.data, loading: false, error: true }));
      });
  }, [key, attempt]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  return { data: state.data, loading: state.loading, error: state.error, retry };
}
