"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useT } from "@/lib/i18n";
import type { Granularity } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  ChartLegend,
  ChartLegendContent,
} from "@/components/ui/chart";
import { Line, LineChart, Area, AreaChart, XAxis, YAxis, CartesianGrid } from "recharts";
import { useChartLegendHighlight } from "@/hooks/use-chart-legend-highlight";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange, formatChartTime, generateEmptyTimeline } from "@/lib/time-range";
import { useChartSeriesColors } from "@/lib/theme";

/** 图表 series 定义（key 同时是 recharts dataKey） */
export interface ChartSeries {
  key: string;
  label: string;
  color: string;
}

/** tooltip formatter 的 item 参数（recharts payload 条目子集） */
export interface TooltipItem {
  color?: string;
  payload?: Record<string, unknown>;
}

export interface LineChartCardProps<T> {
  /** 卡片标题 i18n key */
  titleKey: string;
  /** usePersistentState 存储前缀（timeRange/customStart/customEnd 三键共用） */
  storageKey: string;
  /** 默认时间范围，默认 "7d" */
  defaultRange?: TimeRangeKey;
  /** 数据获取；参数与 api.* 图表方法一致 */
  fetchData: (params: {
    startTime: string;
    endTime: string;
    granularity: Granularity;
  }) => Promise<{ data?: T[] }>;
  /** 原始数据 → 扁平行数据 + series。colors 为当前主题图表色板（固定色板图表可忽略） */
  toChart: (
    data: T[],
    colors: readonly string[],
  ) => { rows: Array<Record<string, unknown>>; series: ChartSeries[] };
  /** 图表类型，默认 "line" */
  chartType?: "line" | "area";
  /** 堆叠 id（chartType="area" 时生效） */
  stackId?: string;
  /** Y 轴配置 */
  yAxis?: {
    /** 非空态 domain，默认 [0, "auto"] */
    domain?: [number | string, number | string];
    /** 空态 domain，默认 [0, 1] */
    emptyDomain?: [number | string, number | string];
    tickFormatter?: (value: number) => string;
  };
  /** tooltip 值 formatter；不传使用 ChartTooltipContent 默认渲染 */
  tooltipFormatter?: (
    value: unknown,
    name: string | number | undefined,
    item: TooltipItem,
  ) => ReactNode;
  /** 额外图表元素（如 ReferenceLine 平均线），可访问原始数据与 legend 状态 */
  extraElements?: (ctx: {
    data: T[];
    series: ChartSeries[];
    chartConfig: Record<string, { label: string; color: string }>;
    activeLegend: string | null;
    isEmpty: boolean;
  }) => ReactNode;
}

/** 构建 "指示色块 + name + 格式化值" 的 tooltip 行；format 为值格式化函数 */
export function formatTooltipRow(
  value: unknown,
  name: string | number | undefined,
  item: TooltipItem,
  format: (v: number) => string,
): ReactNode {
  if (value == null) return null;
  const indicatorColor = item?.color ?? "#888";
  return (
    <>
      <div
        className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
        style={{ backgroundColor: indicatorColor }}
      />
      <div className="flex flex-1 items-center justify-between leading-none">
        <span className="text-muted-foreground">{name}</span>
        <span className="font-mono font-medium text-foreground tabular-nums">
          {format(Number(value))}
        </span>
      </div>
    </>
  );
}

/**
 * Dashboard 时序图卡片通用骨架：时间范围状态 + 竞态保护数据获取 + TimeRangePicker +
 * loading/error 态 + 坐标轴 + tooltip/legend + 空数据坐标轴。
 * 数据转换与图表特有配置（Y 轴、tooltip、平均线等）由调用方以 props 注入。
 */
