# 工具 Checksum 纳入 Description 与去重写入修复设计

## 背景

写入工具（Tool）的去重以 `check_sum` 为键，当前实现存在三个问题。

### 问题 1：checksum 不包含工具描述

`ComputeToolChecksum` 的 wire 结构只含 `Name` 与 `Parameters`：

```go
type toolChecksumWire struct {
	Name       string              `json:"name"`
	Parameters *JSONSchemaProperty `json:"parameters"`
}
```

工具级`Description` 不参与计算。同名同参数 Schema 但描述不同的工具（不同 Agent 客户端、不同版本对同一工具的 prompt 调优）会被去重成同一条记录，丢失描述差异。现有 fixture `same_name_different_description` 把该行为固化为 `expect_equal: true`，属于需要修正的既定语义。

### 问题 2：`tools.check_sum` 缺唯一约束

`dbmodel.Tool.CheckSum` 只有普通列定义，生产实测`tools` 表仅存在主键索引 `tools_pkey`，`check_sum` 上**没有任何索引**。对比 `messages` 表已有 `idx_message_checksum` 单列唯一索引。后果：

- 并发写入同一工具时，两个请求都"先查无→ 后插入"，产生重复内容记录；
- 同一批次内若出现两条相同 checksum 的工具（`deduplicateAndStoreTools` 缺 `lo.UniqBy`，而消息版有），会双双插入；
- 去重查询走全表扫描（当前 1052 行无感，但属于隐性债）。

### 问题 3：并发兜底逻辑失效，且会写入脏数据

`pool.deduplicateAndStoreTools` 与 `pool.deduplicateAndStoreMessages` 在 `BatchCreate` 失败后尝试"重新查询已存在记录"作为并发兜底，该逻辑有两处硬伤：

1. **在 PostgreSQL 事务中必然失败**：`store_pool.go` 用 `db.Transaction()` 包裹整个流程。PostgreSQL 中任一语句报错后事务进入 aborted 状态，后续语句一律返回 `current transaction is aborted, commands ignored until end of transaction block`。因此"冲突后在同一 `tx` 内重查"永远拿不到数据，需要 SAVEPOINT 才可能工作。
2. **吞掉错误并写入 0 值ID**：`_ = err` 丢弃原始错误。若失败原因不是唯一冲突（字段超长、连接中断等），`existingMap` 缺少对应 checksum，末尾 `existingMap[m.CheckSum]` 返回零值 **0**，最终 `session.message_ids` / `session.tool_ids` 写入 `[0, ...]` 脏数据，且事务照常提交。

`messages` 表已有唯一索引，因此问题 3 在消息写入路径上是**当前活跃的缺陷**，而非理论风险。

## 生产事实基线（2026-08-10 实测）

| 项 | 值 |
|---|---|
| `tools` 行数 | 1052 |
| `tools` distinct `check_sum` | 1052（零重复） |
| `tools` 软删除行（`deleted_at <> 0`） | 0 |
| `tools` 现有索引 | 仅 `tools_pkey` |
| `messages` 索引 | `idx_message_checksum` 单列 UNIQUE |
| `sessions` 行数 | 约2948 |
| tool 删除路径 | `HardDeleteByIDs`（`Unscoped().Delete`，真硬删，无软删态） |

零重复是本设计成立的关键前提：唯一索引可直接创建，无需先做数据清理。

## 目标

1. `ComputeToolChecksum` 纳入 `Description`。
2. `tools.check_sum` 建立复合唯一约束，使去重具备数据库层保证。
3. 消除去重写入路径的吞错与 0 值 ID 缺陷，使并发冲突成为可正确处理的正常路径。
4. 回填存量 tool 的 `check_sum` 至新算法，使存量记录继续参与去重而非变成孤儿。

## 关键决策

| 决策点 | 选择 | 理由 |
|---|---|---|
| 存量 checksum 处理 | **回填重算** | 存量记录继续参与去重，避免同一工具新旧两份长期共存。 |
| 唯一索引范围 | **复合 `(check_sum, deleted_at)`** | 与 `models`/`endpoints`/`users` 的项目惯例一致；语义为"活跃行内 checksum 唯一"，与去重查询的 `deleted_at = 0` 条件对齐；为未来引入 tool 软删留出空间。 |
| 吞错缺陷修复范围 | **tools + messages 两条路径** | 两者是同一份逻辑的孪生实现，且 messages 路径缺陷当前活跃。 |
| 写入冲突处理方式 | **`ON CONFLICT DO NOTHING` + 补查 + 严格校验** | 冲突不报错即不会使事务 aborted，无需 SAVEPOINT；项目已有此惯例（`trace_repository.go`）。|

