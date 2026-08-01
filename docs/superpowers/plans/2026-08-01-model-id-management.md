# Model ID 管理与冗余字段改造实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Model 实体新增业务 `modelId`（string，创建默认=alias，可更新，不唯一），并将 model_call_audit / session / message 三张表的 model 冗余字段改为存储 modelId 值、列名改为 `model_id` / `model_ids`。

**Architecture:** 后端 Go（GORM + dig DI），按「Model 聚合 → 写路径（审计/消息存储）→ 读路径（查询/DTO）→ 存量迁移 → 前端」顺序推进。所有对外 JSON 字段统一为 `modelId` / `modelIds`。`trace` 表不动。`Model` 表自身的 `model` 列（上游实际模型名）**不改名**。

**Tech Stack:** Go 1.25 / GORM / Fiber / Huma / Next.js（web/）

## Global Constraints

- 业务错误必须走 `internal/common/ierr`，禁止 `fmt.Errorf` / `errors.New`。
- DTO/JSON 统一用 `github.com/bytedance/sonic`。
- 测试只用标准库 `testing`；单测放 `test/unit/<topic>/`，E2E 放 `test/e2e/<topic>/`。
- 日志前缀 `[PascalCaseModule]`。
- `Model` 表的 `model` 列（column:model，上游实际模型名）保留不动；只改 `model_call_audits` / `sessions` / `messages` 三张表的列。
- GORM AutoMigrate 不能改列名/删列，存量迁移必须幂等（重复执行不报错）。
- API filter 参数 key（`field=model`）是协议名不是列名，保留 `"model"` 不变；只改 SQLColumn 引用。
- 每次任务结束跑 `go build ./...` 与对应单测。

---

### Task 1: Model 实体新增 modelId 字段

**Files:**
- Modify: `internal/infrastructure/database/model/model.go`
- Modify: `internal/domain/llmproxy/aggregate/model.go`
- Modify: `internal/infrastructure/repository/endpoint_repository.go`（modelRepository + toModelAggregate/toModelDBModel）
- Modify: `internal/common/constant/sql.go`（ModelRepoFieldsFull 加 FieldModelID）
- Modify: `internal/application/model/port/handler.go`
- Modify: `internal/application/model/command/update_model.go`
- Modify: `internal/dto/model.go`
- Modify: `internal/handler/model.go`
- Test: `test/unit/model_repository/model_update_test.go`

**Interfaces:**
- Consumes: 现有 `aggregate.CreateModel` 签名（**不修改**）。
- Produces: `(*aggregate.Model).ModelID() string`、`(*aggregate.Model).SetModelID(string)`、`(*aggregate.Model).Update(... modelID *string ...)`；`UpdateModelCommand.ModelID *string`；`ModelView.ModelID string`；`dto.ModelItem.ModelID string`。

- [ ] **Step 1: dbmodel.Model 新增字段**

`internal/infrastructure/database/model/model.go` 在 `Alias` 字段后新增：

```go
ModelID         string   `json:"model_id" gorm:"column:model_id;not null;default:'';comment:业务模型ID(创建默认=alias,可更新)"`
```

- [ ] **Step 2: 聚合新增 modelID 字段与访问器**

`internal/domain/llmproxy/aggregate/model.go`：

- 结构体新增 `modelID string` 字段。
- `CreateModel` 内部构造 `m` 时新增 `modelID: alias.String()`（创建时默认=alias，无需改签名）。
- 新增方法：

```go
// ModelID 返回业务模型 ID
func (m *Model) ModelID() string { return m.modelID }

// SetModelID 设置业务模型 ID（仓储恢复用）
func (m *Model) SetModelID(modelID string) { m.modelID = modelID }
```

- `Update` 签名新增 `modelID *string` 参数，函数体新增：

```go
if modelID != nil {
	if *modelID == "" {
		return ierr.New(ierr.ErrValidation, "model id cannot be empty")
	}
	m.modelID = *modelID
}
```

- [ ] **Step 3: repository 恢复与写入 modelID**

`internal/infrastructure/repository/endpoint_repository.go`：

- `toModelAggregate`：`CreateModel(...)` 后新增 `model.SetModelID(m.ModelID)`。
- `toModelDBModel`：新增 `ModelID: m.ModelID(),`。
- `Update` 的 `updates` map 新增：

```go
constant.FieldModelID: m.ModelID(),
```

- [ ] **Step 4: constant 加字段**

`internal/common/constant/sql.go`：`ModelRepoFieldsFull` 改为：

```go
ModelRepoFieldsFull = []string{FieldID, FieldAlias, FieldModelID, FieldModel, FieldEndpointID, FieldEnabled, FieldModelContextLength, FieldModelMaxOutputTokens, FieldModelCapabilities, FieldCreatedAt, FieldUpdatedAt}
```

（`FieldModelID = "model_id"` 已存在；`FieldModel` 保留给 Model 表 `model` 列。）

- [ ] **Step 5: port 命令与视图**

`internal/application/model/port/handler.go`：

