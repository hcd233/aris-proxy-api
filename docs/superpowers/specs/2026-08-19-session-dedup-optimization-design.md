# Session 去重算法优化设计文档

> 日期：2026-08-19
> 状态：设计已确认，待实现
> 拆分：PR1（IO 下推与写回原子性）→ PR2（分组与前缀语义统一）

## 1. 背景与目标

`internal/cron/session_dedup.go`（583 行）每小时（`0 * * * *`）全量扫描 sessions，清理 `MessageIDs` 被其他 session 包含的冗余 session。当前实现存在三类问题：

- **性能**：`FindRedundantSessionsWithMerge` 排序后 O(N²) 两两比较，每次比较是 O(L·M) 滑动窗口 `IsSubArray`，最坏 O(N²·L²)；`FindTerminalToolCallSessions` → `findParentSessionID` 又是第二轮 O(N²·L²)。函数注释声称"构建首元素索引加速查找"，但代码中并无该索引。
- **资源**：terminal tool call 检查一次性加载全部候选 session 的最后一条消息的**完整 JSON**。
- **可维护性**：两段独立规则通过 `terminalExcludeIDs` 相互打补丁，保护条件依赖 `MergeMapping` 的 key，语义不清。

**目标**：在**保持去重结果不变**的前提下，降低耗时与峰值内存，并将两段规则统一为单一决策。

**非目标**：不改 session 写入路径，不改 cron 调度，不引入 DB schema 变更。

## 2. 生产数据基线（2026-08-18/19 实测）

所有决策基于生产库只读实测，非估算。

### 2.1 规模

| 指标 | 值 |
|---|---|
| sessions 总行 / 活跃 / 已软删 | 3283 / **2551** / 732 |
| messages 表 | ~122,000 行 |
| `message_ids` 长度 | min 2 / p50 21 / avg 60.4 / p95 247 / max 752 |
| 总 message 引用数 | 154,193 → 仅 **1326 kB** |

### 2.2 分组收益（按 `message_ids[0]` 分组）

| 指标 | 值 |
|---|---|
| 组数 | 913 |
| 单例组（不可能有冗余） | **603** |
| 最大组 | 338 |
| 平均组大小 | 2.79 |
| Σgi² vs N² | **145,503 vs 6,507,601 → 44.7x** |

### 2.3 实测耗时（`cron_call_audits`，1514 次运行）

| cron | avg_ms | max_ms |
|---|---|---|
| ThinkExtractCron | 2496 | 6370 |
| **SessionDeduplicateCron** | **1586** | **4059** |
| SoftDeletePurgeCron | 1549 | 2791 |
| TriggerHitSyncCron | 21 | 130 |
| BlockedHitSyncCron | 20 | 361 |

`deduped_sessions_count` 每次 0~94，多数为 0。

### 2.4 真正的瓶颈：terminal 检查的 IO

```
rows_fetched | total_message_json | avg_message_json | max_message_json
        2080 | 5063 kB            | 2492 bytes       | 98 kB

last_msgs | role_assistant | assistant_with_tool_calls
     2080 |           2079 |                        30
```

**为了找出 30 个信号，加载并反序列化了 2080 行 / 5063 kB 完整对话 JSON**，是 session 自身扫描量（1326 kB）的 3.8 倍。这是耗时与峰值内存的主要来源，而非算法复杂度。

### 2.5 语义等价性：子数组语义从未生效

`store_pool.go:68-101` 决定了冗余的形态：每次请求把完整对话历史按 checksum 去重存 message，再**新建**一条 session。历史消息 checksum 不变即复用同一行 ID，故第 k 轮的 `messageIDs` **必然是**第 k+1 轮的**前缀**。

在全量生产数据上精确对比两种语义判定出的冗余集合：

```
redundant_subarray_semantics | redundant_prefix_semantics | lost_if_narrowed | gained_if_narrowed
                          31 |                         31 |                0 |                  0
```

