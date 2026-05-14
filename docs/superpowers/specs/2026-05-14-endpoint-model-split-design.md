# 拆分 model_endpoint 表为 endpoint + model 设计文档

**日期**: 2026-05-14
**状态**: 待评审

---

## 背景

当前 `model_endpoint` 表是一个扁平表，将端点（上游服务）和模型（对外别名）绑在同一行。存在以下问题：

1. 一个上游端点可能支持多个模型，但表结构要求每个模型独立一行，导致端点信息（api_key、base_url）重复
2. 一个端点同时拥有 Anthropic 和 OpenAI 两个 base URL，但表结构只能存一个
3. 部分端点不支持 response/chat completion/message 接口，无字段记录

## 目标

将 `model_endpoint` 拆成 `endpoint` 和 `model` 两张表，支持一个端点关联多个模型，独立存储两个协议的 base URL，并在端点级别记录接口支持情况。

---

## 表结构设计

### endpoint 表

```sql
CREATE TABLE endpoint (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,                    -- 端点标识名，如 "aws-bedrock"
    openai_base_url VARCHAR(512) NOT NULL,                    -- OpenAI 协议 base URL
    anthropic_base_url VARCHAR(512) NOT NULL,                 -- Anthropic 协议 base URL
    api_key         VARCHAR(512) NOT NULL,                    -- 共享 API Key
    support_openai_chat_completion  BOOLEAN NOT NULL DEFAULT TRUE,  -- 支持 /chat/completions
    support_openai_response         BOOLEAN NOT NULL DEFAULT FALSE, -- 支持 /responses
    support_anthropic_message       BOOLEAN NOT NULL DEFAULT FALSE, -- 支持 /messages
    deleted_at      BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE,
    updated_at      TIMESTAMP WITH TIME ZONE
);
CREATE UNIQUE INDEX idx_endpoint_name_deleted ON endpoint(name, deleted_at);
```

### model 表

```sql
CREATE TABLE model (
    id              SERIAL PRIMARY KEY,
    alias           VARCHAR(255) NOT NULL,                    -- 对外暴露的模型别名
    model           VARCHAR(255) NOT NULL,                    -- 上游实际模型名
    endpoint_id     INTEGER NOT NULL,                         -- 逻辑外键 → endpoint.id
    deleted_at      BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE,
    updated_at      TIMESTAMP WITH TIME ZONE
);
CREATE UNIQUE INDEX idx_model_alias_endpoint_deleted ON model(alias, endpoint_id, deleted_at);
```

### 约束说明

- `endpoint.name` 在活跃记录（deleted_at = 0）中唯一
- `model(alias, endpoint_id, deleted_at)` 组合唯一，防止同一别名在同一端点重复注册
- 同一 `alias` 可通过不同 `endpoint_id` 关联多个端点，解析时随机选择
- 两个表之间使用逻辑外键，不建立物理外键约束

---

## 领域模型变更

### aggregate.Endpoint（端点聚合根）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | 端点 ID |
| name | string | 端点名称 |
| openaiBaseURL | string | OpenAI base URL |
| anthropicBaseURL | string | Anthropic base URL |
| apiKey | string | 共享 API Key |
| supportOpenAIChatCompletion | bool | 支持 /chat/completions |
| supportOpenAIResponse | bool | 支持 /responses |
| supportAnthropicMessage | bool | 支持 /messages |

### aggregate.Model（新增关联实体）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint | 模型关联 ID |
| alias | vo.EndpointAlias | 对外暴露的模型别名 |
| model | string | 上游实际模型名 |
| endpointID | uint | 逻辑外键 → endpoint.id |

### vo.UpstreamCreds 变更

- `model` 字段来源：从 `model` 表读取
- `baseURL` 字段来源：从 `endpoint` 表按请求协议取 `openai_base_url` 或 `anthropic_base_url`
- `apiKey` 字段来源：从 `endpoint` 表读取