- `UpdateModelCommand` 新增 `ModelID *string`。
- `ModelView` 新增 `ModelID string`。

- [ ] **Step 6: 命令透传**

`internal/application/model/command/update_model.go` 在 `Update` 调用前构造指针并传给 `m.Update`：

```go
var modelIDPtr *string
if cmd.ModelID != nil {
	id := *cmd.ModelID
	modelIDPtr = &id
}
```

调用处更新为：`m.Update(aliasPtr, cmd.ModelName, cmd.EndpointID, cmd.Enabled, cmd.ContextLength, cmd.MaxOutputTokens, cmd.Capabilities, modelIDPtr)`（参数顺序：... capabilities 之后追加 modelIDPtr）。

- [ ] **Step 7: DTO 与 Handler**

`internal/dto/model.go`：

- `UpdateModelReqBody` 新增 `ModelID *string \`json:"modelId,omitempty" doc:"业务模型ID(非空)"\``。
- `ModelItem` 新增 `ModelID string \`json:"modelId" doc:"业务模型ID"\``。

`internal/handler/model.go`：

- `HandleUpdateModel` 命令新增 `ModelID: req.Body.ModelID,`。
- `HandleListModels` 的 `ModelItem` 构造新增 `ModelID: v.ModelID,`。

- [ ] **Step 8: 运行验证**

Run: `go build ./...`
Expected: 编译通过；`model_update_test.go` 仅检查 Update map 常量与 gorm tag 匹配（可增强新增 `{goField: "ModelID", constant: constant.FieldModelID}` 检查项，非强制）。

Run: `go test -count=1 ./test/unit/model_repository/ ./test/unit/endpoint_resolver/`
Expected: PASS（endpoint_resolver 的 `CreateModel` 调用不受影响，签名未变）。

- [ ] **Step 9: Commit**

```bash
git add internal/infrastructure/database/model/model.go internal/domain/llmproxy/aggregate/model.go internal/infrastructure/repository/endpoint_repository.go internal/common/constant/sql.go internal/application/model/port/handler.go internal/application/model/command/update_model.go internal/dto/model.go internal/handler/model.go test/unit/model_repository/
git commit -m "feat(model): 新增业务 modelId 字段，创建默认=alias，可更新"
```

---

### Task 2: 审计写路径改造（聚合 + 任务 + recorder）

**Files:**
- Modify: `internal/infrastructure/database/model/model_call_audit.go`
- Modify: `internal/domain/modelcall/aggregate/audit.go`
- Modify: `internal/domain/modelcall/aggregate/reconstruct.go`
- Modify: `internal/dto/asynctask.go`
- Modify: `internal/application/llmproxy/usecase/recorder.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`（blocked 路径）
- Modify: `internal/application/llmproxy/usecase/anthropic.go`（blocked 路径）
- Test: `test/unit/model_call_audit/model_call_audit_test.go`、`test/unit/pool_manager/audit_write_test.go`、`test/unit/audit_architecture/audit_architecture_test.go`

**Interfaces:**
- Consumes: Task 1 的 `(*aggregate.Model).ModelID() string`。
- Produces: `RecordCallInput.ModelID string`（删除 `Model string`）；`ReconstructAuditInput.ModelID string`（删除 `Model`）；`dto.ModelCallAuditTask.ModelID string`（删除 `Model`）。

- [ ] **Step 1: dbmodel 改列**

`internal/infrastructure/database/model/model_call_audit.go`：删除 `ModelID uint` 行，`Model string` 行改为：

```go
ModelID string `json:"model_id" gorm:"column:model_id;not null;default:'';comment:业务模型ID(创建默认=alias);index:idx_model_id_created_at,priority:1"`
```

（索引 `idx_model_created_at` 随旧列删除；`idx_model_id_created_at` 保留但语义变为业务 modelId。）

- [ ] **Step 2: 聚合改造**

`internal/domain/modelcall/aggregate/audit.go`：

- 结构体 `modelID uint` → `modelID string`；删除 `model string` 字段。
- `RecordCallInput`：`ModelID uint` → `ModelID string`；删除 `Model string`。
- `newAudit`：删除 `model: input.Model,`；`modelID: input.ModelID,` 不变。
- getter：`ModelID() uint` → `ModelID() string`；删除 `Model() string`。

`internal/domain/modelcall/aggregate/reconstruct.go`：`ReconstructAuditInput` 的 `ModelID uint` → `ModelID string`、删除 `Model string`；`ReconstructAudit` 删除 `model: input.Model,`。

- [ ] **Step 3: 任务 DTO**

`internal/dto/asynctask.go`：`ModelCallAuditTask` 的 `ModelID uint` → `ModelID string`，删除 `Model string` 字段。

- [ ] **Step 4: recorder 组装**

`internal/application/llmproxy/usecase/recorder.go` 的 `recordModelCall`：

