# Web 前端重构实现计划：无数据图表显示坐标 + title 统一替换为 Tooltip

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 后端无数据时 dashboard 图表渲染空坐标轴（不再显示 "No data for this period"）；全站 19 处 `title=` 悬停提示统一替换为 shadcn Tooltip 组件（样式与 Endpoints Base URL 列一致）。

**Architecture:** ① 在 `fetchData` 中新增保存 `computeRange` 结果的 state，图表数据为空时用 `date-fns` 按粒度生成空时间轴数据点（值 `null`）驱动 recharts 渲染空坐标。② 各处 `title=` 按统一模式替换为 `TooltipProvider/Root/Trigger/Content`，保留 `aria-label`。

**Tech Stack:** Next.js 16.2.6 / React 19 / recharts 3.8 / date-fns 4.4 / Tailwind v4 / shadcn `@/components/ui/tooltip`（基于 `@base-ui/react`）。

## Global Constraints

- 仅改 `web/` 目录；不改 `basePath`、不新增依赖（空时间轴用已有 `date-fns`）。
- 需求 1 不含 monitor 页 `RuntimeChart`（保留 "Collecting…"）。
- 不替换 `title` 作为组件 prop 的用法（`Dialog`/`StepCard`/`Sheet` 标题）。
- 替换后保留原元素 `aria-label`；`TooltipTrigger` 用 `render` prop 包裹原元素，不改动原 DOM 样式。
- 验证：`cd web && npm run lint && npm run build` 通过；grep 确认无数据类 `title={` 残留。
- 所有中文回复，提交信息遵循仓库规范。

---

### Task 1: `time-range.ts` 增加空时间轴生成函数

**Files:**
- Modify: `web/src/lib/time-range.ts`

**Interfaces:**
- Produces: `generateEmptyTimeline(startTime: string, endTime: string, granularity: Granularity): string[]` —— 返回等间隔 ISO 时间点数组（含首尾），供各图表空态复用。

- [ ] **Step 1: 实现函数**

```ts
import { addMinutes, addHours, addDays, addWeeks, isAfter } from "date-fns";

// 生成 startTime 到 endTime 之间按 granularity 等间隔的时间点（含起点，最多约 200 点，防自定义超长区间爆炸）
export function generateEmptyTimeline(
  startTime: string,
  endTime: string,
  granularity: Granularity,
): string[] {
  const start = new Date(startTime);
  const end = new Date(endTime);
  const step =
    granularity === "minute" ? addMinutes
    : granularity === "hour" ? addHours
    : granularity === "day" ? addDays
    : addWeeks;
  const points: string[] = [];
  const limit = 200;
  let cur = start;
  while (!isAfter(cur, end) && points.length < limit) {
    points.push(cur.toISOString());
    cur = step(cur, 1);
  }
  return points;
}
```

- [ ] **Step 2: 验证编译**

Run: `cd web && npx tsc --noEmit`
Expected: 无类型错误。

- [ ] **Step 3: Commit**

```bash
rtk git add web/src/lib/time-range.ts
rtk git commit -m "feat(web): time-range 增加空时间轴生成函数"
```

### Task 2: 6 个 dashboard 图表空态改为渲染空坐标轴

**Files:**
- Modify: `web/src/components/charts/model-trend-chart.tsx`
- Modify: `web/src/components/charts/request-rate-chart.tsx`
- Modify: `web/src/components/charts/token-rate-chart.tsx`
- Modify: `web/src/components/charts/first-token-latency-chart.tsx`
- Modify: `web/src/components/charts/token-volume-chart.tsx`
- Modify: `web/src/components/charts/model-token-bar-chart.tsx`

**Interfaces:**
- Consumes: `generateEmptyTimeline`（Task 1）。
- Produces: 每个图表组件在无数据时渲染坐标轴/表头；`ModelTokenBarChart` 空态渲染完整表头 + 空表体。

- [ ] **Step 1: 每个图表新增 range state**

