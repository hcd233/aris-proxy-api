# 模型调用审计表设计

## 1. 概述

为系统新增**模型调用审计表** (`model_call_audit`)，记录每次模型调用的关键指标，支持按模型统计和按 API Key 排查两种主要查询场景。

同时删除 `message` 表的 `token_count` 字段和 `session` 表的 `client` 字段，简化原有模型职责。

## 2. 背景与目标

- **现状**：`message.token_count` 只能粗粒度分摊 token，无法反映真实单次调用；`session.client` 混合了调用元数据与业务聚合语义
- **目标**：独立审计表专注调用指标，`message`/`session` 专注数据持久化，各司其职

## 3. 数据模型变更

### 3.1 删除字段

| 表 | 字段 | 原因 |
|----|------|------|
| `message` | `token_count` | 调用级 token 应记在审计表，不应分摊到 message |
| `session` | `client` | 调用元数据迁移到审计表，`session` 专注会话聚合 |

### 3.2 新增表 `model_call_audit`

```go
type ModelCallAudit struct {
    BaseModel
    ID                        uint   `gorm:"column:id;primary_key;auto_increment"`
    APIKeyID                  uint   `gorm:"column:api_key_id;not null;index:idx_api_key_id_created_at"`
    ModelID                   uint   `gorm:"column:model_id;not null;index:idx_model_id_created_at"`
    Model                     string `gorm:"column:model;not null;index:idx_model_created_at"`
    UpstreamProvider          string `gorm:"column:upstream_provider;not null"`
    APIProvider               string `gorm:"column:api_provider;not null"`
    InputTokens               int    `gorm:"column:input_tokens;default:0"`
    OutputTokens              int    `gorm:"column:output_tokens;default:0"`
    CacheCreationInputTokens  int    `gorm:"column:cache_creation_input_tokens;default:0"`
    CacheReadInputTokens      int    `gorm:"column:cache_read_input_tokens;default:0"`
    FirstTokenLatencyMs       int64  `gorm:"column:first_token_latency_ms;default:0"`
    StreamDurationMs          int64  `gorm:"column:stream_duration_ms;default:0"`
    UserAgent                 string `gorm:"column:user_agent;not null;default:''"`
    UpstreamStatusCode        int    `gorm:"column:upstream_status_code;default:0"`
    ErrorMessage              string `gorm:"column:error_message;not null;default:''"`
    TraceID                  string `gorm:"column:trace_id;not null;default:'';index"`
}
```

### 3.3 索引设计

| 索引名 | 字段 | 用途 |
|--------|------|------|
| `idx_model_created_at` | `(model, created_at)` | 按模型统计 |
| `idx_api_key_id_created_at` | `(api_key_id, created_at)` | 按 API Key 排查 |
| `idx_model_id_created_at` | `(model_id, created_at)` | 按模型实体 ID 统计 |
| `idx_trace_id` | `trace_id` | 按 trace_id 定位单次请求 |

## 4. 字段定义

| 字段 | 说明 |
|------|------|
| `api_key_id` | 关联 `proxy_api_key.id`，请求时从 context `CtxKeyAPIKeyID` 获取 |
| `model_id` | 关联 `model_endpoint.id`，端点查找后记录 |
| `model` | 对外暴露的模型别名（客户端传入的模型名），用于按模型名统计 |
| `upstream_provider` | 上游提供商类型，`openai` / `anthropic`，来自 `model_endpoint.provider` |
| `api_provider` | 本次请求入口协议类型，`openai` / `anthropic`，来自调用路由 |
| `input_tokens` | 上游返回的输入 token 数 |
| `output_tokens` | 上游返回的输出 token 数 |
| `cache_creation_input_tokens` | Anthropic 缓存创建 token 数；OpenAI 无此字段则填 0 |
| `cache_read_input_tokens` | Anthropic 缓存读取 token 数；OpenAI 无此字段则填 0 |
| `first_token_latency_ms` | 请求开始到收到首 token 的耗时（毫秒），流式调用有效 |
| `stream_duration_ms` | 首 token 到流结束的总耗时（毫秒），流式调用有效 |
| `user_agent` | 请求头 `User-Agent` |
| `upstream_status_code` | 上游 HTTP 响应状态码 |
| `error_message` | 上游错误信息，无错误则空字符串 |
| `trace_id` | 请求追踪 ID，从 context `CtxKeyTraceID` 获取 |

### 4.1 `api_provider` 与 `upstream_provider` 的区别

| 场景 | `api_provider` | `upstream_provider` |
|------|----------------|---------------------|
| OpenAI 接口调用 OpenAI 上游 | `openai` | `openai` |
| OpenAI 接口调用 Anthropic 上游 | `openai` | `anthropic` |
| Anthropic 接口调用 Anthropic 上游 | `anthropic` | `anthropic` |
| Anthropic 接口调用 OpenAI 上游 | `anthropic` | `openai` |

