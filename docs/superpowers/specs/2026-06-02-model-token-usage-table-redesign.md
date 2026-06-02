# Model Token Usage 排名表重设计

## 概述

将 Dashboard 页面的 "Model Token Usage" 堆叠柱状图替换为排名表格 + 行内比例条，使各模型 Token 用量及缓存命中率更直观。

## 改动范围

### 前端

**替换文件**：`web/src/components/charts/model-token-bar-chart.tsx`

将 Recharts 堆叠 `BarChart` 替换为纯表格组件，包含以下列：

| 列 | 说明 |
|---|---|
| # | 排名序号，按 Total 降序 |
| Model | 模型名称 |
| Total | 该模型四种 Token 的总和 |
| Input | 行内堆叠比例条：Cache Read（紫色） + New Input（桔色），底部标注各段数值 |
| Output | 行内堆叠比例条：Cache Created（绿色） + Output（蓝色），底部标注各段数值 |

**不修改的文件**：
- `token-volume-chart.tsx`（Token Volume 堆叠面积图保持不变）
- `token-rate-chart.tsx`（Output Token Rate 折线图保持不变）
- `types.ts`、`api-client.ts`（数据层不变，`TokenThroughputItem` 结构已满足需求）

### 后端

无改动。

## 视觉规格

### 比例条

- 高度 12px，圆角 6px
- Input 条：左侧 `#7C6BA5`（Cache Read），右侧 `#D97757`（New Input）
- Output 条：左侧 `#4A9E7D`（Cache Created），右侧 `#5B8DB8`（Output）
- 底部两行小字标注各段名称和数值（如 "Cache Read 3.5M"）
- 使用 `formatTokenCount` 格式化数值（K/M）

### 表格

- 复用现有 `Card` / `CardHeader` / `CardContent` 包裹
- 列头可点击排序（默认按 Total 降序）
- 当前排序列显示三角形指示符（▲/▼）
- 悬停行显示背景高亮
- 等宽数字字体（`tabular-nums`）

### 状态

- 加载中：`Skeleton` 占位
- 无数据：展示空状态文案
- 加载失败：展示重试按钮

### TimeRangePicker

- 保留在 CardHeader 右上角，与现有布局一致

## 交互

1. 点击 Total / Input / Output 列头切换排序方向和排序列
2. 悬停比例条片段时 tooltip 显示该段精确数值
3. TimeRangePicker 切换时间范围后数据重新加载并重新排序

## 数据计算

基于现有 `TokenThroughputItem[]` 数据：

```
modelData = data.map(item => ({
  model: item.model,
  total: sum of all points across 4 metrics,
  inputTokens: sum of points.inputTokens,
  outputTokens: sum of points.outputTokens,
  cacheReadTokens: sum of points.cacheReadTokens,
  cacheCreationTokens: sum of points.cacheCreationTokens,
}))
```

排序：默认按 `total` 降序，支持点击列头切换排序列和方向。

## 不变更项

- Token Volume 和 Output Token Rate 两个图表卡片完全不动
- API 端点 `GET /api/v1/audit/stats/token/throughput` 不修改
- `TokenThroughputItem` / `TokenThroughputPoint` 类型不修改