「非前缀型连续子数组」关系在生产数据中**一例都不存在**（采样查询返回 0 行）。O(N²·L²) 的子数组算法从未产出过前缀算法产不出的结果。

**结论**：改为前缀判定不是"收窄语义换性能"，而是**删除从未生效的代码路径**。

## 3. 决策记录

| # | 决策点 | 结论 |
|---|---|---|
| 1 | 优化目标 | 可维护性 + 内存 + 性能（正确性非痛点，需保持结果不变） |
| 2 | 去重语义 | 收窄为前缀判定；已由生产数据证明与现状零差异 |
| 3 | 实施方式 | 三管齐下（SQL 下推 + 分组 + 语义统一），拆两个 PR |
| 4 | 已发现缺陷 | 两个都修（`MergeMapping` 覆盖丢失、写回非原子） |
| 5 | 保留者保护条件 | **选项 X**：改为「是否吸收了至少一个冗余成员」（真正的 merge target），而非现状的「是否落进 `MergeMapping`」 |
| 6 | 是否引入 trie | 不引入（YAGNI，见 4.2.3） |

## 4. 设计

### 4.1 PR1：IO 下推与写回原子性

不碰去重算法。

#### 4.1.1 terminal tool call 判定下推 SQL

新增常量，对齐 `DBJSONConditionAssistantRole` 既有风格（`internal/common/constant/database.go`）：

```go
DBJSONConditionHasToolCalls = "jsonb_typeof((message::jsonb)->'tool_calls') = 'array' AND jsonb_array_length((message::jsonb)->'tool_calls') > 0"
```

`jsonb_typeof` 前置守卫是必需的：`tool_calls` 键缺失时 `jsonb_array_length(NULL)` 返回 NULL 尚可，但若该键为非数组类型会直接报错。

新增 DAO 方法（与 `messageRepository.FindThinkExtractCandidates` 同构，只 select ID）：

```go
// internal/infrastructure/database/dao/message.go
func (dao *MessageDAO) FilterTerminalToolCallIDs(db *gorm.DB, ids []uint) ([]uint, error)
```

`loadLastMessagesForTerminalToolCheck` 相应改为只返回命中的 message ID 集合，不再返回 `[]*dbmodel.Message`。

#### 4.1.2 纯函数签名：DB 判定 / 纯函数决策

```go
// 旧：需要 messages 全量 payload，函数内自行判 role/tool_calls
func FindTerminalToolCallSessions(sessions []*dbmodel.Session, messages []*dbmodel.Message, excludeIDs []uint) MergeResult
// 新：SQL 负责筛选，纯函数只负责决策
func FindTerminalToolCallSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint, excludeIDs []uint) MergeResult
```

函数内 `mo.TupleToOption(...).FlatMap(...)` 的 nil/role/tool_calls 检查随之删除（DB 已保证）。函数仍不依赖 DB，可完整单测。

> 该签名是过渡态，PR2 会与 `FindRedundantSessionsWithMerge` 合并为单一入口。

#### 4.1.3 缺陷修复 A：`MergeMapping` 覆盖导致 ToolIDs 丢失

`processTerminalToolCallSession` 中的直接赋值，在同一 parent 有多个 terminal 子 session 时会覆盖前一个的结果：

```go
// 现状
result.MergeMapping[parentID] = parentToolIDSet
// 改为复用既有 helper
result.MergeMapping[parentID] = mergeToolIDs(result.MergeMapping[parentID], incoming)
```

#### 4.1.4 缺陷修复 B：写回原子性

现状 `deduplicate` 中 N 次 `tool_ids` UPDATE 无事务，某条失败被 `continue` 吞掉后 **`BatchDeleteByField` 照样执行** → 冗余 session 已删、ToolIDs 引用永久丢失。改为：

```go
err := db.Transaction(func(tx *gorm.DB) error {
    // 所有 tool_ids UPDATE，任一失败直接 return err
    // BatchDeleteByField 删除冗余 session
})
```