在 `fetchData` 内 `computeRange` 之后追加：

```ts
const range = computeRange(range ?? timeRange, cs ?? customStart, ce ?? customEnd);
setRangeState({ startTime: range.startTime, endTime: range.endTime, granularity: range.granularity });
```

并在组件顶部声明：

```ts
const [rangeState, setRangeState] = useState<{ startTime: string; endTime: string; granularity: Granularity } | null>(null);
```

（`Granularity` 从 `@/lib/types` 导入，确认 `api-client` 已有导出。）

- [ ] **Step 2: 空态渲染坐标轴**

删除 `flatData.length === 0` 显示文字的 `<div>` 分支，改为统一渲染图表；当 `flatData` 为空时用 `generateEmptyTimeline(rangeState...)` 生成时间点，映射为 `{ time }` 对象数组（无序列字段，值缺失即 null）。`models.map` 的 `Line/Area` 保留（空数组时不渲染序列）。若 Y 轴 `domain={[0, "auto"]}` 在纯空数据下显示异常，改用 `domain={[0, 1]}`（实测后决定，5 个 Line/Area 图表统一处理）。

- [ ] **Step 3: `ModelTokenBarChart` 空态渲染表头**

删除 `sorted.length === 0` 的 no-data 分支，直接渲染现有表格结构；表体为空时 `<tbody>` 内无行（保留表头列结构）。

- [ ] **Step 4: 验证**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无错误。

- [ ] **Step 5: Commit**

```bash
rtk git add web/src/components/charts/
rtk git commit -m "feat(web): dashboard 图表无数据时渲染空坐标轴"
```

### Task 3: models 页 title → Tooltip（8 处）

**Files:**
- Modify: `web/src/app/(dashboard)/models/page.tsx`

- [ ] **Step 1: 导入 Tooltip**

```ts
import { TooltipProvider, TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
```

- [ ] **Step 2: 替换 8 处**

统一模式（截断文本，trigger 用 render 包裹原 span/button）：

```tsx
<TooltipProvider>
  <TooltipRoot>
    <TooltipTrigger render={<span className="...原样式..." aria-label={...可选...}>{展示文本}</span>} />
    <TooltipContent side="top" align="start" className="max-w-xs break-all">{完整文本}</TooltipContent>
  </TooltipRoot>
</TooltipProvider>
```

覆盖：移动端 upstreamModel（432）、alias（494）、modelId（500）、upstreamModel（505）、context length 徽章（513）、max output 徽章（520）、endpoint 按钮（542，trigger 包裹原 `<button>`，保留 onClick）。注意 `render` 包裹原元素时把 `title` 移除、`truncate/max-w` 等 class 留在 trigger 元素上。

- [ ] **Step 3: 验证**

Run: `cd web && npx tsc --noEmit && npm run lint`
Expected: 无错误。

- [ ] **Step 4: Commit**

```bash
rtk git add web/src/app/\(dashboard\)/models/page.tsx
rtk git commit -m "feat(web): models 页 title 提示统一替换为 Tooltip"
```

### Task 4: dataset 页 title → Tooltip（1 处）

**Files:**
- Modify: `web/src/app/(dashboard)/dataset/page.tsx`

- [ ] **Step 1: 替换 DistributionList 标签**

`<span className="max-w-[24ch] truncate ..." title={item.label}>` → Tooltip 包裹，内容 `item.label`。

- [ ] **Step 2: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint` → 通过后提交 `feat(web): dataset 页 title 提示统一替换为 Tooltip`。

### Task 5: trace 列表页 + trace-detail 页 title → Tooltip（5 处）

**Files:**
- Modify: `web/src/app/(dashboard)/trace/page.tsx`（删除按钮 ×2）
- Modify: `web/src/components/trace-detail/trace-detail-client.tsx`（返回按钮、删除按钮、cwd 截断）

- [ ] **Step 1: 替换图标按钮**

`<DeleteIconButton ... title={t("trace.delete_aria")} />` → 用 Tooltip 包裹，`TooltipTrigger render={<DeleteIconButton ... aria-label={...} />}`，移除 title。返回按钮同模式。cwd：`<p title={detail.cwd}>` → trigger 包 `<p>`，内容 `detail.cwd || "—"`。

- [ ] **Step 2: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint` → 提交 `feat(web): trace 相关页 title 提示统一替换为 Tooltip`。