```go
task := &dto.ModelCallAuditTask{
	Ctx:                 util.CopyContextValues(ctx),
	ModelID:             out.model.ModelID(),
	Endpoint:            out.endpoint,
	UpstreamProtocol:    out.upstreamProtocol,
	APIProtocol:         out.apiProtocol,
	FirstTokenLatencyMs: out.firstTokenLatencyMs,
	StreamDurationMs:    out.streamDurationMs,
}
```

（删除 `Model: out.exposedModel,` 行。`callOutcome.exposedModel` 字段保留，其他地方仍使用。）

- [ ] **Step 5: blocked 路径**

`internal/application/llmproxy/usecase/openai.go` 与 `anthropic.go` 的 blocked auditTask：

```go
auditTask := &dto.ModelCallAuditTask{
	Ctx:              util.CopyContextValues(ctx),
	ModelID:          m.ModelID(),
	Endpoint:         ep.Name(),
	UpstreamProtocol: upstreamProtocol,
	APIProtocol:      enum.ProtocolOpenAIChatCompletion, // anthropic.go 为 ProtocolAnthropicMessage
	ErrorMessage:     fmt.Sprintf(constant.BlockedAuditRemarkTemplate, formatBlockedWords(words)),
}
```

（删除 `Model: req.Body.Model,`。）

- [ ] **Step 6: 修复单测**

- `test/unit/model_call_audit/model_call_audit_test.go`：fixture 结构体 `ModelID uint` → `string`，删除 `Model string`；`task.Model` 断言改为 `task.ModelID`。
- `test/unit/pool_manager/audit_write_test.go`：`ModelID: 7` → `ModelID: "gpt-test"`，删除 `Model: "gpt-test"`；`audit.ModelID() != 7` → `!= "gpt-test"`，`%d` → `%q`。
- `test/unit/audit_architecture/audit_architecture_test.go` 不涉及 Model 字段，无需修改（运行确认即可）。

- [ ] **Step 7: 运行验证**

Run: `go build ./... && go test -count=1 ./test/unit/model_call_audit/ ./test/unit/pool_manager/ ./test/unit/audit_architecture/`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/infrastructure/database/model/model_call_audit.go internal/domain/modelcall/ internal/dto/asynctask.go internal/application/llmproxy/usecase/recorder.go internal/application/llmproxy/usecase/openai.go internal/application/llmproxy/usecase/anthropic.go test/unit/model_call_audit/ test/unit/pool_manager/ test/unit/audit_architecture/
git commit -m "feat(audit): model_call_audit model 列改为 model_id 存储业务 modelId"
```

---

### Task 3: 审计读路径改造（repo SQL + 查询 + DTO）

**Files:**
- Modify: `internal/common/constant/sql.go`（AuditRepoFields、AuditQueryFields、AuditDistinctSelectModel、AuditDistinctWhereModel、AuditFilterModelSQLColumn）
- Modify: `internal/infrastructure/repository/audit_repository.go`
- Modify: `internal/domain/modelcall/repository.go`（4 个 Point 结构）
- Modify: `internal/application/audit/query/list_audit_logs.go`
- Modify: `internal/application/audit/query/service.go`
- Modify: `internal/application/audit/port/fill_series.go`
- Modify: `internal/dto/audit.go`（AuditLogItem）
- Modify: `internal/dto/audit_stats.go`（ModelTrendItem 等）
- Modify: `internal/application/audit/port/handler.go`（AuditLogView 结构）
- Test: `test/unit/audit_dto/audit_dto_test.go`、`test/unit/audit_repo/audit_option_sql_test.go`、`test/unit/audit_query/audit_query_test.go`

**Interfaces:**
- Consumes: Task 2 的 `(*aggregate.ModelCallAudit).ModelID() string`。
- Produces: `dto.AuditLogItem.ModelID string`（json `modelId`）；`dto.ModelTrendItem.ModelID string`（json `modelId`）。

- [ ] **Step 1: constant SQL 引用改列**

`internal/common/constant/sql.go`：

```go
AuditRepoFields = []string{AuditRepoFieldIDQualified, FieldAPIKeyID, FieldModelID, FieldUpstreamProtocol, FieldAPIProtocol, FieldEndpoint, FieldInputTokens, FieldOutputTokens, FieldCacheCreationInputTokens, FieldCacheReadInputTokens, FieldFirstTokenLatencyMs, FieldStreamDurationMs, FieldUserAgent, FieldUpstreamStatusCode, FieldErrorMessage, FieldTraceID, AuditRepoFieldCreatedAtQualified}

AuditQueryFields = []string{FieldTraceID, FieldModelID}

AuditFilterModelSQLColumn = "model_id"