失败即整体回滚，下个整点重跑（任务天然幂等）。生产实测 `MergeMapping` 规模 ≤ 31，事务窗口很短。

> 注意不要在同一事务中依赖"失败后继续查询"——PostgreSQL 中任一语句报错即令事务进入 aborted 状态，后续语句全被拒。此处设计为失败即回滚，不涉及该陷阱。

#### 4.1.5 PR1 预期收益

| 指标 | 当前 | PR1 后 |
|---|---|---|
| messages 查询 | 2080 行 / 5063 kB | **30 行 / ~1 KB** |
| `UnifiedMessage` 反序列化 | 2080 次 | 0 次 |
| `duration_ms` avg | 1586 | 400~600 |
| 写回失败时 | 丢 tool 引用 | 整体回滚 |

### 4.2 PR2：分组与前缀语义统一

#### 4.2.1 对话组

按 `messageIDs[0]` 分组，一组 = 同一对话的所有快照：

```go
groups := map[uint][]sessionEntry{} // key = messageIDs[0]
```

空 `messageIDs` 的 session 继续跳过（现状行为，生产上为 0 条）。

#### 4.2.2 组内决策（单一规则）

组内按 `(len desc, id asc)` 排序后线性扫描，维护保留者列表：

```
for e in entries:                    # 长 → 短
    if ∃ k ∈ keepers, e 是 k 的前缀:  e 冗余，ToolIDs 并入首个匹配的 k
    else:                            e 成为新 keeper
```

- 保留者列表天然处理**对话分叉**：`[1,2]` 被 `[1,2,3]` 与 `[1,2,4]` 共享时，前者冗余、后两者各自保留。与现算法「未被吸收者成为新 longer」语义一致。
- 前缀判定：`len` 做 O(1) 剪枝 + `slices.Equal(k[:len(e)], e)`。
- `keepers` 按加入顺序（即 `len desc, id asc`）遍历，故「首个匹配的 k」= 最长且 `id` 最小的容器，与现算法外层循环命中的 longer 相同。
- 长度相同且内容相同：按 `id asc` 排序保证保留较早者，与现状一致。

**terminal 规则（选项 X）**：保护条件由现状的「是否落进 `MergeMapping`」改为「**是否吸收了至少一个冗余成员**」（即是否为真正的 merge target），与 `tool_ids` 是否为空无关。未吸收任何冗余的 session（含分叉产生的 keeper、单例组成员）仍适用 terminal 规则：末条 message ∈ `terminalMsgIDs` 即判定冗余，ToolIDs 无处合并故丢弃（现状行为）。

现状的保护条件依赖 `mergeToolIDsIntoMapping`，而它在双方 `tool_ids` 全空时直接 return，导致漏保护：

| 组的情况 | 现状是否受保护 | 选项 X | 差异 |
|---|---|---|---|
| merge target + 有 tool_ids | 是（在 `MergeMapping` 的 key 中） | 是 | 无 |
| merge target + 全无 tool_ids | **否** → 末条为 tool_calls 时被删，且它是最长者无处合并 → **整组消失** | 是 | **修复漏洞** |
| 分叉 keeper（未吸收冗余） | 否 → 被删 | 否 → 被删 | 无 |
| 单例组成员 | 否 → 被删（合理：孤立的中断分支） | 否 → 被删 | 无 |

即选项 X 唯一改变的是表中第二行——把 commit 37c53864（#85）的修复补完整。#85 想保护 merge target，却用 `MergeMapping` 的 key 作代理，于是漏掉了 `tool_ids` 全空的 merge target。其余三种情况与现状严格等价。

反向的选项 Y（严格删除所有末条为 tool_calls 的 session，含 merge target）会让有 tool 的组也整组消失并丢 tool 引用，等于回退 #85，已否决。

#### 4.2.3 明确不引入 trie

最大组 338 贡献 114,244 次比较（占 Σgi² 的 78%），约 685 万次 uint 比较 ≈ 5ms。trie 可降到 O(Σlen)，但引入额外结构与内存，收益不可观测。YAGNI。

