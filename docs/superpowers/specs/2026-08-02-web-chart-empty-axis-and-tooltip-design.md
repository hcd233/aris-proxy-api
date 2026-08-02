# 前端重构：无数据图表显示坐标 + title 统一替换为 Tooltip

> 日期：2026-08-02
> 分支：`refactor/web-chart-tooltip-2026-08-02`
> 范围：`web/` 前端（Next.js 静态导出）

## 背景与目标

用户反馈两个前端体验问题：

1. **后端无数据时，图表直接显示 "No data for this period" 文字**，坐标系（X 轴时间刻度、Y 轴、网格）完全消失，用户无法感知所选时间范围对应的坐标空间。
2. **部分悬停提示仍用原生 `title=` 属性**（浏览器默认 tooltip，样式不可控、有延迟且跨语言不统一），希望统一为项目内 `@/components/ui/tooltip` 组件，样式与 Endpoints 页 Base URL 列一致。

已确认的决策：
- **A1**：需求 1 仅覆盖 dashboard 历史图表（`web/src/components/charts/` 下 6 个组件），**不包含** monitor 页实时轮询的 `RuntimeChart`（保留 "Collecting…" 采集提示）。
- **A2**：需求 2 全部替换，包括图标按钮类（删除、主题切换、侧边栏等），保留 `aria-label` 无障碍属性。

## 需求 1：无数据时渲染空坐标轴

### 涉及组件（6 个 dashboard 图表）

| 组件文件 | 图表类型 | 当前空态行为 |
|---|---|---|
| `model-trend-chart.tsx` | LineChart | 显示 "No data for this period" |
| `request-rate-chart.tsx` | LineChart | 同上 |
| `token-rate-chart.tsx` | LineChart | 同上 |
| `first-token-latency-chart.tsx` | LineChart | 同上 |
| `token-volume-chart.tsx` | AreaChart | 显示 `t("charts.no_data")`（同义文案） |
| `model-token-bar-chart.tsx` | **表格 + 条形图**（非坐标系） | 显示 `t("charts.no_data")` |

### 设计

1. **保存时间范围 state**：`fetchData` 中 `computeRange()` 的返回值（`startTime` / `endTime` / `granularity`）除了传给 API，另存为组件 state（如 `rangeRef` / `rangeState`），供空态生成时间轴使用。
2. **空时间轴生成**：当 `flatData`（或 `data`）为空时，生成从 `startTime` 到 `endTime`、按 `granularity` 等间隔的时间点数组：
   - `minute` → 每 1 分钟一个点
   - `hour` → 每 1 小时一个点
   - `day` → 每 1 天一个点
   - `week` → 每 1 周一个点
   - 所有序列值填 `null`（recharts 不画线/面积，仅渲染坐标轴、网格、图例）。
   - 用项目已有依赖 `date-fns` 实现（`addMinutes/addHours/addDays/addWeeks`），不新增依赖。
   - 点数上限安全：granularity 由 range 推导（1h→minute 约 60 点、24h→hour 24 点、7d→hour 168 点、30d→day 30 点），不会产生性能问题。
3. **渲染逻辑**：删除 `flatData.length === 0` 时显示文字的分支，恒渲染 `ChartContainer + LineChart/AreaChart`；`models`（序列）为空时 `chartConfig` 为空、无 `Line/Area` 子元素，recharts 仍渲染 X/Y 轴与网格。
4. **Y 轴刻度修复（实测结论）**：recharts 3.x 在**所有序列数据缺失**时 Y 轴无法生成刻度（`ticks`/`tickCount` 均无效）。实测方案：空时间轴数据点附加 `__empty: 0` 字段 + 渲染一条透明序列（`<Line dataKey="__empty" stroke="transparent" />` / `<Area ... fill="transparent" />`），使 Y 轴获得真实数据域，配合 `domain={[0, 1]}` + `tickCount={3}` 渲染 0/0.5/1 刻度。非空数据时 `__empty` 分支不渲染，行为与原来一致。
5. **`model-token-bar-chart.tsx`（表格型）**：无坐标系概念，空态改为渲染完整表头（Rank / Model / Total / Input / Output）+ 空表体，保持列结构可见；不再显示 "No data" 文字块。
6. **图例空态**：`models` 为空时图例自然不渲染（没有序列可展示），不做特殊处理。

### 数据流

```
fetchData() → computeRange() → { startTime, endTime, granularity }
   ├─→ api.fetchXxx() → setData()
   └─→ setRangeState({ startTime, endTime, granularity })   // 新增
渲染：data/flatData 为空 ? 生成空时间轴 : 正常 flatData
```

## 需求 2：title → Tooltip 全量替换

### 统一模式（与 Endpoints Base URL 列一致）

```tsx
<TooltipProvider>
  <TooltipRoot>
    <TooltipTrigger render={<原元素 />} />
    <TooltipContent side="top" align="start" className="max-w-…">
      {完整内容}
    </TooltipContent>
  </TooltipRoot>
</TooltipProvider>
```