AuditDistinctSelectModel = "DISTINCT model_id"
AuditDistinctWhereModel   = "model_id LIKE ?"
```

（`AuditFilterFieldModel = "model"` 保留——API filter key 非列名。）

- [ ] **Step 2: repository 同步**

`internal/infrastructure/repository/audit_repository.go`：

- `Save`：删除 `Model: audit.Model(),` 行。
- `ReconstructAudit` 调用处：删除 `Model: rec.Model,`。
- 各 `Query*`（Trend/Rate/Throughput/Latency）与 `ListDistinctModels` 的 `Select/Group/Where` 引用 `constant.FieldModel` → `constant.FieldModelID`（检查每处 SQL 拼接，`FieldModelID` 即 `"model_id"`）。

- [ ] **Step 3: Point 结构改名**

`internal/domain/modelcall/repository.go`：4 个 Point（ModelTrendPoint/RequestRatePoint/TokenThroughputPoint/FirstTokenLatencyPoint）的 `Model string \`gorm:"column:model"\`` → `ModelID string \`gorm:"column:model_id"\``。

- [ ] **Step 4: 查询层 getter 同步**

- `internal/application/audit/query/list_audit_logs.go`：`Model: audit.Model()` → `ModelID: audit.ModelID()`。
- `internal/application/audit/query/service.go:115`：`Model: v.Model` → `ModelID: v.ModelID`（确认该处为 Point → DTO 转换）。
- `internal/application/audit/port/fill_series.go`：`func(p *modelcall.ModelTrendPoint) string { return p.Model }` → `p.ModelID`；`RequestRatePoint` 同理；`IndexSeries` 内部变量名按需同步。
- `internal/application/audit/query/first_token_latency.go:76`、`token_usage.go:75`、`token_rate.go:76`：`p.Model` → `p.ModelID`。

- [ ] **Step 5: DTO 改名**

- `internal/dto/audit.go`：`AuditLogItem.Model string \`json:"model" doc:"模型名"\`` → `ModelID string \`json:"modelId" doc:"业务模型ID"\``。
- `internal/dto/audit_stats.go`：`ModelTrendItem.Model`、`RequestRateItem`（若存在）等 → `ModelID`，json `modelId`。
- `internal/application/audit/port/handler.go`：`AuditLogView.Model string` → `ModelID string`（port 层投影）。
- `internal/application/audit/query/service.go:115`：`Model: v.Model` → `ModelID: v.ModelID`（toPortAuditLogViews）。

- [ ] **Step 6: 修复单测**

- `test/unit/audit_dto/audit_dto_test.go`：`Model: "gpt-4o"` → `ModelID: "gpt-4o"`；`obj["model"]` 断言 → `obj["modelId"]`。
- `test/unit/audit_repo/audit_option_sql_test.go`：若断言 `AuditRepoFields` 长度/内容，同步去掉 FieldModel。
- `test/unit/audit_query/audit_query_test.go`：Point 断言字段 `Model` → `ModelID`。

- [ ] **Step 7: 运行验证**

