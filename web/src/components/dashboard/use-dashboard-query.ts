"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { replaceEqualDeep } from "@/lib/structural-share";

export interface DashboardQuery<T> {
  data: T | undefined;
  /** 当前 data 所属的 scope（见 useDashboardQuery），尚无数据时为空串 */
  scope: string;
  loading: boolean;
  error: boolean;
  retry: () => void;
}

interface QueryState<T> {
  data?: T;
  scope: string;
  loading: boolean;
  error: boolean;
}

/**
 * 面板级数据拉取：竞态保护（只认最后一次请求）+ 独立 loading/error，
 * 单个面板失败不影响其他面板。key 变化即重拉；fetcher 用 ref 引用最新闭包。
 *
 * scope 区分「同一视图的刷新」与「换了一个视图」（如切换时间范围）：
 * 同 scope 的新结果与旧数据做结构共享，内容不变时 data 引用不变，下游图表
 * 不重放动画；返回的 scope 随数据一起落定，图表以它作 key，换视图时整图重新入场。
 */
export function useDashboardQuery<T>(
  key: string,
  fetcher: () => Promise<T>,
  scope = "",
): DashboardQuery<T> {
  const fetcherRef = useRef(fetcher);
  useEffect(() => {
    fetcherRef.current = fetcher;
  }, [fetcher]);

  const seqRef = useRef(0);
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<QueryState<T>>({ scope: "", loading: true, error: false });

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects */
  useEffect(() => {
    const seq = ++seqRef.current;
    setState((s) => ({ ...s, loading: true, error: false }));
    fetcherRef
      .current()
      .then((data) => {
        if (seq !== seqRef.current) return;
        setState((s) => ({
          data: s.scope === scope ? replaceEqualDeep(s.data, data) : data,
          scope,
          loading: false,
          error: false,
        }));
      })
      .catch(() => {
        if (seq === seqRef.current) setState((s) => ({ ...s, loading: false, error: true }));
      });
    // scope 只随 key 一起变化（调用方保证 key 包含 scope 的组成部分）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, attempt]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  return {
    data: state.data,
    scope: state.scope,
    loading: state.loading,
    error: state.error,
    retry,
  };
}
