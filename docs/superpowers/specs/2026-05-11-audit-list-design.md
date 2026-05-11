# 审计日志列表接口设计

## 1. 概述

基于已有的 `model_call_audit` 表，新增分页查询接口，支持按用户 API Key 查看自己的调用记录，支持按 trace_id、model 等关键字搜索，支持时间范围过滤和多字段排序。

## 2. 路由与认证

- **路由组**: `/api/v1/audit`
- **认证方式**: `apiKeyAuth`（通过 `APIKeyMiddleware` 注入 `CtxKeyAPIKeyID`，自动限定只查该 Key 的调用记录）
- **新建路由文件**: `internal/router/audit.go`

### 注册端点

```
GET /api/v1/audit/logs
```

## 3. 请求参数

| 参数 | 类型 | 来源 | 必填 | 默认值 | 说明 |
|------|------|------|------|--------|------|
| `page` | int | query | 否 | 1 | 页码 |
| `pageSize` | int | query | 否 | 20 | 每页条数，最大 100 |
| `query` | string | query | 否 | `""` | 搜索关键词，模糊匹配 `trace_id`、`model` |
| `startTime` | time.Time | query | 否 | - | 开始时间（ISO 8601），不传则不限制 |
| `endTime` | time.Time | query | 否 | - | 结束时间（ISO 8601），不传则不限制 |
| `sort` | string | query | 否 | `desc` | 排序方向，枚举值 `asc` / `desc` |
| `sortField` | string | query | 否 | `created_at` | 排序字段，支持 `created_at`、`input_tokens`、`output_tokens`、`first_token_latency_ms`、`stream_duration_ms` |

## 4. 响应格式

```json
{
  "Body": {
    "Error": null,
    "Logs": [
      {
        "ID": 1,
        "CreatedAt": "2026-05-11T10:00:00Z",
        "Model": "gpt-4o",
        "UpstreamProvider": "openai",
        "APIProvider": "openai",
        "InputTokens": 100,
        "OutputTokens": 50,
        "CacheCreationInputTokens": 0,
        "CacheReadInputTokens": 0,
        "FirstTokenLatencyMs": 200,
        "StreamDurationMs": 1500,
        "UserAgent": "curl/8.0",
        "UpstreamStatusCode": 200,
        "ErrorMessage": "",
        "TraceID": "abc123"
      }
    ],
    "PageInfo": {
      "Page": 1,
      "PageSize": 20,
      "Total": 150
    }
  }
}
```

## 5. 分层设计

### 5.1 DTO 层

- **文件**: `internal/dto/audit.go`（新增）
- `ListAuditLogsReq`: 嵌入 `model.PageParam` + `model.QueryParam` + `model.SortParam` + `StartTime` + `EndTime`
- `ListAuditLogsRsp`: 嵌入 `CommonRsp`，包含 `Logs []*AuditLogItem` + `PageInfo *model.PageInfo`
- `AuditLogItem`: 对应 `ModelCallAudit` 全部公开字段（不含 `APIKeyID`、`ModelID`）

### 5.2 领域层

- **文件**: `internal/domain/modelcall/repository.go`（修改）
- 新增方法到 `AuditRepository` 接口：
  ```go
  ListByAPIKeyID(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
  ```
- 参数说明：
  - `param`: 包含 `Page`、`PageSize`、`Query`（用于模糊搜索）、`Sort`、`SortField`
  - `startTime` / `endTime`: 时间范围过滤（零值表示不限）

### 5.3 查询 handler 层

- **文件**: `internal/application/audit/query/list_audit_logs.go`（新增）
- 接口方法：
  ```go
  ListAuditLogs(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
  ```
- 校验 `PageSize` 上限为 100
- 校验 `SortField` 白名单
- 调用 domain repository 的 `ListByAPIKeyID`

### 5.4 基础设施层

- **文件**: `internal/infrastructure/repository/audit_repository.go`（修改）
- 新增 `ListByAPIKeyID` 方法实现，调用 `dao.Paginate`
- 时间过滤：在 `dao.QueryParam.QueryFields` 中添加 `created_at` 字段，在调用 `dao.Paginate` 时传入 `where` 条件包含 `api_key_id`
- 搜索（Query）：在 `QueryFields` 中包含 `trace_id`、`model`
- 排序支持：`dao.CommonParam` 已内置 `SortParam`，透传即可
- 对于 `sortField` 不是 GORM 默认字段的情况，需在 DAO 层做字段映射

### 5.5 Handler 层

- **文件**: `internal/handler/audit.go`（新增）
- 从 context 获取 `apiKeyID = util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID)`
- 调用 query handler 获取结果
- 将 aggregate 列表映射为 DTO 列表返回
- 错误处理：使用 `ierr.ToBizError(err, ierr.ErrInternal.BizError())`

### 5.6 路由注册

- **文件**: `internal/router/audit.go`（新增）
- 使用 `apiKeyAuth` 认证
- 注册 `GET /logs` 端点

## 6. 依赖注入变更

- `internal/bootstrap/container.go`:
  - `provideApplication()`: 注册 `NewListAuditLogsHandler`
  - `provideHandlers()`: 注册 `NewAuditHandler`
- `internal/bootstrap/router.go`: `routeParams` 添加 `AuditHandler`
- `internal/router/router.go`: `APIRouterDependencies` 添加 `AuditHandler`

## 7. 安全约束

- 接口强制使用 `apiKeyAuth`，从请求头解析 API Key 并注入 `CtxKeyAPIKeyID`
- 查询始终限定 `api_key_id = ctx.apiKeyID`，用户只能查看自己的记录
- 不暴露 `api_key_id` 和 `model_id` 字段到响应中

## 8. 测试要点

| 测试场景 | 说明 |
|---------|------|
| 分页查询 | 验证分页参数正确返回 |
| 空结果 | 新 Key 无调用记录时返回空列表 |
| 时间过滤 | start/end 时间范围内外均验证 |
| 关键字搜索 | trace_id 部分匹配、model 名模糊匹配 |
| 排序 | 多字段排序正确 |
| 认证拦截 | 无 API Key 返回 401 |
| PageSize 上限 | 超过 100 截断或报错 |