Run: `go build ./... && go test -count=1 ./test/unit/audit_dto/ ./test/unit/audit_repo/ ./test/unit/audit_query/`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/common/constant/sql.go internal/infrastructure/repository/audit_repository.go internal/domain/modelcall/repository.go internal/application/audit/ internal/dto/audit.go internal/dto/audit_stats.go test/unit/audit_dto/ test/unit/audit_repo/ test/unit/audit_query/
git commit -m "feat(audit): 审计读路径按 model_id 列查询与展示"
```

---

### Task 4: 消息存储链路改造（message + store 投递 + store_pool + checksum）

**Files:**
- Modify: `internal/infrastructure/database/model/message.go`
- Modify: `internal/domain/conversation/aggregate/message.go`
- Modify: `internal/infrastructure/repository/message_repository.go`
- Modify: `internal/dto/asynctask.go`（MessageStoreTask）
- Modify: `internal/application/llmproxy/usecase/openai_store.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_store.go`
- Modify: `internal/application/llmproxy/usecase/openai_chat.go`
- Modify: `internal/application/llmproxy/usecase/anthropic_message.go`
- Modify: `internal/application/llmproxy/usecase/openai_response.go`
- Modify: `internal/infrastructure/pool/store_pool.go`
- Modify: `internal/common/vo/checksum.go`
- Modify: `internal/common/constant/sql.go`（MessageRepoFieldsFull/Detail）
- Test: `test/unit/message_checksum/`、`test/unit/pool_manager/pool_manager_test.go`（若含 store 逻辑）

**Interfaces:**
- Consumes: Task 1 的 `ModelID()`；resolver 解析出的 `m *aggregate.Model`（各 store 调用点已有 `m`）。
- Produces: `dto.MessageStoreTask.ModelID string`；`extractAssistantModelIDs`；`dbmodel.Session.ModelIDs`（Task 5 消费）。

- [ ] **Step 1: message dbmodel 与聚合**

`internal/infrastructure/database/model/message.go`：

```go
ModelID string `json:"model_id" gorm:"column:model_id;not null;default:'';comment:业务模型ID(创建默认=alias)"`
```

`internal/domain/conversation/aggregate/message.go`：

- 字段 `model string` → `modelID string`（注释同步：业务模型ID，仅 assistant 有）。
- `RecordMessage` / `RestoreMessage` 参数 `upstreamModel string` → `modelID string`。
- getter `Model() string` → `ModelID() string`。

`internal/infrastructure/repository/message_repository.go`：

- `Model: m.Model()` → `ModelID: m.ModelID()`。
- `RestoreMessage(m.ID, m.Message, m.Model, m.CheckSum)` → `m.ModelID`。

`internal/common/constant/sql.go`：`MessageRepoFieldsFull` / `MessageRepoFieldsDetail` 中 `FieldModel` → `FieldModelID`。

- [ ] **Step 2: MessageStoreTask**

`internal/dto/asynctask.go`：`MessageStoreTask.Model string` → `ModelID string`（注释：业务模型ID）。

- [ ] **Step 3: store 函数签名与调用点**

`internal/application/llmproxy/usecase/openai_store.go`：

- `storeOpenAIChatFromCompletion(ctx, req, completion, proxyErr, upstreamModel string)` → 参数名 `upstreamModel` 改为 `modelID string`。
- `storeOpenAIChatMessages` 同；`MessageStoreTask{ ... Model: upstreamModel, ...}` → `ModelID: modelID,`。
- `submitResponseMessageStoreTask(ctx, submitter, req, upstreamModel string, ...)` → `modelID string`；`MessageStoreTask` 内 `ModelID: modelID,`。

`internal/application/llmproxy/usecase/anthropic_store.go`：`storeAnthropicFromMsg` / `storeAnthropicMessages` 参数 `upstreamModel` → `modelID string`；`MessageStoreTask{ ... ModelID: modelID, }`。

调用点（各 forward 路径，`m` 为 `*aggregate.Model`）：

- `openai_chat.go`（4 处）：`m.Alias().String()` → `m.ModelID()`（含 `s.m.Alias().String()` → `s.m.ModelID()`）。
- `anthropic_message.go`（4 处）：同上。
- `openai_response.go`（6 处）：`m.Alias().String()` / `s.m.Alias().String()` → `m.ModelID()` / `s.m.ModelID()`。

- [ ] **Step 4: store_pool 落库**

`internal/infrastructure/pool/store_pool.go`：

- `messages` 构造：`Model: model` → `ModelID: model`（局部变量改名 `modelID`），条件 `lo.Contains([]enum.Role{enum.RoleAssistant}, m.Role)` 不变。
- `extractAssistantModels` 改名 `extractAssistantModelIDs`，取 `m.ModelID`：

```go
func extractAssistantModelIDs(messages []*dbmodel.Message) []string {
	candidates := lo.FilterMap(messages, func(m *dbmodel.Message, _ int) (string, bool) {
		if m.Message.Role == enum.RoleAssistant && m.ModelID != "" {
			return m.ModelID, true
		}
		return "", false
	})
	return lo.Uniq(candidates)
}
```

- `session` 构造：`Models: models` → `ModelIDs: modelIDs`。

- [ ] **Step 5: checksum 参数语义**

`internal/common/vo/checksum.go`：`ComputeMessageChecksum(msg *UnifiedMessage, model string, toolSchemas ToolSchemaMap)` 参数名 `model` → `modelID`；`messageChecksumWire` 的 `Model string \`json:"model"\`` 字段名改为 `ModelID string \`json:"model_id"\``；函数体 `Model: model,` → `ModelID: modelID,`。

（wire 是内部结构，json tag 变化使新 checksum 与旧值不同——语义本就变化，属预期。）

- [ ] **Step 6: 运行验证**

Run: `go build ./... && go test -count=1 ./test/unit/message_checksum/ ./test/unit/pool_manager/`
Expected: message_checksum 测试若硬编码期望 hash 需按新 wire 重新生成（`go test -run TestX -v` 打印实际值后更新 fixture/期望值）。

- [ ] **Step 7: Commit**

```bash
git add internal/infrastructure/database/model/message.go internal/domain/conversation/aggregate/message.go internal/infrastructure/repository/message_repository.go internal/dto/asynctask.go internal/application/llmproxy/usecase/openai_store.go internal/application/llmproxy/usecase/anthropic_store.go internal/application/llmproxy/usecase/openai_chat.go internal/application/llmproxy/usecase/anthropic_message.go internal/application/llmproxy/usecase/openai_response.go internal/infrastructure/pool/store_pool.go internal/common/vo/checksum.go internal/common/constant/sql.go test/unit/message_checksum/ test/unit/pool_manager/
git commit -m "feat(message): message 表 model 列改为 model_id 存储业务 modelId，session 提取 model_ids"
```

---

### Task 5: session 读路径改造（dbmodel + domain + repo + query + dataset）

