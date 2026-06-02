# Token 吞吐量仪表盘设计

## 概述

在 Dashboard 页面新增 Token 吞吐量仪表盘，展示模型的 token 使用量趋势和生成速率性能。一个 API 端点返回全部聚合数据，前端渲染两张图表。

## 需求

- **Token 总量图**：按时间桶展示每个模型的 input/output/cacheRead/cacheCreation token SUM，堆叠面积图
- **Token 速率图**：按时间桶展示每个模型的 output tokens/s，折线图
- **按模型分组**：和现有 ModelTrendChart/RequestRateChart 一致，每个模型一条线/区域，legend 切换
- **共享时间范围**：与现有图表共用 TimeRangePicker

## 方案

**一个 API，返回全部数据**（方案 A）

前端只请求一次，两个图表共享同一份数据。后端用 SQL 算好速率，前端直接使用。与现有 stats 端点模式一致。

## 后端设计

### API 端点

`GET /api/v1/audit/stats/token/throughput`

**认证**：`jwtAuth` + `PermissionUser`

**请求参数**（与现有 stats 端点一致）：

| 参数 | 位置 | 类型 | 必填 | 说明 |
|------|------|------|------|------|
| `startTime` | query | string (RFC3339) | 是 | 查询起始时间 |
| `endTime` | query | string (RFC3339) | 是 | 查询结束时间 |
| `granularity` | query | string | 是 | `minute`/`hour`/`day`/`week` |
| `apiKeyIDs` | query | string | 否 | 逗号分隔的 API Key ID |

**SQL 核心查询**：

```sql
SELECT model,
       date_trunc(?) AS time,
       SUM(input_tokens)                AS input_tokens,
       SUM(output_tokens)               AS output_tokens,
       SUM(cache_creation_input_tokens) AS cache_creation_tokens,
       SUM(cache_read_input_tokens)     AS cache_read_tokens,
       SUM(output_tokens) * 1000.0 / NULLIF(SUM(stream_duration_ms), 0) AS output_tokens_per_second
FROM model_call_audits
WHERE created_at BETWEEN ? AND ?
  AND (? = '' OR api_key_id = ANY(string_to_array(?, ','))
GROUP BY model, time
ORDER BY time
```

`outputTokensPerSecond`：`SUM(output_tokens) * 1000.0 / NULLIF(SUM(stream_duration_ms), 0)`，当 `SUM(stream_duration_ms) = 0`（无非流式请求）时返回 null。

**响应结构**：

```json
{
  "data": [
    {
      "model": "gpt-4o",
      "points": [
        {
          "time": "2026-06-02T10:00:00Z",
          "inputTokens": 15000,
          "outputTokens": 8000,
          "cacheCreationTokens": 0,
          "cacheReadTokens": 2000,
          "outputTokensPerSecond": 4.09
        }
      ]
    }
  ]
}
```

### 分层改动

| 层 | 文件 | 改动 |
|---|---|---|
| Domain VO | `internal/domain/modelcall/vo/` | 新增 `TokenThroughputPoint` 结构体 |
| Repository 接口 | `internal/domain/modelcall/repository/audit_repository.go` | 新增 `QueryTokenThroughput` 方法签名 |
| Repository 实现 | `internal/infrastructure/repository/audit_repository.go` | 实现 `QueryTokenThroughput`，SQL 聚合查询 |
| Application | `internal/application/audit/audit_service.go` | 新增 `TokenThroughput` 方法 |
| DTO | `internal/dto/audit_stats.go` | 新增 `TokenThroughputReq/Rsp`、`TokenThroughputItem`、`TokenThroughputPoint` |
| Handler | `internal/handler/audit.go` | 新增 `HandleTokenThroughput` (admin + user) |
| Router | `internal/router/audit.go` | 注册 `GET /stats/token/throughput` |

## 前端设计

### 图表组件

**TokenVolumeChart**（`web/src/components/charts/token-volume-chart.tsx`）：
- 类型：recharts `AreaChart`，堆叠面积
- 数据：`inputTokens` + `outputTokens` + `cacheReadTokens` + `cacheCreationTokens` 四层堆叠
- Y 轴：token 数量，自动格式化（K/M）
- 图例：legend 点击切换模型显示/隐藏，hover 高亮（复用 `use-chart-legend-highlight` hook）
- 色板：与现有图表一致 `CHART_COLORS`

**TokenRateChart**（`web/src/components/charts/token-rate-chart.tsx`）：
- 类型：recharts `LineChart`，与现有图表同风格
- 数据：`outputTokensPerSecond`，每模型一条线
- Y 轴：tokens/s
- 图例：同上

### 布局

Dashboard 页面（`web/src/app/(dashboard)/page.tsx`）现有 2 图下方新增一行 `grid-cols-1 md:grid-cols-2`，放置 TokenVolumeChart（左）和 TokenRateChart（右），共享同一个 `TimeRangePicker`。

### 前端改动

| 层 | 文件 | 改动 |
|---|---|---|
| Types | `web/src/lib/types.ts` | 新增 `TokenThroughputPoint`、`TokenThroughputItem`、`TokenThroughputRsp` |
| API Client | `web/src/lib/api-client.ts` | 新增 `fetchTokenThroughput()` |
| 图表组件 | `web/src/components/charts/token-volume-chart.tsx` | 新增 |
| 图表组件 | `web/src/components/charts/token-rate-chart.tsx` | 新增 |
| Dashboard | `web/src/app/(dashboard)/page.tsx` | 新增一行两图 |

## 不涉及

- 数据库迁移（`ModelCallAudit` 已有全部所需字段）
- DI container 改动（handler 已注册）
- 新依赖（recharts、shadcn chart 已有）
