# Dashboard 统计图表设计

## 概述

在 Dashboard 页面新增两个基于审计日志的图表卡片，展示模型调用趋势和请求成功率，支持按时间粒度切换。

## 需求

1. 砍掉原 Dashboard 的 "Recent Sessions" 和 "Quick Actions" 卡片
2. 新增 "Model Call Trend" 卡片：展示各模型在指定时间范围内的调用次数变化曲线
3. 新增 "Request Success Rate" 卡片：展示各模型在指定时间范围内的请求成功率曲线
4. 支持按 `minute` / `hour` / `day` / `week` 粒度切换聚合
5. 普通 user 跟 admin 布局相同，数据范围不同（user 只看自己 key 的审计）

## 后端

### 路由

```
GET /api/v1/audit/stats/model/trend
GET /api/v1/audit/stats/request/rate
```

- 鉴权：JWT（`jwtAuth`），`LimitUserPermissionMiddleware`
- admin 查全量，user 只查自己 key 的记录（复用现有 `apiKeyIDLookup` 逻辑）

### 请求参数

| 参数 | 类型 | 位置 | 必填 | 说明 |
|------|------|------|------|------|
| `startTime` | `time.Time` | query | 是 | ISO 8601 起始 |
| `endTime` | `time.Time` | query | 是 | ISO 8601 结束 |
| `granularity` | `string` | query | 是 | 枚举：`minute` / `hour` / `day` / `week` |

### 响应结构

**model/trend：**
```json
{
  "error": null,
  "data": [
    {
      "model": "gpt-4",
      "points": [
        {"time": "2026-05-25T00:00:00Z", "count": 42}
      ]
    }
  ]
}
```

**request/rate：**
```json
{
  "error": null,
  "data": [
    {
      "model": "gpt-4",
      "points": [
        {"time": "2026-05-25T00:00:00Z", "total": 100, "success": 95, "failed": 5, "successRate": 0.95}
      ]
    }
  ]
}
```

成功定义：`upstream_status_code = 200`。

### 新增文件

| 层 | 文件 | 说明 |
|----|------|------|
| DTO | `internal/dto/audit_stats.go` | `ModelTrendReq` / `ModelTrendRsp` / `RequestRateReq` / `RequestRateRsp` 及内部结构体 |
| Repository interface | `internal/domain/modelcall/repository.go` | 新增 `QueryModelTrend` / `QueryRequestRate` 方法 |
| Repository impl | `internal/infrastructure/repository/audit_repository.go` | GORM 原生 SQL 实现，`date_trunc` + `GROUP BY` |
| Usecase query | `internal/application/audit/query/` | `model_trend.go` + `request_rate.go`，各含 handler 和 query struct |
| Handler | `internal/handler/audit.go` | 新增两个 handler 方法 |
| Router | `internal/router/audit.go` | 注册两条新路由 |
| Container | `internal/bootstrap/container.go` | 注册新的 usecase handler |

### SQL 示意（PostgreSQL）

```sql
-- model trend
SELECT model,
       date_trunc('hour', created_at) AS time_bucket,
       COUNT(*) AS count
FROM model_call_audits
WHERE created_at BETWEEN ? AND ?
  AND api_key_id IN (?)
GROUP BY model, time_bucket
ORDER BY model, time_bucket

-- request rate
SELECT model,
       date_trunc('hour', created_at) AS time_bucket,
       COUNT(*) AS total,
       COUNT(*) FILTER (WHERE upstream_status_code = 200) AS success
FROM model_call_audits
WHERE created_at BETWEEN ? AND ?
  AND api_key_id IN (?)
GROUP BY model, time_bucket
ORDER BY model, time_bucket
```

`granularity` 参数映射到 `date_trunc` 的第一个参数。

## 前端

### 新增依赖

```bash
npx shadcn add chart
```

shadcn/chart 会安装 recharts 作为底层渲染引擎。

### 新增/修改文件

| 文件 | 说明 |
|------|------|
| `src/components/charts/model-trend-chart.tsx` | Model Trend 卡片（自包含：fetch + 图表 + 粒度 toggle） |
| `src/components/charts/request-rate-chart.tsx` | Request Rate 卡片 |
| `src/lib/api-client.ts` | 新增 `fetchModelTrend` / `fetchRequestRate` 方法 |
| `src/lib/types.ts` | 新增 `ModelTrendRsp` / `RequestRateRsp` 等类型 |
| `src/app/(dashboard)/page.tsx` | 引入两个卡片，放在 StatCards 下方 |

### 卡片布局

```
┌─────────────────────────────┬─────────────────────────────┐
│  Model Call Trend           │  Request Success Rate       │
│  [Hour] [Day] [Week]        │  [Hour] [Day] [Week]       │
│                             │                             │
│  📈 LineChart (recharts)    │  📈 LineChart (recharts)    │
│  每模型一条线               │  每模型一条线               │
│  X=时间 Y=调用次数          │  X=时间 Y=成功率(0-100%)   │
│                             │                             │
└─────────────────────────────┴─────────────────────────────┘
```

- 两卡片在 `lg:grid-cols-2` 网格中并排
- 粒度 Toggle 用 shadcn `ToggleGroup` 组件
- 默认：Model Trend → 近 7 天 / `day`；Request Rate → 近 24h / `hour`

### 图表风格

- shadcn/chart 默认主题，与 dashboard 的 dark/light 模式一致
- 每模型一条线，颜色从 chart 调色板自动分配
- Y 轴自动缩放
- 无数据时显示 "No data" 占位

### 加载与错误

- 加载中显示 Skeleton
- 失败时显示 "Failed to load" 带重试按钮
- 数据为空显示 "No data for this period"

## 权限

- admin → `repo.ListAll` → 全量审计统计
- user → `repo.ListByAPIKeyIDs` → 只统计自己 key 的记录
- 复用现有 `ListAuditLogsByUserHandler` 中的 `apiKeyIDLookup` 逻辑

## 测试

- 后端：`test/unit/audit/` 下为新的 usecase handler 和 repository 方法补单元测试
- 前端：编译检查 + 手动验证两种粒度和时间范围的渲染