**Files:**
- Modify: `internal/infrastructure/database/model/session.go`
- Modify: `internal/domain/session/repository.go`
- Modify: `internal/infrastructure/repository/session_repository.go`
- Modify: `internal/application/session/port/handler.go`
- Modify: `internal/application/session/query/jwt_session_queries.go`
- Modify: `internal/application/session/query/session_message_list_query.go`
- Modify: `internal/application/session/query/option_list.go`
- Modify: `internal/application/dataset/port/handler.go`
- Modify: `internal/application/dataset/query/dataset_query.go`
- Modify: `internal/dto/session.go`（SessionSummary/MessageItem）
- Modify: `internal/dto/dataset.go`（query 参数）
- Modify: `internal/common/constant/sql.go`（SessionSummarySelect、SessionDistinct*、SessionFilterModelSQLColumn、SessionExportModelFilterSQL、FieldModelIDs）
- Test: `test/unit/session_option_list/`、`test/unit/session_service/`

**Interfaces:**
- Consumes: Task 4 的 `dbmodel.Session.ModelIDs`、`extractAssistantModelIDs`。
- Produces: `session.SessionSummaryProjection.ModelIDs`、`session.ExportFilter.ModelIDs`、`sessionport.SessionSummaryView.ModelIDs`、`sessionport.MessageView.ModelID`。

- [ ] **Step 1: dbmodel 与常量**

`internal/infrastructure/database/model/session.go`：

```go
ModelIDs []string `json:"model_ids" gorm:"column:model_ids;comment:回答模型ID列表;serializer:json"`
```

`internal/common/constant/sql.go`：

- 新增 `FieldModelIDs = "model_ids"`。
- `SessionSummarySelect`：`..., questions, models, ...` → `..., questions, model_ids, ...`。
- `SessionDistinctModelSelect = "DISTINCT jsonb_array_elements_text(model_ids::jsonb) AS model"`；`SessionDistinctModelWhere = "model_ids IS NOT NULL AND model_ids::jsonb <> '[]'::jsonb"`；`SessionDistinctModelLike = "jsonb_array_elements_text(model_ids::jsonb) ILIKE ?"`。
- `SessionFilterModelSQLColumn = "model_ids"`。
- `SessionExportModelFilterSQL = "model_ids::jsonb @> ?::jsonb"`。

- [ ] **Step 2: 领域投影**

`internal/domain/session/repository.go`：`SessionSummaryProjection.Models []string` → `ModelIDs []string`；`MessageDetailProjection.Model string` → `ModelID string`；`ExportFilter.Models` → `ModelIDs`；`ExportSessionRow.Models` → `ModelIDs`。

- [ ] **Step 3: repository**

`internal/infrastructure/repository/session_repository.go`：

- `SessionRow`（237 行附近）：`Models []string \`gorm:"column:models;serializer:json"\`` → `ModelIDs ... column:model_ids ...`。
- 投影赋值 `Models: row.Models` → `ModelIDs: row.ModelIDs`（约 300/365 行）。
- `MessageDetailProjection` 赋值 `Model: m.Model` → `ModelID: m.ModelID`（约 428/489 行）。
- `ExportSessionRow` 构造（588 行附近）：`Models: row.Models` → `ModelIDs`；`f.Models` → `f.ModelIDs`。
- `ListDistinctModels`：`Select(constant.SessionDistinctModelSelect)` 等不变（常量已改），返回名不变。

- [ ] **Step 4: session 应用层**

`internal/application/session/port/handler.go`：`SessionSummaryView.Models` → `ModelIDs`；`MessageView.Model` → `ModelID`；`MessageCacheRecord.Model` → `ModelID`。

`internal/application/session/query/jwt_session_queries.go`：`Models: p.Models` → `ModelIDs: p.ModelIDs`；`sessionFieldConfigs` 的 `SessionFilterModelSQLColumn` 引用不变（常量已改）。

`internal/application/session/query/session_message_list_query.go`：`Model: m.Model` / `rec.Model` → `ModelID: m.ModelID` / `rec.ModelID`。

`internal/application/session/query/option_list.go`：`SessionFilterFieldModel` 分支调 `ListDistinctModels` 不变。

- [ ] **Step 5: dataset 与 DTO**

`internal/application/dataset/port/handler.go`：`ExportParams.Models []string` → `ModelIDs []string`。

`internal/application/dataset/query/dataset_query.go`：`Models: p.Models`（3 处）→ `ModelIDs: p.ModelIDs`；`f := &session.ExportFilter{ ... Models: p.Models ...}` → `ModelIDs: p.ModelIDs`。

`internal/dto/dataset.go`（3 处）：`Models []string \`query:"models"...\`` → `ModelIDs []string \`query:"modelIds"...\``。

`internal/dto/session.go`：

```go
// SessionSummary
ModelIDs []string `json:"modelIds,omitempty" doc:"回答模型ID列表"`

// MessageItem
ModelID string `json:"modelId" doc:"业务模型ID"`
```

- [ ] **Step 6: 修复单测**

`test/unit/session_option_list/`、`test/unit/session_service/` 中引用 `Models` / `.Model` 字段处改为 `ModelIDs` / `ModelID`（session_service fixtures 若含 JSON `"models"` 字段，改 `"model_ids"`）。

- [ ] **Step 7: 运行验证**

