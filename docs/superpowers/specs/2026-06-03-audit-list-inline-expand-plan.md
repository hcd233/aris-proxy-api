# Audit 列表内联展开优化 - 实施计划

日期：2026-06-03

## 改动范围

**单文件改动**：`web/src/app/(dashboard)/audit/page.tsx`

## 实施步骤

### Step 1: 新增状态与工具函数
- `isExpanded` state（`Set<number>`，key 为 log.id）
- `statusFilter` state（`"all" | "failed" | "success"`）
- `toggleExpand(id)` / `shouldAutoExpand(log)` / `shouldHighlight(log)` 工具函数
- `LATENCY_THRESHOLD_MS = 3000`

### Step 2: 重构筛选区
- 新增快速过滤芯片：All / Failed / Success（用 shadcn Badge/Toggle 实现）
- 过滤芯片与现有 TimeRangePicker + 搜索栏同行

### Step 3: 重构桌面端表格
- 精简表头：Status / Time / Model / User / API Key / Tokens（移除 Provider / Cache / Latency / TraceID 列）
- 每行拆为主行 + 详情行：
  - 主行：`<TableRow onClick={toggleExpand}>`，渲染上述 6 列，状态行带左边框颜色
  - 详情行：`<TableRow>` 内单格 `colSpan={6}`，渲染 Provider / Cache / Latency / TraceID / ErrorMessage / UA 的 grid 布局
  - 详情行显隐条件：`isExpanded.has(log.id) || shouldAutoExpand(log)`
- 异常高亮：错误行红色左边框 + ErrorMessage 红字；高延迟行 Latency 红字

### Step 4: 重构移动端卡片
- 卡片保持现有字段排列，新增展开/折叠交互
- 详情区显示 Provider / Cache / Latency / TraceID / UA
- 异常自动展开 + 高亮

### Step 5: 过滤逻辑
- `statusFilter` 非 "all" 时，调用 api 前先过滤？不，服务端过滤：需要新增 `status` 参数传给后端？
  - **决策**：暂不做服务端过滤，客户端过滤已加载数据。因为分页模式下客户端过滤体验差，改为：仅在当前页数据上做客户端过滤，并显示提示"showing filtered results on current page"
  - 更优方案：不做过滤，等后续需求再加服务端支持。当前仅加 UI 骨架

等等，用户说了要加 quick-filter chips。让我想想...如果客户端过滤，分页会乱。如果服务端过滤，需要改后端。最简单的初版方案是：仅在客户端对当前页日志做视觉过滤（高亮/淡化），不隐藏行。这样既能快速定位失败行，又不破坏分页。

实际上更好的方案是加 status query param 到后端，但我应该只改前端。让我用客户端过滤，并在过滤后的列表小于 pageSize 时显示提示。

不对，再次考虑：最简单的方案是客户端过滤当前页面数据。用户切换过滤时，在当前已加载的 logs 数组上 apply filter。如果过滤后为空，显示"no matching logs on current page"。这对排障场景来说够用了——用户通常从 All 开始，看到问题后切到 Failed 聚焦。

### Step 5 (revised): 客户端过滤
- 新增 `filteredLogs` useMemo：根据 `statusFilter` 过滤 `logs`
- 过滤芯片在筛选区渲染
- 当过滤后为空时显示不同文案

### Step 6: 验证
- `cd web && npm run lint`
- `cd web && npm run build`（typecheck）
- 如有问题修复

## 关键设计约束
- 后端 DTO 不动
- 分页/排序/搜索逻辑不动
- 仅改 `page.tsx` 单文件
- 遵循项目现有样式 token（warm terracotta 色系 + shadcn/ui）