## 回填的正确性论证：一对一 UPDATE，无需 remap

回填最令人担心的风险是"多条旧记录重算后撞成同一新 checksum，需要合并记录并 remap 所有 `session.tool_ids`"。该风险在本场景下**不存在**，证明如下：

- 新算法的输入 `(name, description, parameters)` 是旧算法输入 `(name, parameters)` 的**超集**；
- 存量 1052 条 checksum 两两不同 ⟹ 各条的 `(name, parameters)` 两两不同 ⟹ 各条的 `(name, description, parameters)` 必然也两两不同；
- ⟹ 重算后仍得到 1052 个互不相同的 checksum（排除 SHA256碰撞）。

即新 checksum 的等价类只会是旧等价类的细分，而旧等价类已全是单元素集。因此回填是**一对一 UPDATE**：零合并、零 remap、零悬空 ID，`sessions.tool_ids` 无需触碰。

## 后端实现

### 1. `internal/common/vo/checksum.go`

wire 结构新增 `Description`，`ComputeToolChecksum` 填充该字段，其余不变（仍用 `encoder.Encode(..., SortMapKeys)` + SHA256 保证 map key 顺序稳定）：

```go
type toolChecksumWire struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  *JSONSchemaProperty `json:"parameters"`
}
```

### 2. `internal/infrastructure/database/model/tool.go`

`CheckSum` 挂复合唯一索引 priority:1，并按 `model.go`/`endpoint.go` 的既有做法在 `Tool` 中重声明 `DeletedAt` 以挂 priority:2（`DeletedAt` 原本继承自 `BaseModel`，重声明仅为附加索引 tag，避免污染其他继承 `BaseModel` 的表）：

```go
type Tool struct {
	BaseModel
	ID        uint            `gorm:"column:id;primary_key;auto_increment;comment:工具ID"`
	Tool      *vo.UnifiedTool `gorm:"column:tool;not null;comment:工具;serializer:json"`
	CheckSum  string          `gorm:"column:check_sum;not null;default:'';uniqueIndex:idx_tool_checksum_deleted,priority:1;comment:校验和"`
	DeletedAt int64           `gorm:"column:deleted_at;default:0;uniqueIndex:idx_tool_checksum_deleted,priority:2;comment:删除时间"`
}
```

索引由 `database migrate` 的 `AutoMigrate` 创建。存量零重复，创建必然成功；1052 行的索引创建锁表时间在毫秒级。

### 3. 去重写入统一模式（四处）

涉及 `pool.deduplicateAndStoreTools`、`pool.deduplicateAndStoreMessages`、`toolRepository.BatchSaveDedup`、`messageRepository.BatchSaveDedup`。统一为：

1. 按 checksum 批量 `IN` 查询已存在记录，构建 `checksum → ID` 映射；
2. 过滤出映射中不存在的记录，并对待插入集合做 `lo.UniqBy`（消除批次内自冲突）；
3. 插入时挂 `clause.OnConflict{DoNothing: true}`——冲突行被跳过而不报错，**事务不进入 aborted 状态**；
4. 成功插入的行由 GORM 回填 ID；被跳过的行 ID 保持 0，对这部分 checksum 补查一次以补齐映射；
5. 按输入顺序取ID，**任一 checksum 缺失或映射到 0 即返回错误**，让事务回滚，绝不写入 0 值 ID。

冲突目标列按各表实际索引形态填写，两处不对称并加注释说明：

- tools：`Columns: []clause.Column{{Name: check_sum}, {Name: deleted_at}}`
- messages：`Columns: []clause.Column{{Name: check_sum}}`

`BatchCreate(db, data)` 接收 `db` 参数，可直接传入 `tx.Clauses(...)`，无需改动 `baseDAO`。

错误路径不再使用 `_ = err`：`BatchCreate` 返回的非冲突错误直接向上返回。

### 4. `BackfillToolChecksums`

新增于 `internal/infrastructure/database/postgresql.go`，由 `cmd/server/database.go` 的 `runMigrate` 在 `AutoMigrate` 之后调用：