Run: `go build ./... && go test -count=1 ./test/unit/session_option_list/ ./test/unit/session_service/ ./test/unit/session_detail_cache/`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/infrastructure/database/model/session.go internal/domain/session/ internal/infrastructure/repository/session_repository.go internal/application/session/ internal/application/dataset/ internal/common/constant/sql.go test/unit/session_option_list/ test/unit/session_service/ test/unit/session_detail_cache/
git commit -m "feat(session): sessions 表 models 列改为 model_ids 存储 modelId 列表"
```

---

### Task 6: 存量数据迁移（幂等，追加到 database migrate）

**Files:**
- Modify: `cmd/server/database.go`
- Test: 无（部署链路验证）

**Interfaces:**
- Consumes: Task 1-5 的列结构变更。
- Produces: `database.AutoMigrateAndBackfill(ctx)` 或等价函数。

- [ ] **Step 1: 实现迁移**

**执行顺序关键：必须先 Drop/Rename 旧列，再 AutoMigrate 建新列，最后回填**。因为旧库 `model_call_audits.model_id` 是 uint 类型，AutoMigrate 不会改已存在列的类型，若先 AutoMigrate 会导致新 string 写入失败。

`cmd/server/database.go` 中 `migrateDatabaseCmd.Run` 改为调用：

```go
func runMigrate(ctx context.Context) {
	lo.Must0(database.ManualMigrations(ctx))
	lo.Must0(database.AutoMigrate(ctx))
	lo.Must0(database.BackfillModelIDs(ctx))
}
```

在 `internal/infrastructure/database/postgresql.go` 新增三个函数：

```go
// ManualMigrations 执行 GORM AutoMigrate 无法覆盖的删列/改列，幂等可重入。
// 必须在 AutoMigrate 之前执行：旧 model_call_audits.model_id 为 uint，
// 先删掉它再 rename model→model_id，避免与 AutoMigrate 建新 text 列冲突。
func ManualMigrations(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	migrator := db.Migrator()

	// 2. model_call_audits：删旧 uint model_id 列 → rename model → model_id
	if migrator.HasColumn(&model.ModelCallAudit{}, "model") {
		if err := migrator.DropColumn(&model.ModelCallAudit{}, "model_id"); err != nil {
			return err
		}
		if err := migrator.RenameColumn(&model.ModelCallAudit{}, "model", "model_id"); err != nil {
			return err
		}
	}

	// 3. sessions：rename models → model_ids
	if migrator.HasColumn(&model.Session{}, "models") {
		if err := migrator.RenameColumn(&model.Session{}, "models", "model_ids"); err != nil {
			return err
		}
	}

	// 4. messages：rename model → model_id
	if migrator.HasColumn(&model.Message{}, "model") {
		if err := migrator.RenameColumn(&model.Message{}, "model", "model_id"); err != nil {
			return err
		}
	}
	return nil
}

