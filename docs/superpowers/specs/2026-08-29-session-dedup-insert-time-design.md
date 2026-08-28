# Session 前缀去重实时化 + Terminal 终态清理独立 cron 设计

> 日期：2026-08-29
> 状态：待评审
> 背景：现由每小时 `SessionDeduplicateCron`（`internal/cron/session_dedup.go`）全表加载 sessions 做前缀去重 + terminal tool_call 清理。决策：前缀去重改为每次 session 插入时实时执行；terminal 清理保留为独立 cron，只扫描最近一天的终态会话。

## 1. 目标与语义变化

- 冗余前缀快照（同一对话第 k 轮是第 k+1 轮 MessageIDs 前缀）在插入时即刻清理，不再等待最多 1 小时的 cron 周期。sessions 列表、消息数分桶统计不再出现重复快照。
- terminal tool_call 清理（末条消息为 assistant 且 tool_calls 非空、对话中断于工具调用处的会话）保留 cron 驱动，扫描窗口收窄为最近 24 小时。
- 用户请求时延不受影响：全部新增逻辑运行在既有异步协程池内。

## 2. 方案总览

三个组成部分：

1. **共享前缀去重逻辑提取**：算法从 cron 包搬到 `internal/infrastructure/repository/session_dedup.go`（package repository），terminal 相关逻辑删除。
2. **插入时实时去重**：`runMessageStoreTask`（`internal/infrastructure/pool/store_pool.go`）事务提交后，在同一协程池 goroutine 内以独立短事务执行前缀去重。
3. **Terminal 终态清理 cron**：`SessionDeduplicateCron` 重命名为 `SessionTerminalCleanupCron`，逻辑重写为 24 小时窗口扫描 + 删除。

## 3. 共享前缀去重逻辑提取

从 `internal/cron/session_dedup.go` 搬入 `internal/infrastructure/repository/session_dedup.go`（package repository，已具备 gorm/dao/dbmodel/sonic 全部依赖，pool 与 cron 引用均无循环）：

- 保留：`MergeResult`、`sessionEntry`、`groupByFirstMessageID`、`resolveGroup`、`isPrefix`、`mergeToolIDsIntoMapping`、`FindRedundantSessions`、`ApplyMergeResult`
- 签名简化：`FindRedundantSessions(sessions []*dbmodel.Session) MergeResult`——移除 `terminalMsgIDs` 参数，`applyTerminalRule` 与 absorbed 保护集整体删除（终态清理改由 cron 的简单扫描承担，不再依赖组内前缀解析）
- `ApplyMergeResult(db *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult)` 保持现签名：事务内 `sonic.MarshalString` + `Updates(map)` 写回 tool_ids（GORM `Updates(map)` 不触发 `serializer:json`，此写法必须单点保留）+ `BatchDeleteByField` 软删冗余

选 repository 而非 application/domain：application 层现有约定不 import infrastructure/database；repository 层本就承担基于 dao 的写路径。

## 4. 插入时实时去重

### 4.1 触发与执行模型（异步）

`runMessageStoreTask` 现有主事务（messages → upgrade reasoning → tools → session Create）**保持不变**。事务成功提交后，在同一 store 协程池 goroutine 内继续执行：

1. 开启独立短事务；
2. `SELECT ... WHERE deleted_at = 0 AND message_ids::jsonb->>0 = '<firstMsgID>' FOR UPDATE`（新增 `SessionDAO.FindGroupForUpdate`，走表达式索引）——取出同组全部已提交会话，行锁串行化同组并发写入；主事务已提交，组查询结果天然包含刚插入的新 session，无需额外拼接；
3. 组查询结果直接喂给 `FindRedundantSessions`（纯前缀规则）；
4. 有冗余则 `ApplyMergeResult`：keeper 合并 ToolIDs、冗余成员软删。重复请求（MessageIDs 完全相同）时新 session 自身判冗余、插入后即被软删；
5. 短事务提交。

### 4.2 并发安全

- 同组并发插入（协程池多 goroutine、多副本）：第 2 步 `FOR UPDATE` 在 PG 层串行化，后到者看到先到者已提交的行，去重结果完整。
- 不引入 Redis 锁：行锁等待为毫秒级，组内工作量极小。
- 空组竞态（两个并发首插都看到空组）：各留一份，下次同组插入自愈。接受。

### 4.3 失败语义

主事务提交后数据已安全；去重短事务任一步失败仅记错误日志，不重试、不回滚主数据。残留重复由该对话下次插入时的去重自愈。接受两个已知边界：

- 提交到去重完成之间，冗余快照短暂可见（毫秒级窗口）；
- 去重执行前进程崩溃，该组冗余残留至下次同组插入（无 cron 前缀兜底后不再有全局扫描）。

### 4.4 成本

每次请求新增一条走索引的组查询（正常情况下组内仅新 session 自身，算法即时返回）。终态判定不前移——插入时只做前缀规则，不查 messages。

## 5. Terminal 终态清理 cron

### 5.1 重命名