### Task 6: session-detail + share 页 title → Tooltip（4 处）

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`（分享/删除/工具按钮 ×3）
- Modify: `web/src/app/(dashboard)/share/page.tsx`（工具按钮 ×1）

- [ ] **Step 1: 替换按钮**

模式同上：`TooltipTrigger render={<Button ... aria-label=... />}`，内容为原 title 文案（`session_detail.*_title` / `share.available_tools`）。

- [ ] **Step 2: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint` → 提交 `feat(web): session-detail 与 share 页 title 提示统一替换为 Tooltip`。

### Task 7: audit 页 title → Tooltip（3 处）

**Files:**
- Modify: `web/src/app/(dashboard)/audit/model/page.tsx`（复制 traceId ×2）
- Modify: `web/src/app/(dashboard)/audit/cron/page.tsx`（复制 traceId ×1）

- [ ] **Step 1: 替换 traceId 点击文本**

原 `<span onClick={...} title={t("audit.copy_traceid_title")}>{log.traceId.slice(-6)}</span>` → Tooltip 包裹（保留 onClick 与 hover:underline 样式），内容为对应 i18n 文案。

- [ ] **Step 2: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint` → 提交 `feat(web): audit 页 title 提示统一替换为 Tooltip`。

### Task 8: layout 侧边栏 + theme-switcher title → Tooltip（3 处）

**Files:**
- Modify: `web/src/app/(dashboard)/layout.tsx`（侧边栏 collapsed 提示、logout 按钮）
- Modify: `web/src/components/theme/theme-switcher.tsx`（inline + FAB 主题按钮 ×2）

- [ ] **Step 1: 侧边栏 collapsed 提示**

`NavItems` 组件根部包 `<TooltipProvider>`；每个 `Link` 的 `title={collapsed ? label : undefined}` 移除，改用条件渲染：`collapsed` 时 `TooltipTrigger render={<Link ... aria-label={label}>{icon}</Link>}` + Tooltip 内容 `label`；非 collapsed 时原样渲染 Link。

- [ ] **Step 2: logout 按钮 + theme-switcher**

`<Button title={label} aria-label={label}>` → Tooltip 包裹（内容 `label`），保留 `aria-label`。FAB 同模式。

- [ ] **Step 3: 验证 + Commit**

Run: `cd web && npx tsc --noEmit && npm run lint` → 提交 `feat(web): 侧边栏与主题切换 title 提示统一替换为 Tooltip`。

### Task 9: 全量验证

- [ ] **Step 1: lint + build**

Run: `cd web && npm run lint && npm run build`
Expected: 全部通过，`out/` 导出成功。

- [ ] **Step 2: grep 残留检查**

Run: `cd web && grep -rn "title={" src --include="*.tsx" | grep -v "aria-label" | grep -E "title=\{(t\(|model|item|log|detail|label|getEndpoint|metadata|collapsed)"` 或人工核对
Expected: 无数据类 hover 提示残留（Dialog/StepCard/Sheet 的 title prop 除外）。

- [ ] **Step 3: 本地联调抽查**

Run: 后端 `go run ./cmd/server server start --host localhost --port 8080` + `cd web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npm run dev`，浏览器访问 `http://localhost:3000/web`
Expected: ① dashboard 无数据时间范围下 6 图表渲染空坐标轴；② 悬停 models/dataset/trace 等替换点显示统一 Tooltip。

- [ ] **Step 4: Commit**

```bash
rtk git add -A
rtk git commit -m "chore(web): 前端重构验证完成"   # 仅在有残留修改时
```