// BackfillModelIDs 回填 models.model_id = alias，幂等（仅空值行）。
// 必须在 AutoMigrate 之后执行（依赖新列已存在）。
func BackfillModelIDs(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	if !db.Migrator().HasColumn(&model.Model{}, "model_id") {
		return nil
	}
	return db.Model(&model.Model{}).
		Where("model_id IS NULL OR model_id = ''").
		Update("model_id", gorm.Expr("alias")).Error
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./cmd/server`
Expected: PASS

- [ ] **Step 3: 本地联调验证（如无 PG 则跳过，标注待部署验证）**

Run: 部署链路 `docker-compose-single.yml` 的 `db-migrate` Job 或 K8s 部署后检查：

```bash
# 迁移后 SQL 检查（在具备 PG 环境执行）
SELECT column_name, data_type FROM information_schema.columns WHERE table_name='model_call_audits' ORDER BY ordinal_position;
# 期望：有 model_id(text)，无 model 列
```

- [ ] **Step 4: Commit**

```bash
git add cmd/server/database.go internal/infrastructure/database/postgresql.go
git commit -m "feat(migrate): 存量列改名/删列/回填幂等迁移"
```

---

### Task 7: E2E 测试

**Files:**
- Create: `test/e2e/model_id/model_id_test.go`
- Modify: 无

**Interfaces:**
- Consumes: 全部后端改动。
- Produces: 可回归验证用例。

- [ ] **Step 1: 编写 E2E**

参照 `test/e2e/model_capabilities/model_capabilities_test.go` 骨架（admin JWT → endpoint list → create model → list 断言 → PATCH → DELETE 清理；alias 用 UnixNano 后缀防冲突；huma unwrap 无 data 外层）：

- 创建 model：断言响应/列表 `modelId == alias`。
- PATCH 更新 `modelId`：断言列表 `modelId` 变为新值；空串更新返回校验错误。
- 通过 mock/真实上游触发一次 LLM 调用（若环境无上游则跳过调用断言，只验证 model CRUD 的 modelId 语义）。

测试文件核心断言片段：

```go
// create
resp, err := api.CreateModel(ctx, adminToken, dto.CreateModelReqBody{Alias: alias, ModelName: "upstream-x", EndpointID: epID})
// list 断言 item.ModelID == alias
// patch
_, err = api.UpdateModel(ctx, adminToken, id, dto.UpdateModelReqBody{ModelID: lo.ToPtr("custom-id")})
// list 断言 item.ModelID == "custom-id"
// 空串
_, err = api.UpdateModel(ctx, adminToken, id, dto.UpdateModelReqBody{ModelID: lo.ToPtr("")})
// 断言返回校验错误
```

（HTTP 调用封装参考同目录既有测试的 helper；无 env 时测试 SKIP，保持与既有 E2E 一致。）

- [ ] **Step 2: 运行验证**

Run: `go vet ./test/e2e/model_id/ && go build ./test/e2e/model_id/`
Expected: 编译通过（无 env 时 `go test -run TestModelID ./test/e2e/model_id/` 输出 SKIP）。

- [ ] **Step 3: Commit**

```bash
git add test/e2e/model_id/
git commit -m "test(e2e): modelId 默认/更新/空值校验 E2E"
```

---

### Task 8: 前端改造

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/app/(dashboard)/models/page.tsx`
- Modify: `web/src/app/(dashboard)/audit/model/page.tsx`
- Modify: `web/src/components/charts/model-trend-chart.tsx`、`model-token-bar-chart.tsx`（若引用 `item.model`）
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`（若展示 `models`）
- Modify: `web/src/app/(dashboard)/dataset/page.tsx`（导出 query 参数）
- Modify: `web/src/lib/api-client.ts`（若类型引用）与 i18n 文案（`web/src/i18n/` 或页面内 `t()` 键）

**Interfaces:**
- Consumes: 后端 API JSON 字段 `modelId` / `modelIds`。
- Produces: 前端展示与编辑 modelId。

- [ ] **Step 1: 类型定义**

`web/src/lib/types.ts` 按行号修改（Trace 相关 653/664 行**不动**）：

- L89 `SessionSummaryView.models?: string[]` → `modelIds?: string[]`。
- L124 `MessageItem.model: string` → `modelId: string`。
- L377 `AuditLogItem.model: string` → `modelId: string`。
- L411 `ModelTrendItem.model`、L428 `RequestRateItem.model`、L450 `TokenRateItem.model`、L463 `ModelUsageItem.model`、L480 `FirstTokenLatencyItem.model` → `modelId`。
- `ModelItem` 新增 `modelId: string;`；`UpdateModelReqBody` 新增 `modelId?: string;`。

- [ ] **Step 2: 模型管理页**

`web/src/app/(dashboard)/models/page.tsx`：

- 表单 state 新增 `modelId: ""`；创建请求**不发送** modelId（后端默认=alias）；编辑时 `openEdit` 填充 `modelId: model.modelId`，PATCH body 仅在非空时带 `modelId`。
- 列表表格新增一列展示 `model.modelId`（使用既有 i18n 键或新增 `models.model_id` 文案）。
- `handleToggleEnabled`、`getEndpointName` 等使用 `model.alias` 的展示逻辑不变。

- [ ] **Step 3: 审计页**

`web/src/app/(dashboard)/audit/model/page.tsx`：`log.model` → `log.modelId`（约 303/471 行与 ProviderIcon 传参）。filter 参数 `buildAuditFilter` 的 `model:${...}` 保留（API key 未变）。

- [ ] **Step 4: 图表、会话页与 dataset**

- 图表组件 `item.model` / `p.model` → `item.modelId` / `p.modelId`（对应 DTO 字段）。
- sessions 页面 `session.models` → `session.modelIds`（若展示）。
- `web/src/app/(dashboard)/dataset/page.tsx` L208：`models: selectedModels` → `modelIds: selectedModels`（后端 query 参数已改为 `modelIds`）。

- [ ] **Step 5: 运行验证**

Run: `cd web && npm run lint`
Expected: 无新增 error（存量 17 problems 基线内，仅 warning 可接受）。

Run: `cd web && npx tsc --noEmit`
Expected: 类型通过。

- [ ] **Step 6: Commit**

```bash
git add web/src/
git commit -m "feat(web): 模型管理/审计/会话页展示与编辑 modelId"
```

---

### Task 9: 全量验证与收尾

**Files:** 无新增

- [ ] **Step 1: 全量测试**

Run: `make test`
Expected: 全部 PASS（SKIP 的 E2E 除外）。

- [ ] **Step 2: lint**

Run: `make lint`
Expected: 无 error。

- [ ] **Step 3: 前端构建**

Run: `make web-build`（或 `cd web && npm run build`）
Expected: 构建成功。

- [ ] **Step 4: 自审 diff**

Run: `git diff master...HEAD --stat`
Expected: 改动仅涉及本计划列出的文件；无死代码/未使用变量（重点检查 `callOutcome.exposedModel`、`AuditLogView` 等残留引用）。

- [ ] **Step 5: 提交收尾**

```bash
git add -A
git commit -m "chore: 全量验证通过"
```

（如无遗留改动则跳过本步。）
