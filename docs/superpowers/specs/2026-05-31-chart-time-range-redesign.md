# Chart 时间范围选择器改造

## 背景

当前 Dashboard 图表（ModelTrendChart、RequestRateChart）使用固定时间窗口和 ToggleGroup 切换 granularity：
- ModelTrendChart：固定 7 天、默认 day granularity
- RequestRateChart：固定 24 小时、默认 hour granularity

问题：时间范围与数据展示不匹配（如选择 week granularity 仍只查 7 天），且缺乏灵活时间范围选择能力。

## 目标

1. 将固定时间窗口 + granularity 切换改造为灵活时间范围选择器
2. 新增共享 TimeRangePicker 组件，统一审计页和图表的时间选择体验
3. Granularity 根据时间范围自动推导，不再手动选择
4. 最小改动：API client、types、后端 DTO/Handler 均不变

## 方案

### 1. 新增 `web/src/lib/time-range.ts`（共享逻辑层）

```ts
export type TimeRangeKey = "1h" | "24h" | "7d" | "30d" | "custom";

export const TIME_RANGE_LABELS: Record<TimeRangeKey, string> = {
  "1h": "Last 1 hour",
  "24h": "Last 24 hours",
  "7d": "Last 7 days",
  "30d": "Last 30 days",
  custom: "Custom",
};

export const TIME_RANGE_PRESETS: TimeRangeKey[] = ["1h", "24h", "7d", "30d"];
```

Granularity 推导规则：

| 时间范围 | Granularity |
|----------|-------------|
| ≤ 1 小时 | minute |
| ≤ 24 小时 | hour |
| ≤ 7 天 | hour |
| ≤ 30 天 | day |
| > 30 天 | week |

`computeRange()` 函数：接受 TimeRangeKey + 可选 custom 起止时间，返回 `{ startTime, endTime, granularity }`。

### 2. 新增 `web/src/components/ui/time-range-picker.tsx`（共享 UI 组件）

Props:
- `value: TimeRangeKey`
- `customStart: string`
- `customEnd: string`
- `onChange: (key: TimeRangeKey, customStart: string, customEnd: string) => void`

UI 结构：
- DropdownMenuTrigger（显示当前预设标签 + Clock 图标）
- DropdownMenuContent：预设选项 + "Custom"
- timeRange === "custom" 时显示两个 `<input type="datetime-local">`

### 3. 改造 `audit/page.tsx`

用 `<TimeRangePicker>` 替换内联实现，行为完全保持一致。

### 4. 改造 `model-trend-chart.tsx` 和 `request-rate-chart.tsx`

- 移除 ToggleGroup 和 granularity useState
- 新增 `timeRange`、`customStart`、`customEnd` state
- 用 `<TimeRangePicker>` 替换 ToggleGroup 区域
- `fetchData` 用 `computeRange()` 计算 startTime/endTime/granularity 后调 API

## 不变部分

- `api-client.ts`（接口签名不变）
- `types.ts`（类型定义不变）
- 后端 DTO、Handler、Router（全部不变）
- 图表渲染、图例、tooltip（全部不变）
- `use-chart-legend-highlight` hook（不变）

## 文件变更清单

| 文件 | 操作 |
|------|------|
| `web/src/lib/time-range.ts` | 新建 |
| `web/src/components/ui/time-range-picker.tsx` | 新建 |
| `web/src/components/charts/model-trend-chart.tsx` | 修改 |
| `web/src/components/charts/request-rate-chart.tsx` | 修改 |
| `web/src/app/(dashboard)/audit/page.tsx` | 修改（复用 TimeRangePicker） |

## 验证

1. 两个图表：选择不同预设 → 图表按正确时间范围 + granularity 渲染
2. Audit 页：行为与改造前完全一致
3. Custom 模式：选择自定义起止时间 → 图表按范围渲染
4. `npm run lint && npm run build` 通过
