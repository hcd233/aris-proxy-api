# Model Token Usage 排名表重设计 + API 解耦

## 概述

1. Dashboard "Model Token Usage" 堆叠柱状图替换为排名表格 + 行内比例条
2. API 解耦：一个共享端点拆为三个专用端点，各返回前端就绪数据

## API 拆分

| 当前 | 改为 | 对应前端组件 |
|---|---|---|
| `GET /audit/stats/token/throughput`（全量） | 保持，数据不变 | Token Throughput 面积图 |
| （无） | 新增 `GET /audit/stats/token/rate` | Output Token Rate 折线图 |
| （无） | 新增 `GET /audit/stats/token/usage` | Model Usage 排名表 |

### 新端点设计

**`GET /api/v1/audit/stats/token/rate`**

Request（与现有 throughput 一致）：

```
startTime: time.Time (query, required)
endTime: time.Time (query, required)
granularity: enum(minute,hour,day,week) (query, required)
```

Response：

```json
{
  "data": [
    {
      "model": "gpt-4o",
      "points": [
        { "time": "2025-06-02T10:00:00Z", "outputTokensPerSecond": 42.5 }
      ]
    }
  ]
}
```

**`GET /api/v1/audit/stats/token/usage`**

Request：同上。

Response：

```json
{
  "data": [
    {
      "model": "gpt-4o",
      "inputTokens": 10000000,
      "outputTokens": 5000000,
      "cacheReadTokens": 3000000,
      "cacheCreationTokens": 1000000
    }
  ]
}
```

注：Total 由前端计算（`inputTokens + outputTokens + cacheReadTokens + cacheCreationTokens`），不在后端返回，避免字段冗余。

### 后端实现策略

- 复用现有 `QueryTokenThroughput` 仓库方法（一次 DB 查询覆盖三种需求）
- 新增两个 usecase handler：`TokenRateHandler` / `TokenUsageHandler`
- usecase 层对查询结果做聚合/转换，返回前端就绪的 DTO
- 遵循现有模式：`TokenRateByUserHandler` 同理新增

### 改动文件清单（后端）

| 文件 | 改动 |
|---|---|
| `internal/dto/audit_stats.go` | 新增 `TokenRateReq/Rsp/Item/Point`、`TokenUsageReq/Rsp/Item` |
| `internal/application/audit/query/token_rate.go` | **新文件**：`TokenRateHandler` / `TokenRateByUserHandler` |
| `internal/application/audit/query/token_usage.go` | **新文件**：`TokenUsageHandler` / `TokenUsageByUserHandler` |
| `internal/application/audit/query/service.go` | `AuditService` 接口新增 `TokenRate` / `TokenUsage` 方法；`auditService` 新增字段和构造参数 |
| `internal/handler/audit.go` | `AuditHandler` 接口新增 `HandleTokenRate` / `HandleTokenUsage`；handler 实现 |
| `internal/router/audit.go` | 注册两个新路由 |
| `internal/bootstrap/container.go` | 注册新 handler 到 dig |

---

## 前端

### 改动文件清单

| 文件 | 改动 |
|---|---|
| `web/src/lib/types.ts` | 新增 `TokenRateReq/Rsp/Item/Point`、`TokenUsageReq/Rsp/Item` 类型 |
| `web/src/lib/api-client.ts` | 新增 `fetchTokenRate` / `fetchTokenUsage` 方法 |
| `web/src/components/charts/model-token-bar-chart.tsx` | **重写**：柱状图 → 排名表格组件 |
| `web/src/components/charts/token-rate-chart.tsx` | 改用新 API |
| `web/src/components/charts/token-volume-chart.tsx` | 无改动（仅 CardTitle 改名 "Token Throughput"） |

### Model Usage 表格设计

| 列 | 说明 |
|---|---|
| # | 排名，按 Total 降序 |
| Model | 模型名称 |
| Total | input + output + cache read + cache creation（前端聚合） |
| Input | 堆叠比例条：Cache Read（#7C6BA5）+ New Input（#D97757），底部标注数值 |
| Output | 堆叠比例条：Cache Created（#4A9E7D）+ Output（#5B8DB8），底部标注数值 |

交互：列头点击排序（默认 Total 降序）、比例条悬停 tooltip、TimeRangePicker 保留右上角。

---

## 不变更

- `QueryTokenThroughput` 仓库方法及 SQL
- `TokenThroughputReq` / `TokenThroughputRsp` DTO
- Request Rate、Model Trend 等其他审计端点