## 5. 写入策略

### 5.1 写入时机

每次模型调用**完成后**写入 1 条审计记录。

- **非流式调用**：在拿到上游完整响应后写
- **流式调用**：在流结束后写

### 5.2 写入方式

新增独立异步任务 `ModelCallAuditTask`，通过 `storePool` 异步落库，不阻塞响应：

```
Service 调用 Proxy → 拿到结果后组装 AuditTask → submitAuditTask → storePool 异步写入
```

- 审计写入失败不影响消息存储，也不影响 API 响应
- 审计写入失败仅记录 error log

### 5.3 流式调用计时方式

```go
startTime := time.Now()
var firstTokenTime time.Time
// ... 流式循环
// 收到第一个有效 content 时：
firstTokenTime = time.Now()
firstTokenLatencyMs := firstTokenTime.Sub(startTime).Milliseconds()
// 流结束后：
streamDurationMs := time.Now().Sub(firstTokenTime).Milliseconds()
```

- `first_token_latency_ms` = `firstTokenTime - startTime`（请求到首 token）
- `stream_duration_ms` = `endTime - firstTokenTime`（首 token 到流结束）
- 非流式调用：两个字段均填 0

## 6. 数据来源

| 字段 | 来源 |
|------|------|
| `api_key_id` | `CtxKeyAPIKeyID`（APIKeyMiddleware 注入） |
| `model_id` | `model_endpoint.id`（端点查找结果） |
| `model` | 请求体中的模型名字段（客户端传入） |
| `upstream_provider` | `model_endpoint.provider`（端点查找结果） |
| `api_provider` | 路由来源：OpenAI 接口 = `openai`，Anthropic 接口 = `anthropic` |
| `input_tokens` | 上游 usage（OpenAI: `prompt_tokens`，Anthropic: `input_tokens`） |
| `output_tokens` | 上游 usage（OpenAI: `completion_tokens`，Anthropic: `output_tokens`） |
| `cache_creation_input_tokens` | Anthropic usage；OpenAI 填 0 |
| `cache_read_input_tokens` | Anthropic usage；OpenAI 填 0 |
| `first_token_latency_ms` | 计时（仅流式） |
| `stream_duration_ms` | 计时（仅流式） |
| `user_agent` | `CtxKeyClient`（APIKeyMiddleware 注入） |
| `upstream_status_code` | Proxy 层返回的错误状态码 |
| `error_message` | Proxy 层返回的错误信息 |
| `trace_id` | `CtxKeyTraceID`（TraceMiddleware 注入） |

## 7. 迁移策略

### 7.1 Schema 迁移

- 通过 GORM AutoMigrate 执行（随服务启动自动应用）
- 新增 `model_call_audit` 表
- 删除 `message.token_count`
- 删除 `session.client`

### 7.2 历史数据

**不做历史回填**。原因：
- 原有 `message.token_count` 按 message 粒度分摊，不可信
- `session.client` 无法恢复时延、缓存 token 等指标
- 回填数据会误导后续统计分析

### 7.3 代码改动范围

| 层级 | 改动文件 |
|------|---------|
| Model | `model/message.go`（删除 `TokenCount`） |
| Model | `model/session.go`（删除 `Client`） |
| Model | `model/model_call_audit.go`（新增） |
| DAO | `dao/model_call_audit.go`（新增） |
| DAO | `dao/singleton.go`（注册 `ModelCallAuditDAO`） |
| DTO | `dto/asynctask.go`（新增 `ModelCallAuditTask`） |
| Pool | `pool/store_pool.go`（新增 `submitAuditTask` 方法） |
| Pool | `pool/pool.go`（新增 `SubmitModelCallAuditTask` 方法） |
| Service | `service/openai.go`（组装并提交 `ModelCallAuditTask`） |
| Service | `service/anthropic.go`（组装并提交 `ModelCallAuditTask`） |
| Constant | `constant/string.go`（如需 Redis Key 模板则新增） |

## 8. 测试要点

| 测试场景 | 说明 |
|---------|------|
| 流式调用写入审计 | 验证所有字段正确落库 |
| 非流式调用写入审计 | 验证 `first_token_latency_ms` 和 `stream_duration_ms` 为 0 |
| 跨协议调用写入审计 | OpenAI 接口→Anthropic 上游，验证 `api_provider`/`upstream_provider` 正确 |
| 上游错误写入审计 | 验证 `upstream_status_code` 和 `error_message` 正确 |
| 删除字段迁移 | 验证 `message.token_count` 和 `session.client` 已删除 |
| 审计写入失败 | 验证不影响 API 响应和消息存储 |
