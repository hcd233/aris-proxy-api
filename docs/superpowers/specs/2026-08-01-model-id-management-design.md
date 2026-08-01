# Model ID 管理与冗余字段改造设计

> 日期：2026-08-01
> 状态：已确认
> 范围：后端 Go + Web 前端

## 背景与目标

Model 实体当前只有数据库主键 `id`、对外别名 `alias`、上游模型名 `model` 等字段，缺少稳定的业务模型 ID。session、model_call_audit、message 三张表冗余存储的模型相关字段（模型名/别名）语义不一，需要统一为「业务 model id」。

目标：
1. Model 实体新增业务字段 `modelId`，创建时默认值 = `alias`，用户可更新。
2. session / model_call_audit / message 三张表写入 model id 的值，相关列名改为 `model_id` / `model_ids`。
3. trace 表保留不动。

## 已确认的决策

| 决策点 | 结论 |
|--------|------|
| modelId 类型 | `string` |
| modelId 唯一性 | **不唯一** |
| modelId 空值 | 不允许为空（创建默认=alias；更新空串报校验错误） |
| model_call_audit 旧 `model_id`(uint 主键) 列 | 删除（读路径无任何使用） |
| model_call_audit 旧 `model`(alias) 列 | 改名为 `model_id`，存业务 modelId |
| 表范围 | model_call_audit、session、message；**trace 保留** |
| 前端 | 一并修改（模型管理页、审计页、会话页） |
| 存量迁移 | model 表回填 model_id=alias；audit 改名后存量天然正确；message/session 改名后保留旧值；迁移幂等 |

## 后端改动

### 1. 数据层（internal/infrastructure/database/model）

- `Model`：新增字段 `ModelID string`（`column:model_id`，非空，无唯一约束）。
- `ModelCallAudit`：删除 `ModelID uint`（旧 `column:model_id`）；`Model string` 改为 `ModelID string`（`column:model_id`，存业务 modelId）。
- `Session`：`Models []string`（`column:models`）→ `ModelIDs []string`（`column:model_ids`）。
- `Message`：`Model string`（`column:model`）→ `ModelID string`（`column:model_id`）。

### 2. 领域层

- `internal/domain/llmproxy/aggregate/model.go`：
  - 新增字段 `modelID string` 与 getter `ModelID()`。
  - `CreateModel`：内部默认 `modelID = alias`（不新增入参，创建时恒等于 alias）。
  - `Update`：新增 `modelID *string` 参数，非空校验。
- `internal/domain/modelcall/aggregate/audit.go`：
  - `RecordCallInput.ModelID` 类型 `uint` → `string`；删除 `Model` 字段与 `Model()` getter；`ModelID()` getter 语义变为业务 modelId。

### 3. 应用层

- `internal/application/model/port/handler.go`：
  - `CreateModelCommand` 不新增字段（modelId 默认=alias）。
  - `UpdateModelCommand` 新增 `ModelID *string`。
  - `ModelView` 新增 `ModelID string`。
- `internal/application/model/command/create_model.go`：无需改动（聚合内部默认 modelID=alias）。
- `internal/application/model/command/update_model.go`：透传 `cmd.ModelID`。
- `internal/application/llmproxy/usecase/recorder.go`：审计任务组装改为 `ModelID: out.model.ModelID()`，删除 `Model: out.exposedModel`。
- `internal/dto/asynctask.go`：`ModelCallAuditTask.ModelID` 类型 `uint` → `string`，删除 `Model` 字段；`MessageStoreTask.Model` → `ModelID`（string）。
- 审计查询（list_audit_logs、ListDistinctModels、QueryModelTrend、QueryRequestRate、QueryTokenThroughput、QueryFirstTokenLatency）：字段引用从 `model` 列改为 `model_id` 列，聚合 getter 相应调整。
- 消息存储投递点（openai chat / openai response / anthropic）：`MessageStoreTask` 改传解析聚合的业务 modelId。
- `internal/infrastructure/pool/store_pool.go`：
  - `message.ModelID = task.ModelID`（assistant 消息才写）。
  - `extractAssistantModels` → 从 message.ModelID 提取，改名 `extractAssistantModelIDs`，写 `session.ModelIDs`。
- checksum：`ComputeMessageChecksum` 的 model 参数改传 modelId（`messageChecksumWire.Model` 语义变化，新数据用新算法，存量无冲突）。

### 4. DTO / Handler

- `internal/dto/model.go`：
  - `UpdateModelReqBody` 新增 `ModelID *string`（`json:"modelId,omitempty"`，非空校验）。
  - `ModelItem` 新增 `ModelID string`（`json:"modelId"`）。
- `internal/handler/model.go`：Update 命令透传、列表响应透传 ModelID。

### 5. Session 读路径

- `internal/infrastructure/repository/session_repository.go`：`Models` → `ModelIDs`（含按模型筛选 SQL 片段）。
- `internal/application/session/query/*`：`Models` → `ModelIDs`。
- `internal/application/dataset/query/dataset_query.go`：`Models` → `ModelIDs`（含模型筛选条件）。

### 6. 存量迁移（追加到 `cmd/server/database.go` 的 migrate 流程）

在 `AutoMigrate` 之后执行幂等手动迁移（GORM Migrator + 列存在性检查）：

1. `models` 表：`UPDATE models SET model_id = alias WHERE model_id IS NULL OR model_id = ''`。
2. `model_call_audits` 表：若存在旧 uint `model_id` 列先 `DropColumn`；再 `RenameColumn model → model_id`。
3. `sessions` 表：`RenameColumn models → model_ids`。
4. `messages` 表：`RenameColumn model → model_id`。

## 前端改动（web/）

- `web/src/app/(dashboard)/models/page.tsx`：模型创建表单不展示 modelId（默认=alias）；编辑表单新增 modelId 输入；列表展示 modelId 列。
- 相关 API 类型定义（`web/src/lib/api` 或相应位置）同步新增/修改字段。
- `web/src/app/(dashboard)/audit/model/page.tsx`：模型展示/筛选字段改为 modelId。
- session 相关页面（列表/详情）：模型展示字段改为 modelIds。
- dataset 导出模型筛选：字段同步。

## 测试

- 单元测试（`test/unit/<topic>/`）：
  - 创建 Model 默认 modelId=alias；更新 modelId 生效；空串更新报 `ErrValidation`。
  - `recordModelCall` 审计任务携带业务 modelId。
  - 消息存储任务 modelId 落库与 session model_ids 提取去重。
- E2E（`test/e2e/model/`）：
  - admin JWT → 创建 endpoint → 创建 model（断言 modelId=alias）→ PATCH 更新 modelId → 触发 LLM 调用（如 mock 上游）→ 断言 audit model_id 与 session model_ids 写入新 modelId → DELETE 清理。
  - 参照既有 model E2E 骨架（UnixNano 后缀 alias 防唯一冲突；huma unwrap 响应无 data 外层）。

## 风险与注意

- GORM AutoMigrate 不能改列名/删列，必须走手动迁移；迁移需幂等（重复执行不报错）。
- `model_call_audits` 列改名顺序：先删旧 uint `model_id` 列，再 rename `model` → `model_id`，避免列名冲突。
- serializer:json 列（sessions.model_ids）保持 text 类型，勿用 map-update 传 raw slice（参考既有陷阱）。
- checksum 算法随 model 字段语义变化而变，属预期行为；存量数据不受影响。