export function LineChartCard<T>({
  titleKey,
  storageKey,
  defaultRange = "7d",
  fetchData: fetchDataProp,
  toChart,
  chartType = "line",
  stackId,
  yAxis,
  tooltipFormatter,
  extraElements,
}: LineChartCardProps<T>) {
  const t = useT();
  const seriesColors = useChartSeriesColors();

  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>(
    `${storageKey}.timeRange`,
    defaultRange,
  );
  const [customStart, setCustomStart] = usePersistentState(`${storageKey}.customStart`, "");
  const [customEnd, setCustomEnd] = usePersistentState(`${storageKey}.customEnd`, "");
  const requestIdRef = useRef(0);
  const [data, setData] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [rangeState, setRangeState] = useState<{
    startTime: string;
    endTime: string;
    granularity: Granularity;
  } | null>(null);
  const { activeLegend, onLegendHover, getStrokeOpacity } = useChartLegendHighlight();

  // 始终引用最新 fetchData，避免调用方内联函数导致 effect 循环
  const fetchDataRef = useRef(fetchDataProp);
  useEffect(() => {
    fetchDataRef.current = fetchDataProp;
  }, [fetchDataProp]);

  const fetchData = useCallback(
    async (range?: TimeRangeKey, cs?: string, ce?: string) => {
      const requestId = ++requestIdRef.current;
      setLoading(true);
      setError(false);
      try {
        const { startTime, endTime, granularity } = computeRange(
          range ?? timeRange,
          cs ?? customStart,
          ce ?? customEnd,
        );
        setRangeState({ startTime, endTime, granularity });
        const rsp = await fetchDataRef.current({ startTime, endTime, granularity });
        if (requestId !== requestIdRef.current) return;
        setData(rsp.data ?? []);
      } catch {
        if (requestId !== requestIdRef.current) return;
        setError(true);
      } finally {
        if (requestId === requestIdRef.current) {
          setLoading(false);
        }
      }
    },
    [timeRange, customStart, customEnd],
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const { rows, series } = useMemo(
    () => toChart(data, seriesColors),
    [data, seriesColors, toChart],
  );

  const chartConfig = useMemo(
    () => Object.fromEntries(series.map((s) => [s.key, { label: s.label, color: s.color }])),
    [series],
  );

  // 后端无数据时仍渲染空坐标轴（X 轴时间刻度 + Y 轴网格），而非显示空态文案
  const isEmpty = rows.length === 0;
  const chartData =
    isEmpty && rangeState
      ? generateEmptyTimeline(rangeState.startTime, rangeState.endTime, rangeState.granularity).map(
          (time) => {
            const row: Record<string, unknown> = { time, __empty: 0 };
            for (const s of series) row[s.key] = 0;
            return row;
          },
        )
      : rows;

  const yDomain = isEmpty ? (yAxis?.emptyDomain ?? [0, 1]) : (yAxis?.domain ?? [0, "auto"]);
  const isArea = chartType === "area";

  const seriesElements = series.map((s) =>
    isArea ? (
      <Area
        key={s.key}
        type="monotone"
        dataKey={s.key}
        stackId={stackId}
        stroke={s.color}
        fill={s.color}
        strokeOpacity={getStrokeOpacity(s.key)}
        fillOpacity={0.6}
      />
    ) : (
      <Line
        key={s.key}
        type="monotone"
        dataKey={s.key}
        stroke={s.color}
        strokeWidth={2}
        strokeOpacity={getStrokeOpacity(s.key)}
        dot={false}
      />
    ),
  );

  const emptySeriesElement = isArea ? (
    <Area dataKey="__empty" stroke="transparent" fill="transparent" />
  ) : (
    <Line dataKey="__empty" stroke="transparent" dot={false} />
  );

  const chartChildren = [
    <CartesianGrid key="grid" strokeDasharray="3 3" vertical={false} />,
    <XAxis
      key="x-axis"
      dataKey="time"
      tickFormatter={(v) => formatChartTime(v, timeRange, customStart, customEnd)}
      fontSize={12}
    />,
    <YAxis
      key="y-axis"
      fontSize={12}
      domain={yDomain}
      tickCount={isEmpty ? 3 : undefined}
      tickFormatter={yAxis?.tickFormatter}
      allowDataOverflow={false}
    />,
    <ChartTooltip key="tooltip" content={<ChartTooltipContent formatter={tooltipFormatter} />} />,
    <ChartLegend
      key="legend"
      content={<ChartLegendContent activeLegend={activeLegend} onLegendHover={onLegendHover} />}
    />,
    ...seriesElements,
    isEmpty ? emptySeriesElement : null,
    extraElements?.({ data, series, chartConfig, activeLegend, isEmpty }),
  ];

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="font-display">{t(titleKey)}</CardTitle>
        <TimeRangePicker
          value={timeRange}
          customStart={customStart}
          customEnd={customEnd}
          onChange={(key, cs, ce) => {
            setTimeRange(key);
            setCustomStart(cs);
            setCustomEnd(ce);
            fetchData(key, cs, ce);
          }}
        />
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : error ? (
          <div className="flex h-64 flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
            <p>{t("charts.failed_to_load")}</p>
            <Button variant="outline" size="sm" onClick={() => fetchData()}>
              {t("charts.retry")}
            </Button>
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-64 w-full">
            {isArea ? (
              <AreaChart data={chartData}>{chartChildren}</AreaChart>
            ) : (
              <LineChart data={chartData}>{chartChildren}</LineChart>
            )}
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