### 解析逻辑变更

**旧流程**：
```
EndpointResolver.Resolve(alias, provider) → 查 model_endpoint 表 → 返回唯一 endpoint
```

**新流程**：
```
EndpointResolver.Resolve(alias)
  1. 查 model 表（按 alias）→ 收集所有 endpointID
  2. 随机选一个 endpointID
  3. 查 endpoint 表（按 id）
  4. 返回 (endpoint, model)
  5. 调用方根据请求协议取对应 base_url，检查接口支持标记
```

---

## 仓储接口变更

### 删除

```go
EndpointRepository.FindByAliasAndProvider(ctx, alias, provider) (*Endpoint, error)
EndpointReadRepository.FindCredentialByAliasAndProvider(ctx, alias, provider) (*CredentialProjection, error)
```

### 新增

```go
// 写仓储
EndpointRepository.FindByID(ctx, id) (*aggregate.Endpoint, error)
ModelRepository.FindByAlias(ctx, alias) ([]*aggregate.Model, error)

// 读仓储
EndpointReadRepository.FindByID(ctx, id) (*EndpointProjection, error)
ModelReadRepository.FindByAlias(ctx, alias) ([]*ModelProjection, error)
```

### EndpointResolver 接口

```go
// 旧
Resolve(ctx, alias vo.EndpointAlias, primary, fallback enum.ProviderType) (*Endpoint, error)

// 新
Resolve(ctx, alias vo.EndpointAlias) (*aggregate.Endpoint, *aggregate.Model, error)
```

---

## GORM Model 定义

```go
// 文件：internal/infrastructure/database/model/endpoint.go
type Endpoint struct {
    BaseModel
    ID                          uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:端点ID"`
    Name                        string `json:"name" gorm:"column:name;not null;uniqueIndex:idx_endpoint_name_deleted,priority:1;comment:端点名称"`
    OpenaiBaseURL               string `json:"openai_base_url" gorm:"column:openai_base_url;not null;comment:OpenAI协议baseURL"`
    AnthropicBaseURL            string `json:"anthropic_base_url" gorm:"column:anthropic_base_url;not null;comment:Anthropic协议baseURL"`
    APIKey                      string `json:"api_key" gorm:"column:api_key;not null;comment:上游API密钥"`
    SupportOpenAIChatCompletion bool   `json:"support_openai_chat_completion" gorm:"column:support_openai_chat_completion;not null;default:true;comment:支持/chat/completions"`
    SupportOpenAIResponse       bool   `json:"support_openai_response" gorm:"column:support_openai_response;not null;default:false;comment:支持/responses"`
    SupportAnthropicMessage     bool   `json:"support_anthropic_message" gorm:"column:support_anthropic_message;not null;default:false;comment:支持/messages"`
    DeletedAt                   int64  `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_endpoint_name_deleted,priority:2;comment:删除时间"`
}

// 文件：internal/infrastructure/database/model/model.go
type Model struct {
    BaseModel
    ID         uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:模型关联ID"`
    Alias      string `json:"alias" gorm:"column:alias;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:1;comment:对外暴露的模型别名"`
    Model      string `json:"model" gorm:"column:model;not null;comment:上游实际模型名"`
    EndpointID uint   `json:"endpoint_id" gorm:"column:endpoint_id;not null;uniqueIndex:idx_model_alias_endpoint_deleted,priority:2;comment:逻辑外键→endpoint.id"`
    DeletedAt  int64  `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_model_alias_endpoint_deleted,priority:3;comment:删除时间"`
}
```

### model.Models 注册

```go
var Models = []any{
    &User{},
    &Message{},
    &Session{},
    &Tool{},
    &Endpoint{},        // 新增，替代原 ModelEndpoint
    &Model{},           // 新增
    &ProxyAPIKey{},
    &ModelCallAudit{},
}
```

---

## UseCase 层变更

### OpenAIUseCase

```go
// chat completion 流程
ep, m, err := h.resolver.Resolve(ctx, req.Model)
if !ep.SupportOpenAIChatCompletion { return ErrNotSupported }
upstreamEndpoint := vo.NewUpstreamEndpointFromCredential(m.Model, ep.APIKey, ep.OpenaiBaseURL)
transport.ForwardChatCompletion(ctx, upstreamEndpoint, body)

