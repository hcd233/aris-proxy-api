"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { api } from "@/lib/api-client";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useAutoRefresh } from "@/hooks/use-auto-refresh";
import { MONITOR_REFRESH_INTERVALS, normalizeRefreshInterval } from "@/lib/refresh-interval";
import { replaceEqualDeep } from "@/lib/structural-share";
import {
  EMPTY_STATE,
  RANGE_WINDOW_SEC,
  mergeInstances,
  mergePoints,
  type RangeKey,
  type SeriesState,
} from "@/lib/monitor";

const DEFAULT_REFRESH_MS = 5_000;

/**
 * Monitor 页数据层：按 range 增量轮询 /metrics/runtime，把新点合并进本地时序窗口。
 * range 切换视为换视图——清空时序并立即重拉；刷新档位与 range 都持久化。
 */
export function useRuntimeMetrics() {
  const [range, setRange] = usePersistentState<RangeKey>("monitor.range", "1h");
  const [storedRefreshMs, setRefreshMs] = usePersistentState<number>(
    "monitor.autoRefreshMs",
    DEFAULT_REFRESH_MS,
  );
  const refreshMs = normalizeRefreshInterval(
    storedRefreshMs,
    MONITOR_REFRESH_INTERVALS,
    DEFAULT_REFRESH_MS,
  );
  const [state, setState] = useState<SeriesState>(EMPTY_STATE);
  const [loading, setLoading] = useState(true);
  const [fetching, setFetching] = useState(false);
  const [failed, setFailed] = useState(false);
  const sinceRef = useRef(0);

  const poll = useCallback(async (rangeKey: RangeKey) => {
    setFetching(true);
    try {
      const rsp = await api.getRuntimeMetrics({ range: rangeKey, since: sinceRef.current });
      const s = rsp.series ?? {};
      const now = Math.floor(Date.now() / 1000);
      const cutoff = now - RANGE_WINDOW_SEC[rangeKey];

      // 结构共享：没变的曲线保持原引用，对应图表这一轮完全不动
      setState((prev) =>
        replaceEqualDeep(prev, {
          qps: mergePoints(prev.qps, s.qps ?? [], cutoff),
          netRx: mergePoints(prev.netRx, s.netRx ?? [], cutoff),
          netTx: mergePoints(prev.netTx, s.netTx ?? [], cutoff),
          instances: mergeInstances(prev.instances, s.instances ?? {}, cutoff),
          latestTime: rsp.latestTime,
        }),
      );

      if (rsp.latestTime > 0) sinceRef.current = rsp.latestTime;
      setFailed(false);
    } catch {
      // 轮询失败保留旧曲线，只标记失败：首次加载失败时面板给重试入口
      setFailed(true);
    } finally {
      setLoading(false);
      setFetching(false);
    }
  }, []);

  const pollRange = useCallback(() => void poll(range), [poll, range]);
  const { cycle, refreshNow } = useAutoRefresh(refreshMs, pollRange);

  /* eslint-disable react-hooks/set-state-in-effect -- range 切换需重置时序状态并立即触发首次拉取 */
  useEffect(() => {
    sinceRef.current = 0;
    setState(EMPTY_STATE);
    setLoading(true);
    void poll(range);
  }, [range, poll]);
  /* eslint-enable react-hooks/set-state-in-effect */

  return {
    range,
    setRange,
    refreshMs,
    setRefreshMs,
    state,
    loading,
    fetching,
    failed,
    cycle,
    refreshNow,
  };
}
