"use client";

import { useCallback, useMemo, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useAutoRefresh } from "@/hooks/use-auto-refresh";
import {
  DASHBOARD_REFRESH_INTERVALS,
  REFRESH_OFF,
  normalizeRefreshInterval,
} from "@/lib/refresh-interval";
import { computeRange, type TimeRangeKey } from "@/lib/time-range";
import type { Granularity } from "@/lib/types";

export interface DashboardRange {
  key: TimeRangeKey;
  customStart: string;
  customEnd: string;
  startTime: string;
  endTime: string;
  granularity: Granularity;
  /** 对比周期的长度（天，可为小数），用于页头「对比上一 N 天」文案 */
  spanDays: number;
  /** 用户选定的视图（范围 + 粒度）；刷新不变，换范围才变。图表以它区分刷新与换视图 */
  scope: string;
  /** 每次刷新自增；面板以它作为拉取依赖 */
  refreshToken: number;
  /** 自动刷新间隔，REFRESH_OFF 表示关闭（默认） */
  refreshIntervalMs: number;
  /** 自动刷新轮次，驱动倒计时环 */
  refreshCycle: number;
  setRefreshIntervalMs: (ms: number) => void;
  setRange: (key: TimeRangeKey, customStart: string, customEnd: string) => void;
  /** 立即刷新，并重置自动刷新倒计时 */
  refresh: () => void;
}

/**
 * 全页唯一的时间范围：持久化选择，并把「现在」固化在 refreshToken 变化时，
 * 避免每次渲染重算 endTime 导致面板无限重拉。
 */
export function useDashboardRange(): DashboardRange {
  const [key, setKey] = usePersistentState<TimeRangeKey>("dashboard.range", "7d");
  const [customStart, setCustomStart] = usePersistentState("dashboard.range.customStart", "");
  const [customEnd, setCustomEnd] = usePersistentState("dashboard.range.customEnd", "");
  const [refreshToken, setRefreshToken] = useState(0);
  const [storedIntervalMs, setRefreshIntervalMs] = usePersistentState<number>(
    "dashboard.autoRefreshMs",
    REFRESH_OFF,
  );
  const refreshIntervalMs = normalizeRefreshInterval(
    storedIntervalMs,
    DASHBOARD_REFRESH_INTERVALS,
    REFRESH_OFF,
  );

  const { startTime, endTime, granularity } = useMemo(
    () => computeRange(key, customStart, customEnd),
    // refreshToken 参与依赖：刷新时以新的「现在」重算相对区间
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [key, customStart, customEnd, refreshToken],
  );
  const spanDays = (new Date(endTime).getTime() - new Date(startTime).getTime()) / 86_400_000;

  const setRange = useCallback(
    (k: TimeRangeKey, cs: string, ce: string) => {
      setKey(k);
      setCustomStart(cs);
      setCustomEnd(ce);
    },
    [setKey, setCustomStart, setCustomEnd],
  );
  const bump = useCallback(() => setRefreshToken((n) => n + 1), []);
  const { cycle, refreshNow } = useAutoRefresh(refreshIntervalMs, bump);

  return {
    key,
    customStart,
    customEnd,
    startTime,
    endTime,
    granularity,
    spanDays,
    scope: `${key}|${customStart}|${customEnd}|${granularity}`,
    refreshToken,
    refreshIntervalMs,
    refreshCycle: cycle,
    setRefreshIntervalMs,
    setRange,
    refresh: refreshNow,
  };
}