```go
func runMigrate(ctx context.Context) {
	lo.Must0(database.AutoMigrate(ctx))
	lo.Must0(database.BackfillToolChecksums(ctx))
}
```

实现要点：

- 用 `FindInBatches` 分批扫描 `tools` 全表（按主键分页，更新 `check_sum` 不影响游标）；
- 每行反序列化 `tool` 列后调`vo.ComputeToolChecksum` 重算；
- **幂等**：重算值等于现值则跳过不UPDATE，因此可安全重复执行；
- 逐行独立 UPDATE（不包裹在单一大事务内），使单行唯一冲突不会中断整批；
- 冲突识别：`gorm.Config.TranslateError` 已开启，用 `errors.Is(err, gorm.ErrDuplicatedKey)` 精确区分。**冲突则计数并warn 日志后跳过**（保留该行旧 checksum，成为无害孤儿）；**非冲突错误直接返回中断**，不静默忽略；
- 结束输出统计：total / unchanged / updated / conflict。

冲突分支仅在"新版本已上线写入新 checksum 之后才执行回填"的错序场景触发，保留它使迁移在错序下也不会失败。

## 部署编排

**必须先执行迁移，再滚动部署新版本。**

1. 用新镜像执行一次性 `database migrate`（此时线上仍是旧版本）：建复合唯一索引 + 回填存量 checksum。存量零重复且线上旧版本仍用旧算法写入，回填零冲突。
2. 滚动部署新版本（`replicas: 2`，`maxUnavailable: 0`，`maxSurge: 1`）。

窗口期行为：步骤 1 与 2 之间旧版本按旧算法写入，算出的旧 checksum 与已回填的新 checksum 值不同，不触发唯一冲突，仅可能产生少量重复内容的孤儿记录——不影响功能，其关联 session 被删除时由 `SoftDeletePurgeCron` 清理。

滚动期间新旧版本共存、两种算法并行写入，同理不冲突，无需停机。

**反序（先部署新版本再迁移）会产生真实冲突**：新版本会先插入新算法 checksum 的记录，回填存量同一工具时撞车。该情况由回填的冲突跳过分支兜住，不会中断，但会留下未回填的旧记录。

## 测试计划

1. **`test/unit/tool_checksum/fixtures/cases.json`** 修正三个受description 语义变更影响的用例：
   - `same_name_different_description`：`expect_equal` 改为 `false`，描述改为"工具级 description 参与 checksum"；
   - `different_provider_same_schema`、`many_params_same_tool`：两侧 `description` 统一，以保留这两个用例"provider 不参与 checksum"的原始意图，避免与 description 变更混淆断言含义。
2. **`test/unit/db_index/db_index_test.go`** 扩展：对 `dbmodel.Tool{}` 执行 AutoMigrate，断言 `idx_tool_checksum_deleted` 的列组合为 `["check_sum", "deleted_at"]` 且为唯一索引。
3. **回填单测**（sqlite 内存库，遵循 `test/unit/message_dedup` 的建库方式）：
   - 存量旧 checksum 记录回填后 checksum 等于新算法值，行数不变（一对一）；
   -幂等：连续执行两次，第二次 updated 计数为 0；
   - 冲突跳过：预置一条已是新 checksum 的记录构造冲突，断言回填不报错、冲突计数为 1、旧记录保持原值。
4. **去重写入单测**：批次内重复 checksum 只插入一条且返回的 ID 列表与输入顺序对齐；映射缺失时返回错误而非0 值 ID。
5. 回归`go test ./test/...`、`make lint`、`make lint-conv`。

## 兼容性与影响

- `ComputeToolChecksum` 签名不变，调用方无需改动。
- 回填后存量 tool 的 `check_sum` 值发生变化；该列不对外暴露、不被任何 API 或前端引用，`sessions.tool_ids` 引用的是 `tools.id`（不变），因此无外部兼容性影响。
- 新增数据库唯一约束会使"同一活跃 checksum 插入第二次"从静默成功变为被 `ON CONFLICT` 跳过，这是预期的去重收敛。
- 已知遗留债（本次不处理）：`pool` 与 `repository` 两套去重实现长期重复（注释自称"字节级一致"），根因是 `pool` 需在外部 `tx` 内工作而 repository 自持 `db`。统一需让仓储支持注入事务，属独立重构范围。