// response 流程同理，检查 ep.SupportOpenAIResponse

// ListModels 查 model 表，按 alias 去重
```

### AnthropicUseCase

```go
// create message 流程
ep, m, err := h.resolver.Resolve(ctx, req.Model)
if !ep.SupportAnthropicMessage { return ErrNotSupported }
upstreamEndpoint := vo.NewUpstreamEndpointFromCredential(m.Model, ep.APIKey, ep.AnthropicBaseURL)
transport.ForwardCreateMessage(ctx, upstreamEndpoint, body)
```

---

## 文件变更清单（部分）

- `internal/infrastructure/database/model/model_endpoint.go` → 删除
- `internal/infrastructure/database/model/endpoint.go` → 新增
- `internal/infrastructure/database/model/model.go` → 新增（注意：与 GORM 的 `Model` 类型命名冲突，需重命名为 `dbmodel.Model` 或使用包别名）
- `internal/infrastructure/database/model/base.go` → 更新 Models 切片
- `internal/infrastructure/database/dao/model_endpoint.go` → 删除
- `internal/infrastructure/database/dao/endpoint.go` → 新增
- `internal/infrastructure/database/dao/model.go` → 新增
- `internal/infrastructure/database/dao/singleton.go` → 更新
- `internal/infrastructure/repository/endpoint_repository.go` → 重写
- `internal/infrastructure/repository/model_repository.go` → 新增
- `internal/domain/llmproxy/repository.go` → 重写接口
- `internal/domain/llmproxy/aggregate/endpoint.go` → 重写
- `internal/domain/llmproxy/aggregate/model.go` → 新增
- `internal/domain/llmproxy/vo/upstream_creds.go` → 适配（移除 model 依赖，model 由外部传入）
- `internal/domain/llmproxy/service/resolver.go` → 重写
- `internal/application/llmproxy/usecase/openai.go` → 适配
- `internal/application/llmproxy/usecase/anthropic.go` → 适配
- `internal/application/llmproxy/usecase/query.go` → 适配
- `internal/bootstrap/container.go` → 更新 DI 注册
- `internal/common/constant/sql.go` → 更新字段常量
- `internal/common/constant/database.go` → 新增 aggregate type

---

## 迁移策略

1. 新增 GORM model：`Endpoint{}`、`Model{}`，加入 `model.Models` 切片
2. 运行 `go run main.go database migrate` → GORM AutoMigrate 创建新表
3. 从旧 `model_endpoint` 表数据迁移到新表（脚本或手动）
4. 删除旧 `ModelEndpoint{}` struct 和相关 DAO/Repository
5. 手动 DROP 旧表
6. 更新所有使用方（usecase/handler/transport）

---

## 测试影响

| 测试 | 变更 |
|---|---|
| `test/unit/endpoint_resolver/` | 接口签名和解析逻辑完全重写 |
| `test/unit/proxy_config/` | 适配新表结构 |
| `test/unit/llmproxy_usecase/` | mock 接口改为新签名 |
| `test/e2e/openai_chat_completion/` | 预期兼容，alias 查询逻辑不变 |

---

## 风险与注意事项

- **命名冲突**：新增的 `model.go` 文件名和 GORM 的 `Model` 标识符可能与现有代码冲突，需使用包别名（如 `dbmodel`）
- **数据迁移**：旧 `model_endpoint` 表的行如何拆入新表需约定（一个旧行可能对应一个新 endpoint 和一个新 model）
- **无回滚**：表结构变化不可逆（除非事先备份数据）