#### 4.2.4 删除的代码

`IsSubArray`、`isSessionRedundant`、`isEqualSlice`、`processEntryAgainstShorter`、`findParentSessionID`，`deduplicate` 中拼 `terminalExcludeIDs` 的 7 行；`mergeToolIDsIntoMapping` 与 `mergeToolIDs` 合一。两段入口合并为：

```go
func FindRedundantSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint) MergeResult
```

583 行预计缩到 300 行内。

#### 4.2.5 PR2 预期收益

| 指标 | 当前 | PR2 后 |
|---|---|---|
| 比较次数 | 6,507,601 | **145,503**（44.7x） |
| `duration_ms` avg（叠加 PR1） | 1586 | 150~250 |
| `session_dedup.go` 行数 | 583 | < 300 |

## 5. 测试策略

### 5.1 单测

conv lint 禁止 `internal/` 下存在 `_test.go`，测试继续放 `test/unit/session_dedup/`，沿用 fixture 驱动范式。

fixture 变更：

| 文件 | 变更 |
|---|---|
| `is_sub_array_cases.json` | **删除**（13 个 case，随 `IsSubArray` 移除） |
| `find_redundant_sessions_cases.json` | `tail_subarray`、`middle_subarray` 期望 `[2]` → `[]`；其余 10 个 case 期望不变 |
| `terminal_tool_call_cases.json` | 入参由 `messages` 改为 `terminal_msg_ids` |

新增 case：

- `forked_conversation`：`[1,2]` / `[1,2,3]` / `[1,2,4]` → 仅 `[1,2]` 冗余，且其 ToolIDs 并入 `[1,2,3]`（首个匹配的 keeper）
- `group_keeper_protected_without_tools`：多成员组、`tool_ids` 全空、merge target 末条为 tool_calls → merge target **不删**（选项 X 修复漏洞的回归锁）
- `forked_keeper_not_protected`：分叉 keeper 未吸收任何冗余、末条为 tool_calls → **仍删除**（锁住"不过度保护"，与现状等价）
- `singleton_terminal_tool_call`：单成员组 + 末条 tool_calls → 删除
- `cross_group_not_redundant`：不同 `first_msg` 的 session 互不判定冗余

`tail_subarray`/`middle_subarray` 是本次**唯一**在 diff 中可见的语义变更点，已由 2.5 节生产数据证明为零影响。

### 5.2 SQL 谓词

sqlite 不支持 `::jsonb`，`FilterTerminalToolCallIDs` 无法单测——与既有 `FindThinkExtractCandidates` 处境相同。该谓词的等价形式已在生产实测验证（2080 个 last message 命中 30 个，与现有 Go 侧判定一致），补 e2e 用例兜底。

### 5.3 语义等价性

已在全量生产数据上完成精确对比（`lost = 0`、`gained = 0`，两种语义均为同一批 31 个 session）。**不需要**把生产数据导出到本地做影子跑。

### 5.4 生产回归基线

部署后直接从 `cron_call_audits` 读取，无需额外埋点：

| 指标 | 当前 | PR1 后 | PR2 后 |
|---|---|---|---|
| `duration_ms` avg | 1586 | 400~600 | 150~250 |
| `deduped_sessions_count` | 0~94/次 | 不变 | **不变（±0）** |

`deduped_sessions_count` 必须与改造前同量级，这是语义等价的生产侧断言。若 PR2 上线后该值异常升高，说明前缀判定实现有误，应回滚。

## 6. 部署

两个 PR 均无 DB schema 变更、无数据迁移、无部署顺序约束。cron 每小时自然触发，亦可用既有手动 `Trigger` 立即验证。

## 7. 代码约束

遵守仓库 conv lint：`internal/` 下禁 `_test.go`、字符串字面量提取到 `constant` 包、`gocognit` ≤ 25、`nestif` ≤ 5、禁匿名 struct。验证用 `go run ./cmd/server lint conv ./...`（仓库根，勿只跑子目录）。