- `TooltipTrigger` 用 `render` prop 包裹原元素（不改变原有 DOM/样式），原元素保留 `aria-label`（若有）。
- 截断文本单元格（`truncate` + `title=`）：移除 `title=`，Tooltip 内容为完整文本。
- 图标按钮：移除 `title=`，保留 `aria-label`，Tooltip 内容为原 title 文案。
- 文本型（inline/渲染型）trigger 需要包裹为 `<span>` / `<button type="button">` 等可聚焦元素（参考 Endpoints 的 `<button type="button" className="w-full cursor-default text-left">` 包裹）。

### 替换清单（16 处）

| 文件 | 位置 | 类型 | Tooltip 内容 |
|---|---|---|---|
| `(dashboard)/dataset/page.tsx` | `DistributionList` 标签 `<span title={item.label}>` | 截断文本 | `item.label` |
| `(dashboard)/models/page.tsx` | 移动端 upstreamModel `<p title={model.upstreamModel}>` | 截断文本 | `model.upstreamModel` |
| `(dashboard)/models/page.tsx` | alias `<span title={model.alias}>` | 截断文本 | `model.alias` |
| `(dashboard)/models/page.tsx` | modelId `<span title={model.modelId}>` | 截断文本 | `model.modelId` |
| `(dashboard)/models/page.tsx` | upstreamModel `<span title={model.upstreamModel}>` | 截断文本 | `model.upstreamModel` |
| `(dashboard)/models/page.tsx` | context length 徽章 | 数值徽章 | `{t("models.context_length")}: N` |
| `(dashboard)/models/page.tsx` | max output 徽章 | 数值徽章 | `{t("models.max_output")}: N` |
| `(dashboard)/models/page.tsx` | endpoint 名称按钮 `<button title={getEndpointName(model)}>` | 截断文本 | `getEndpointName(model)` |
| `(dashboard)/trace/page.tsx` | 删除按钮 ×2 | 图标按钮 | `t("trace.delete_aria")` |
| `components/trace-detail/trace-detail-client.tsx` | 返回按钮 | 图标按钮 | `t("trace.back_to_traces")` |
| `components/trace-detail/trace-detail-client.tsx` | 删除按钮 | 图标按钮 | `t("trace.delete_aria")` |
| `components/trace-detail/trace-detail-client.tsx` | cwd `<p title={detail.cwd}>` | 截断文本 | `detail.cwd` |
| `components/session-detail/session-detail-client.tsx` | 分享按钮 / 删除按钮 / 工具按钮 ×3 | 图标按钮 | 对应 `session_detail.*_title` |
| `(dashboard)/share/page.tsx` | 工具按钮 | 图标按钮 | `t("share.available_tools")` |
| `(dashboard)/audit/model/page.tsx` | 复制 traceId ×2 | 可点击文本 | `t("audit.copy_traceid_title")` |
| `(dashboard)/audit/cron/page.tsx` | 复制 traceId | 可点击文本 | `t("cron_audit.copy_traceid_title")` |
| `(dashboard)/layout.tsx` | 侧边栏折叠提示（collapsed 时） | 导航项 | `label` |
| `(dashboard)/layout.tsx` | logout 按钮 | 图标按钮 | `t("nav.logout")` |
| `components/theme/theme-switcher.tsx` | 主题切换按钮（inline + FAB）×2 | 图标按钮 | `label` |

> 说明：`sessions/page.tsx`、`blocked/page.tsx`、`shares/page.tsx`、`cron/page.tsx` 等中的 `title=` 均为 `Dialog`/`StepCard`/`Sheet` 等组件的 `title` prop（标题属性，非 hover 提示），**不替换**。`reading-layout.tsx` 的 `title` 是 Sheet 标题 prop，同样不替换。

### Provider 组织

- 每处沿用现有"局部 `TooltipProvider`"模式（与 Endpoints 一致），避免改动布局层级。
- 侧边栏（`layout.tsx` 的 `NavItems`）多个导航项都需要 tooltip 时，在该组件根部包一个 `TooltipProvider`。

## 验证方式

1. `cd web && npm run lint && npm run build` 通过（类型与导出验证）。
2. 本地联调（`go run ./cmd/server server start` + `npm run dev`）：
   - 需求 1：选一个无数据的时间范围（或清空后端数据），确认 6 个 dashboard 图表渲染空坐标轴（X 轴时间刻度、Y 轴、网格可见），无 "No data" 文字。
   - 需求 2：悬停各替换点，确认展示统一 Tooltip 样式，且无原生 `title=` 残留（检查页面 HTML 与代码 grep）。
3. grep 验证：`web/src` 下不再有数据类 `title={` hover 提示残留。

## 边界与不做的事

- 不改 monitor 页 `RuntimeChart`（保留 "Collecting…"）。
- 不改 `title` 作为组件 prop 的用法（Dialog/StepCard 等）。
- 不新增依赖（空时间轴用 `date-fns`）。
- 不引入 `title=` 的 a11y 语义替代（保留 `aria-label`）。