| 项 | 旧 | 新 |
|---|---|---|
| struct/文件 | `SessionDeduplicateCron` / `session_dedup.go` | `SessionTerminalCleanupCron` / `session_terminal_cleanup.go` |
| 模块常量 | `CronModuleSessionDeduplicate` | `CronModuleSessionTerminalCleanup`（值 `"SessionTerminalCleanupCron"`） |
| spec 常量 | `CronSpecSessionDeduplicate` | `CronSpecSessionTerminalCleanup`（保持 `"0 * * * *"` 每小时） |
| 描述常量 | `CronDescriptionSessionDeduplicate` | `"Scan sessions created in the last 24h interrupted at an assistant tool_call and remove them"` |
| 配置键 | `cron.session.deduplicate.enabled` | `cron.session.terminal_cleanup.enabled`（`config.go` 的 `SetDefault` 同步改，默认 true） |
| config 字段 | `CronSessionDeduplicateEnabled` | `CronSessionTerminalCleanupEnabled` |

前端 cron 页面名称/描述/spec 由接口动态下发，无需改动。旧模块名的 cron 审计记录保留为历史。已核实 `api.env` 不含任何 `cron.*` 键（启用值仅来自 `SetDefault`），**改名不涉及生产 api.env / ConfigMap 变更**。

### 5.2 逻辑

每小时执行（`TriggerWithLock` 与手动触发 `Trigger()` 机制保留）：

1. `SessionDAO` 新增 `FindCreatedSince(db, since)`：`WHERE deleted_at = 0 AND created_at >= since`，只取 `id, message_ids`（`SessionRepoFieldsDedup` 收窄），`since = now - 24h`，走新增的 created_at 索引；
2. 取每个 session 的末条 message ID，复用 `MessageDAO.FilterTerminalToolCallIDs` 下推 SQL 判定 assistant+tool_calls（现有方法，单次约 1 KB IO）；
3. 命中的 session 经 `BatchDeleteByField` 软删。

不再全表加载、不再组内前缀解析、不再合并 ToolIDs。语义依据：现行算法中 absorbed 保护仅使 keeper 多存活一个 cron 周期（下一轮成单例组后仍被删除），稳态行为即"中断会话最终被清理"；新逻辑将一个周期的延迟归零，稳态等价。被删会话的 messages/tools 若不再被引用，由既有 soft_delete_purge cron 按保留期处理。

边界：cron 停机超过 24h 期间产生的中断会话不在扫描窗口内，残留至同组下次插入。接受。

## 6. 数据库迁移（生产环境）

| 迁移项 | 内容 | 方式 |
|---|---|---|
| 表达式索引 | `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_first_msg ON sessions ((message_ids::jsonb->>0)) WHERE deleted_at = 0;` | GORM tag 无法表达表达式索引（ThinkExtract trigram 先例），**生产手工执行**（login-prod-server）；DAO 查询表达式须与索引表达式逐字一致以命中 |
| sessions.created_at 索引 | Session 模型内重声明 `CreatedAt`（覆盖 BaseModel）挂 `index:idx_sessions_created_at`，覆盖 24h 窗口扫描与列表默认排序 | 普通 GORM tag，随 `database migrate` job（AutoMigrate）自动建，无手工步骤 |

上线顺序：**先在生产建表达式索引，再推 master 部署代码**——避免部署后每请求组查询退化为全表 JSON 扫描。本地/测试环境无索引时查询仍正确（小数据量全表扫无感）。

交接：合并前通过现成手动触发入口 `Trigger()` 跑一次现行去重，保证交接起点干净（生产当前每小时已在跑，残留 ≤1h）。

## 7. 测试计划

- `test/unit/session_dedup` 表驱动用例整体迁移到新包路径，fixtures 不动；删除 `terminal_msg_ids` 相关用例（`apply_terminal` 类），其余前缀/分叉/保护用例一一对应保留；
- `ApplyMergeResult` 写回行为测试保持；
- 新增 SQL 形态护栏测试（仿 `session_keyword_filter` 护栏风格）：`FindGroupForUpdate` 必须含 `::jsonb->>0`、`FOR UPDATE`、`deleted_at = 0`；`FindCreatedSince` 必须含 `created_at >=` 与列收窄；
- store_pool 去重流程单测：keeper 为新 session 的合并路径、新 session 自身判冗余被软删路径、去重失败不影响已提交数据的路径；
- 部署后跑 `test/e2e/` 会话相关用例验证（连续两轮请求后列表仅保留最新快照）。

## 8. 明确不做（YAGNI）

- 不给插入时去重加开关配置（失败即退化为现状行为，无需开关）；
- 不给组查询加 Redis 分布式锁（行锁已足够）；
- 不动 soft_delete_purge、think_extract 等其他 cron；
- 不迁移旧模块名的历史审计数据。

## 9. 涉及文件清单

| 动作 | 文件 |
|---|---|
| 新增 | `internal/infrastructure/repository/session_dedup.go`（算法 + ApplyMergeResult） |
| 重写 | `internal/cron/session_terminal_cleanup.go`（由 `session_dedup.go` 改名重写，仅保留 cron 壳 + 终态扫描） |
| 修改 | `internal/infrastructure/pool/store_pool.go`（提交后追加去重步骤） |
| 修改 | `internal/infrastructure/database/dao/session.go`（`FindGroupForUpdate`、`FindCreatedSince`） |
| 修改 | `internal/infrastructure/database/model/session.go`（重声明 CreatedAt 挂索引） |
| 修改 | `internal/common/constant/{cron,session}.go`、`internal/config/config.go`（重命名） |
| 迁移 | `test/unit/session_dedup/**`（包路径与用例调整） |
| 新增 | `test/unit/` 组查询 SQL 护栏、store_pool 去重流程用例；`test/e2e/<topic>/` 部署验证用例 |
| 文档 | `CONTEXT.md` 补充「前缀去重实时化」「终态清理」词条 |
