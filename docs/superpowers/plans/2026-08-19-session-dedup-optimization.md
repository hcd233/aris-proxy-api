# Session 去重算法优化 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保持去重结果不变的前提下，把 `SessionDeduplicateCron` 的耗时从 1586ms 降到 150~250ms、峰值内存降约 80%，并把两段互相打补丁的规则统一为单一决策。

**Architecture:** 分两个 PR。PR1 把 terminal tool call 的 role/tool_calls 判定下推到 SQL（IO 5063 kB → ~1 KB），并把 ToolIDs 合并与冗余删除放进同一事务；PR2 按首个 message ID 分组、把冗余判定从「连续子数组」改为「前缀」，并将 `FindRedundantSessionsWithMerge` 与 `FindTerminalToolCallSessions` 合并为单一入口。

**Tech Stack:** Go + GORM（PostgreSQL，`serializer:json` 文本列）、`samber/lo`、`bytedance/sonic`、`robfig/cron`、sqlite 内存库（单测）、fixture 驱动的表驱动测试。

**设计依据：** `docs/superpowers/specs/2026-08-19-session-dedup-optimization-design.md`

## Global Constraints

- 去重结果必须与改造前一致：生产 `cron_call_audits.metadata.deduped_sessions_count` 部署后须与改造前同量级（0~94/次），异常升高即回滚。
- conv lint：`internal/` 下禁止 `_test.go`（测试放 `test/unit/session_dedup/`）；字符串字面量提取到 `constant` 包；`gocognit` ≤ 25；`nestif` ≤ 5；禁匿名 struct。
- 验证命令：`go test -count=1 ./test/unit/session_dedup/...`、`make lint`（= `lint-conv` + `lint-static`，均依赖 `web-build`）。
- 仓库 pre-commit hook 会执行 `gofmt -w .` 并 auto-stage 工作区**所有**未暂存修改；分批提交前必须先 `git stash push -- <files>` 隔离后续批次。
- 无 DB schema 变更、无数据迁移、无部署顺序约束。
- PostgreSQL 专有 SQL（`::jsonb`）无法在 sqlite 单测中覆盖，遵循既有 `FindThinkExtractCandidates` 的做法。
- 所有新增导出函数/方法需带项目风格的 doc 注释（`@param`/`@return`/`@author centonhuang`/`@update`）。

---

# PR1：IO 下推与写回原子性

分支建议：`perf/session-dedup-io-pushdown-2026-08-19`

## Task 1: SQL 谓词常量与 `FilterTerminalToolCallIDs`

**Files:**
- Modify: `internal/common/constant/database.go:22-24`（在 `DBJSONCondition*` 组内追加）
- Modify: `internal/infrastructure/database/dao/message.go`
- Test: `test/unit/session_dedup/filter_terminal_ids_test.go`（新建）

**Interfaces:**
- Consumes: 无
- Produces: `constant.DBJSONConditionHasToolCalls string`；`func (dao *MessageDAO) FilterTerminalToolCallIDs(db *gorm.DB, ids []uint) ([]uint, error)`

- [ ] **Step 1: 写失败测试**

sqlite 不支持 `::jsonb`，只能覆盖不触库的短路分支。SQL 本身由 Step 6 的生产验证 + e2e 兜底。

创建 `test/unit/session_dedup/filter_terminal_ids_test.go`：

```go
package session_dedup

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

// TestFilterTerminalToolCallIDsEmptyInput 验证空输入直接短路，不触发数据库查询。
//
// 传入 nil db：若实现没有短路，会因 nil 指针解引用 panic；短路则安全返回。
func TestFilterTerminalToolCallIDsEmptyInput(t *testing.T) {
	t.Parallel()

	got, err := dao.GetMessageDAO().FilterTerminalToolCallIDs(nil, nil)
	if err != nil {
		t.Fatalf("FilterTerminalToolCallIDs(nil, nil) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("FilterTerminalToolCallIDs(nil, nil) = %v, want empty", got)
	}

	got, err = dao.GetMessageDAO().FilterTerminalToolCallIDs(nil, []uint{})
	if err != nil {
		t.Fatalf("FilterTerminalToolCallIDs(nil, []uint{}) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("FilterTerminalToolCallIDs(nil, []uint{}) = %v, want empty", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/session_dedup/ -run TestFilterTerminalToolCallIDsEmptyInput -v`
Expected: 编译失败，`dao.MessageDAO has no field or method FilterTerminalToolCallIDs`

- [ ] **Step 3: 新增 SQL 谓词常量**

在 `internal/common/constant/database.go` 的 `DBJSONConditionReasoningEmpty` 之后追加：

```go
	// DBJSONConditionHasToolCalls message 的 tool_calls 为非空数组。
	//
	// jsonb_typeof 前置守卫是必需的：tool_calls 键缺失时 jsonb_array_length(NULL)
	// 返回 NULL 尚可，但该键为非数组类型时会直接报错。
	DBJSONConditionHasToolCalls = "jsonb_typeof((message::jsonb)->'tool_calls') = 'array' AND jsonb_array_length((message::jsonb)->'tool_calls') > 0"
```

- [ ] **Step 4: 实现 DAO 方法**

在 `internal/infrastructure/database/dao/message.go` 追加（并把 import 补成 `lo` + `gorm` + `gorm/clause`）：

```go
// FilterTerminalToolCallIDs 从候选 message ID 中筛出 role=assistant 且 tool_calls 非空的 ID
//
//	判定下推到 SQL，避免把 message JSON 全量载入内存反序列化。
//	生产实测：2080 个候选中仅 30 个命中，下推后 IO 由 5063 kB 降至约 1 KB。
//
//	谓词使用 PostgreSQL 专有的 ::jsonb 强转（message 为 text 列），
//	sqlite 不可用，故 SQL 本身由 e2e 覆盖，单测只覆盖空输入短路。
//
//	@receiver dao *MessageDAO
//	@param db *gorm.DB
//	@param ids []uint 候选 message ID
//	@return []uint 命中的 message ID
//	@return error
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func (dao *MessageDAO) FilterTerminalToolCallIDs(db *gorm.DB, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var records []*dbmodel.Message
	err := db.Model(&dbmodel.Message{}).
		Select([]string{constant.FieldID}).
		Where(constant.DBConditionWhereIDIn, ids).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.DBJSONConditionAssistantRole).
		Where(constant.DBJSONConditionHasToolCalls).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(records, func(m *dbmodel.Message, _ int) uint { return m.ID }), nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/session_dedup/ -run TestFilterTerminalToolCallIDsEmptyInput -v`
Expected: PASS

- [ ] **Step 6: 在生产只读验证 SQL 谓词等价性**

谓词是 PostgreSQL 专有且单测覆盖不到，必须实测一次。把下面 SQL 存为本地临时文件后执行（结束后删除该文件）：

```sql
SET statement_timeout = '60000ms';
WITH last_ids AS (
    SELECT DISTINCT (message_ids::jsonb ->> (jsonb_array_length(message_ids::jsonb) - 1))::bigint AS last_msg_id
    FROM sessions
    WHERE deleted_at = 0 AND jsonb_array_length(message_ids::jsonb) > 0
)
SELECT count(*) AS candidates,
       count(*) FILTER (
           WHERE (m.message::jsonb) ->> 'role' = 'assistant'
             AND jsonb_typeof((m.message::jsonb) -> 'tool_calls') = 'array'
             AND jsonb_array_length((m.message::jsonb) -> 'tool_calls') > 0
       ) AS hits
FROM messages m JOIN last_ids l ON m.id = l.last_msg_id
WHERE m.deleted_at = 0;
```

```bash
ssh -o ConnectTimeout=15 ubuntu@api.lvlvko.top 'set -a; . /home/ubuntu/code/aris-proxy-api/env/api.env; set +a; docker exec -i -e PGPASSWORD="$POSTGRES_PASSWORD" postgresql psql --no-psqlrc --username="$POSTGRES_USER" --dbname="$POSTGRES_DATABASE" -f -' < /tmp/verify-predicate.sql
```

Expected: `candidates` 约 2080、`hits` 约 30（2026-08-19 基线）。`hits` 若为 0 或接近 `candidates`，说明谓词写错，停止并修正。

> 通过 SSH 到生产前先加载 `login-prod-server` skill；只读查询无需授权，禁止输出凭据。

- [ ] **Step 7: 提交**

```bash
git add internal/common/constant/database.go internal/infrastructure/database/dao/message.go test/unit/session_dedup/filter_terminal_ids_test.go
git commit -m "feat(dao): push terminal tool call detection down to SQL"
```

## Task 2: `FindTerminalToolCallSessions` 改为接收 terminal message ID

把「判定」交给 SQL，「决策」留在纯函数。

**Files:**
- Modify: `internal/cron/session_dedup.go:214-242`（`loadLastMessagesForTerminalToolCheck`）、`:488-555`（`FindTerminalToolCallSessions`、`processTerminalToolCallSession`）、`:148-164`（`deduplicate` 接线）
- Modify: `test/unit/session_dedup/session_dedup_test.go`
- Modify: `test/unit/session_dedup/fixtures/terminal_tool_call_cases.json`

**Interfaces:**
- Consumes: `dao.MessageDAO.FilterTerminalToolCallIDs`（Task 1）
- Produces: `func FindTerminalToolCallSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint, excludeIDs []uint) MergeResult`；`func (c *SessionDeduplicateCron) loadTerminalToolCallMsgIDs(db *gorm.DB, sessions []*dbmodel.Session, excludeIDs []uint) ([]uint, error)`

- [ ] **Step 1: 改造 fixture 为 `terminal_msg_ids`**

`test/unit/session_dedup/fixtures/terminal_tool_call_cases.json` 中，每个 case 把 `"messages": [...]` 整个替换为 `"terminal_msg_ids": [...]`，值为原 messages 中 `role == "assistant"` 且 `tool_calls` 非空的那些 `id`：

| case | 原 messages 中命中的 id | 新字段值 |
|---|---|---|
| `terminal_tool_call_basic` | 3 | `[3]` |
| `terminal_tool_call_no_parent` | 2 | `[2]` |
| `terminal_tool_call_excluded_session` | 2 | `[2]` |
| `terminal_tool_call_multiple_parents_picks_longest` | 3 | `[3]` |
| `terminal_tool_call_no_tool_ids` | 3 | `[3]` |
| `terminal_tool_call_last_msg_not_assistant` | 无 | `[]` |
| `terminal_tool_call_empty_sessions` | 无 | `[]` |
| `terminal_tool_call_merge_target_excluded` | 50 | `[50]` |

其余字段（`sessions`/`exclude_ids`/`expected_redundant_ids`/`expected_merge_mapping`）**全部保持不变**——期望值不变正是本 Task 语义等价的证明。

- [ ] **Step 2: 改造测试代码**

在 `test/unit/session_dedup/session_dedup_test.go` 中：

1. `terminalToolCallCase` 结构体：删除 `Messages []terminalToolCallMessageFix` 字段，新增 `TerminalMsgIDs []uint \`json:"terminal_msg_ids"\``
2. 删除 `terminalToolCallMessageFix`、`terminalToolCallFix` 两个结构体与 `toDBMessages` 函数
3. 删除 `vo` 与 `dbmodel` 中仅为 `toDBMessages` 服务的 import（`dbmodel` 仍被 `toDBSessions` 使用，只删 `vo`）
4. 调用处 `cron.FindTerminalToolCallSessions(sessions, toDBMessages(tc.Messages), tc.ExcludeIDs)` 改为 `cron.FindTerminalToolCallSessions(sessions, tc.TerminalMsgIDs, tc.ExcludeIDs)`

- [ ] **Step 3: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/session_dedup/ -run TestFindTerminalToolCallSessions -v`
Expected: 编译失败，`too many arguments` / `cannot use tc.TerminalMsgIDs (variable of type []uint) as []*dbmodel.Message`

- [ ] **Step 4: 改造实现**

`internal/cron/session_dedup.go`：

```go
// FindTerminalToolCallSessions 查找末条消息为 assistant+tool_calls 的 session
//
// 这些 session 的对话在工具调用阶段中断，属于不完整分支。
// 标记为冗余并尝试查找 parent session 以合并 ToolIDs。
//
// role/tool_calls 的判定已下推到 SQL（见 MessageDAO.FilterTerminalToolCallIDs），
// 本函数只消费判定结果，不再加载 message payload。
//
//	@param sessions []*dbmodel.Session
//	@param terminalMsgIDs []uint 已判定为 assistant+tool_calls 的 message ID
//	@param excludeIDs []uint 已被前缀检查标记为冗余或作为 merge target 的 session ID
//	@return MergeResult
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func FindTerminalToolCallSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint, excludeIDs []uint) MergeResult {
	excludeSet := lo.SliceToMap(excludeIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })
	terminalSet := lo.SliceToMap(terminalMsgIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })
	sessionByID := lo.SliceToMap(sessions, func(s *dbmodel.Session) (uint, *dbmodel.Session) { return s.ID, s })

	result := MergeResult{
		RedundantIDs: make([]uint, 0),
		MergeMapping: make(map[uint]map[uint]struct{}),
	}

	for _, s := range sessions {
		if _, excluded := excludeSet[s.ID]; excluded {
			continue
		}
		processTerminalToolCallSession(s, sessions, terminalSet, sessionByID, &result)
	}

	return result
}

// processTerminalToolCallSession 检查单个 session 是否为终端 tool_call session，若是则标记冗余并合并 ToolIDs
//
//	@param s *dbmodel.Session
//	@param sessions []*dbmodel.Session
//	@param terminalSet map[uint]struct{}
//	@param sessionByID map[uint]*dbmodel.Session
//	@param result *MergeResult
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func processTerminalToolCallSession(s *dbmodel.Session, sessions []*dbmodel.Session, terminalSet map[uint]struct{}, sessionByID map[uint]*dbmodel.Session, result *MergeResult) {
	if len(s.MessageIDs) == 0 {
		return
	}

	lastMsgID := s.MessageIDs[len(s.MessageIDs)-1]
	if _, ok := terminalSet[lastMsgID]; !ok {
		return
	}

	result.RedundantIDs = append(result.RedundantIDs, s.ID)

	if len(s.ToolIDs) == 0 {
		return
	}

	parentID := findParentSessionID(s, sessions)
	if parentID == 0 {
		return
	}

	parentOwn := lo.SliceToMap(sessionByID[parentID].ToolIDs, func(tid uint) (uint, struct{}) { return tid, struct{}{} })
	incoming := lo.SliceToMap(s.ToolIDs, func(tid uint) (uint, struct{}) { return tid, struct{}{} })
	// 必须合并而非覆盖：同一 parent 可能有多个 terminal 子 session（Task 3 的回归用例）
	result.MergeMapping[parentID] = mergeToolIDs(mergeToolIDs(result.MergeMapping[parentID], parentOwn), incoming)
}
```

把 `loadLastMessagesForTerminalToolCheck` 整体替换为：

```go
// loadTerminalToolCallMsgIDs 取出候选 session 的末条 message ID，并下推 SQL 筛出终端 tool_call 消息
//
//	lo.Uniq 后的 ID 仅用于 WHERE IN 查询，调用方按集合语义使用返回值，不依赖顺序。
//
//	@receiver c *SessionDeduplicateCron
//	@param db *gorm.DB
//	@param sessions []*dbmodel.Session
//	@param excludeIDs []uint
//	@return []uint
//	@return error
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func (c *SessionDeduplicateCron) loadTerminalToolCallMsgIDs(db *gorm.DB, sessions []*dbmodel.Session, excludeIDs []uint) ([]uint, error) {
	excludeSet := lo.SliceToMap(excludeIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })

	lastMsgIDs := lo.FilterMap(sessions, func(s *dbmodel.Session, _ int) (uint, bool) {
		if _, excluded := excludeSet[s.ID]; excluded {
			return 0, false
		}
		if len(s.MessageIDs) == 0 {
			return 0, false
		}
		return s.MessageIDs[len(s.MessageIDs)-1], true
	})

	if len(lastMsgIDs) == 0 {
		return nil, nil
	}

	return c.messageDAO.FilterTerminalToolCallIDs(db, lo.Uniq(lastMsgIDs))
}
```

`deduplicate` 中的接线（原 `:147-164`）：

```go
	// 额外检查：Session 末条消息是 assistant 且有 tool_calls 的也标记为冗余
	terminalMsgIDs, err := c.loadTerminalToolCallMsgIDs(db, sessions, terminalExcludeIDs)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load terminal tool call message ids", zap.Error(err))
		// 不 return，继续执行已有的去重结果
	}

	if len(terminalMsgIDs) > 0 {
		terminalToolCallResult := FindTerminalToolCallSessions(sessions, terminalMsgIDs, terminalExcludeIDs)
		if len(terminalToolCallResult.RedundantIDs) > 0 {
			mergeResult.RedundantIDs = append(mergeResult.RedundantIDs, terminalToolCallResult.RedundantIDs...)

			// 合并 TerminalToolCall 的 ToolIDs 映射到主结果
			for sessionID, toolIDSet := range terminalToolCallResult.MergeMapping {
				mergeResult.MergeMapping[sessionID] = mergeToolIDs(mergeResult.MergeMapping[sessionID], toolIDSet)
			}
		}
	}
```

删除 `session_dedup.go` 中已不再使用的 import：`"github.com/hcd233/aris-proxy-api/internal/common/enum"` 与 `"github.com/samber/mo"`（两者原先只被删掉的 role/tool_calls 判定使用）。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/session_dedup/... -v`
Expected: 全部 PASS（含未改动的 `TestIsSubArray`、`TestFindRedundantSessions`）

- [ ] **Step 6: 提交**

```bash
git add internal/cron/session_dedup.go test/unit/session_dedup/
git commit -m "refactor(cron): consume SQL-detected terminal tool call ids"
```

## Task 3: 修复 `MergeMapping` 覆盖导致 ToolIDs 丢失

Task 2 的实现已写成合并形式，本 Task 补上锁死该行为的回归用例。

**Files:**
- Modify: `test/unit/session_dedup/fixtures/terminal_tool_call_cases.json`
- Modify: `test/unit/session_dedup/session_dedup_test.go`（`caseNames` 列表）

**Interfaces:**
- Consumes: `FindTerminalToolCallSessions`（Task 2）
- Produces: 无

- [ ] **Step 1: 新增回归 fixture**

在 `terminal_tool_call_cases.json` 数组末尾追加：

```json
    {
        "name": "terminal_tool_call_two_children_same_parent",
        "description": "Two terminal sessions sharing the same parent must both merge their ToolIDs into it. Regression for MergeMapping[parentID] = set overwriting the first child's tools.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2, 3, 4, 5, 6], "tool_ids": [100]},
            {"id": 2, "message_ids": [1, 2, 3], "tool_ids": [20]},
            {"id": 3, "message_ids": [1, 2, 3, 4], "tool_ids": [30]}
        ],
        "terminal_msg_ids": [3, 4],
        "exclude_ids": [],
        "expected_redundant_ids": [2, 3],
        "expected_merge_mapping": {"1": [20, 30, 100]}
    }
```

推演：session 1 末条 6 不在 `terminal_msg_ids` → 保留；session 2 末条 3 命中 → 冗余，parent 取最长容器 session 1，映射为 `{100, 20}`；session 3 末条 4 命中 → 冗余，parent 同为 session 1。**改造前**会把映射覆盖成 `{100, 30}`（丢掉 20），改造后为 `{100, 20, 30}`。

- [ ] **Step 2: 在测试中注册该 case**

在 `session_dedup_test.go` 的 `TestFindTerminalToolCallSessions` 的 `caseNames` 列表末尾加入 `"terminal_tool_call_two_children_same_parent"`。

- [ ] **Step 3: 用 git stash 验证该用例确实能捕获缺陷**

```bash
git stash push -- internal/cron/session_dedup.go
go test -count=1 ./test/unit/session_dedup/ -run TestFindTerminalToolCallSessions/terminal_tool_call_two_children_same_parent -v
```

Expected: FAIL（旧实现丢失 tool ID 20）。若 PASS 说明用例没吃到缺陷，需重新设计用例。

```bash
git stash pop
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/session_dedup/... -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交**

```bash
git add test/unit/session_dedup/
git commit -m "fix(cron): merge instead of overwrite tool ids for shared parent session"
```

## Task 4: 写回原子化

**Files:**
- Modify: `internal/cron/session_dedup.go:173-201`（`deduplicate` 的写回段）
- Test: `test/unit/session_dedup/apply_merge_result_test.go`（新建）

**Interfaces:**
- Consumes: `MergeResult`
- Produces: `func ApplyMergeResult(db *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error)`

- [ ] **Step 1: 写失败测试**

创建 `test/unit/session_dedup/apply_merge_result_test.go`：

```go
package session_dedup

import (
	"errors"
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newApplyTestDB 创建 sqlite 内存库并迁移 sessions 表
func newApplyTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Session{}); err != nil {
		t.Fatalf("failed to migrate sqlite db: %v", err)
	}
	return db
}

// seedSessions 写入两条 session：id=1 为保留者，id=2 为冗余者
func seedSessions(t *testing.T, db *gorm.DB) {
	t.Helper()
	keeper := &dbmodel.Session{MessageIDs: []uint{1, 2, 3}, ToolIDs: []uint{10}}
	keeper.ID = 1
	redundant := &dbmodel.Session{MessageIDs: []uint{1, 2}, ToolIDs: []uint{20}}
	redundant.ID = 2
	for _, s := range []*dbmodel.Session{keeper, redundant} {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("failed to seed session %d: %v", s.ID, err)
		}
	}
}

// TestApplyMergeResultCommits 验证 ToolIDs 合并与冗余软删在同一事务内成功提交
func TestApplyMergeResultCommits(t *testing.T) {
	t.Parallel()
	db := newApplyTestDB(t, "apply_merge_commit")
	seedSessions(t, db)

	result := cron.MergeResult{
		RedundantIDs: []uint{2},
		MergeMapping: map[uint]map[uint]struct{}{
			1: {10: {}, 20: {}},
		},
	}

	merged, err := cron.ApplyMergeResult(db, dao.GetSessionDAO(), result)
	if err != nil {
		t.Fatalf("ApplyMergeResult() error = %v, want nil", err)
	}
	if merged != 1 {
		t.Errorf("ApplyMergeResult() merged = %d, want 1", merged)
	}

	var keeper dbmodel.Session
	if err := db.Unscoped().Where("id = ?", 1).First(&keeper).Error; err != nil {
		t.Fatalf("failed to reload keeper: %v", err)
	}
	if !slices.Equal(keeper.ToolIDs, []uint{10, 20}) {
		t.Errorf("keeper tool_ids = %v, want [10 20]", keeper.ToolIDs)
	}
	if keeper.DeletedAt != 0 {
		t.Errorf("keeper deleted_at = %d, want 0", keeper.DeletedAt)
	}

	var deleted dbmodel.Session
	if err := db.Unscoped().Where("id = ?", 2).First(&deleted).Error; err != nil {
		t.Fatalf("failed to reload redundant: %v", err)
	}
	if deleted.DeletedAt == 0 {
		t.Error("redundant session deleted_at = 0, want non-zero (soft deleted)")
	}
}

// TestApplyMergeResultRollsBackOnFailure 验证 ToolIDs 更新失败时不会删除冗余 Session。
//
// 这是旧实现的数据丢失路径：更新失败被 continue 吞掉后仍执行 BatchDeleteByField，
// 冗余 session 被删而 ToolIDs 未合并，tool 引用永久丢失。
//
// 失败注入用 GORM callback（driver 无关），不用 DropColumn——sqlite 的 DropColumn
// 会重建表，行为不可靠。
func TestApplyMergeResultRollsBackOnFailure(t *testing.T) {
	t.Parallel()
	db := newApplyTestDB(t, "apply_merge_rollback")
	seedSessions(t, db)

	if err := db.Callback().Update().Before("gorm:update").
		Register("test:force_update_error", func(tx *gorm.DB) {
			tx.AddError(errors.New("injected update failure"))
		}); err != nil {
		t.Fatalf("failed to register callback: %v", err)
	}

	result := cron.MergeResult{
		RedundantIDs: []uint{2},
		MergeMapping: map[uint]map[uint]struct{}{
			1: {10: {}, 20: {}},
		},
	}

	if _, err := cron.ApplyMergeResult(db, dao.GetSessionDAO(), result); err == nil {
		t.Fatal("ApplyMergeResult() error = nil, want non-nil")
	}

	var redundant dbmodel.Session
	if err := db.Unscoped().Where("id = ?", 2).First(&redundant).Error; err != nil {
		t.Fatalf("failed to reload redundant session: %v", err)
	}
	if redundant.DeletedAt != 0 {
		t.Error("redundant session deleted_at != 0, want 0: it must not be deleted when the tool id update failed")
	}
}
```

> 注意 `BatchDeleteByField` 内部也是 UPDATE（软删写 `deleted_at`），同样会被注入的 callback 拦下。但由于 ToolIDs 更新先失败并直接 `return err`，删除根本不会执行，断言依然成立——这正是要锁住的行为。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/session_dedup/ -run TestApplyMergeResult -v`
Expected: 编译失败，`undefined: cron.ApplyMergeResult`

- [ ] **Step 3: 实现 `ApplyMergeResult`**

在 `internal/cron/session_dedup.go` 中新增：

```go
// ApplyMergeResult 在单个事务内写回 ToolIDs 合并结果并软删冗余 Session
//
// 原子性是必需的：若 tool_ids 更新失败却仍执行删除，被删 session 的 tool 引用
// 会永久丢失。任一步骤失败即整体回滚，任务幂等，下个整点重跑。
//
//	导出以便外部测试包验证写回的原子性。
//
//	@param db *gorm.DB
//	@param sessionDAO *dao.SessionDAO
//	@param result MergeResult
//	@return int 成功合并 ToolIDs 的 Session 数
//	@return error
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func ApplyMergeResult(db *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error) {
	mergedCount := 0

	err := db.Transaction(func(tx *gorm.DB) error {
		mergedCount = 0
		for sessionID, toolIDSet := range result.MergeMapping {
			if len(toolIDSet) == 0 {
				continue
			}

			// 集合转排序切片，保证写入内容稳定
			mergedToolIDs := lo.Keys(toolIDSet)
			slices.Sort(mergedToolIDs)

			// tool_ids 列为 text 类型(GORM serializer:json)，直接存 JSON 字符串
			toolIDsJSON, err := sonic.MarshalString(mergedToolIDs)
			if err != nil {
				return err
			}
			if err := sessionDAO.Update(tx, &dbmodel.Session{ID: sessionID}, map[string]any{
				constant.FieldToolIDs: toolIDsJSON,
			}); err != nil {
				return err
			}
			mergedCount++
		}

		return sessionDAO.BatchDeleteByField(tx, constant.WhereFieldID, result.RedundantIDs)
	})
	if err != nil {
		return 0, err
	}

	return mergedCount, nil
}
```

把 `deduplicate` 中原来的写回段（`// 合并ToolIDs到保留的Session` 起至 `BatchDeleteByField` 的错误处理止）整体替换为：

```go
	mergedCount, err := ApplyMergeResult(db, c.sessionDAO, mergeResult)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to apply deduplication", zap.Error(err))
		return nil, err
	}
```

注意 `lo.Must1` 已被显式错误处理替代，`sonic` 与 `slices` import 保留。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/session_dedup/... -v`
Expected: 全部 PASS

- [ ] **Step 5: 跑全量校验**

Run: `go test -count=1 ./test/unit/... && make lint`
Expected: 测试全绿；lint 无新增告警（若 `lint-static` 报陈旧路径问题，执行 `$(go env GOPATH)/bin/golangci-lint cache clean` 后重跑）

- [ ] **Step 6: 提交**

```bash
git add internal/cron/session_dedup.go test/unit/session_dedup/
git commit -m "fix(cron): apply tool id merge and redundant delete in one transaction"
```

---

# PR2：分组与前缀语义统一

分支建议：`perf/session-dedup-prefix-grouping-2026-08-19`。PR1 合入后再开始。

## Task 5: 分组与前缀判定原语

**Files:**
- Modify: `internal/cron/session_dedup.go`（新增 `groupByFirstMessageID`、`isPrefix`）
- Test: `test/unit/session_dedup/fixtures/find_redundant_sessions_cases.json`
- Test: `test/unit/session_dedup/session_dedup_test.go`

**Interfaces:**
- Consumes: `sessionEntry`（既有）
- Produces: `func isPrefix(short, long []uint) bool`（包内）、`func groupByFirstMessageID(sessions []*dbmodel.Session) map[uint][]sessionEntry`（包内）

- [ ] **Step 1: 改写受语义收窄影响的 fixture 期望**

在 `find_redundant_sessions_cases.json` 中，把这两个 case 的 `expected_redundant_ids` 由 `[2]` 改为 `[]`，并同步更新 `description`：

```json
    {
        "name": "tail_subarray",
        "description": "Session B [3,4,5] is a tail (not prefix) subarray of Session A [1,2,3,4,5]. Under prefix semantics B is NOT redundant: a differing first message means a different conversation root. Verified against full production data (2026-08-19): zero such pairs exist.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2, 3, 4, 5]},
            {"id": 2, "message_ids": [3, 4, 5]}
        ],
        "expected_redundant_ids": []
    },
    {
        "name": "middle_subarray",
        "description": "Session B [2,3,4] is a middle (not prefix) subarray of Session A [1,2,3,4,5]. Under prefix semantics B is NOT redundant. Verified against full production data (2026-08-19): zero such pairs exist.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2, 3, 4, 5]},
            {"id": 2, "message_ids": [2, 3, 4]}
        ],
        "expected_redundant_ids": []
    },
```

`single_element_subarray`（`[1,2,3,4,5]` 与 `[3]`）同样是非前缀关系，`expected_redundant_ids` 由 `[2]` 改为 `[]`，description 追加同样的说明。其余 case 期望值不变。

- [ ] **Step 2: 新增跨组不互删的 fixture**

在 `find_redundant_sessions_cases.json` 追加：

```json
    {
        "name": "cross_group_not_redundant",
        "description": "Sessions with different first message ids belong to different conversations and never dedup each other, even if one's ids are a subset of the other's.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2, 3, 4]},
            {"id": 2, "message_ids": [7, 2, 3]}
        ],
        "expected_redundant_ids": []
    },
    {
        "name": "forked_conversation",
        "description": "A conversation that forks: [1,2] is a prefix of both [1,2,3] and [1,2,4]. Only [1,2] is redundant; both branches are kept.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2, 3], "tool_ids": [10]},
            {"id": 2, "message_ids": [1, 2, 4], "tool_ids": [20]},
            {"id": 3, "message_ids": [1, 2], "tool_ids": [30]}
        ],
        "expected_redundant_ids": [3]
    }
```

- [ ] **Step 3: 在测试中注册新 case**

在 `TestFindRedundantSessions` 的 `caseNames` 末尾加入 `"cross_group_not_redundant"` 与 `"forked_conversation"`。

- [ ] **Step 4: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/session_dedup/ -run TestFindRedundantSessions -v`
Expected: `tail_subarray`、`middle_subarray`、`single_element_subarray`、`cross_group_not_redundant` FAIL（旧的子数组语义仍会判定冗余）；`forked_conversation` PASS（旧语义下结果相同）

- [ ] **Step 5: 实现分组与前缀原语**

在 `internal/cron/session_dedup.go` 中新增：

```go
// isPrefix 判断 short 是否是 long 的前缀
//
//	长度比较先做 O(1) 剪枝，避免无谓的逐元素比较。
//
//	@param short []uint
//	@param long []uint
//	@return bool
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func isPrefix(short, long []uint) bool {
	if len(short) > len(long) {
		return false
	}
	return slices.Equal(long[:len(short)], short)
}

// groupByFirstMessageID 按首个 message ID 将 session 分组，组内按 MessageIDs 长度降序、ID 升序排列
//
// 同一对话的所有快照必然共享首个 message ID：每次请求都把完整对话历史按 checksum
// 去重后落库，历史消息复用同一行，故第 k 轮的 MessageIDs 是第 k+1 轮的前缀。
// 跨组不可能存在冗余关系，分组把两两比较从 O(N²) 降到 Σ(组内²)。
//
// 生产实测（2026-08-19）：2551 个 session → 913 组、603 个单例组，
// 比较次数 6,507,601 → 145,503（44.7 倍）。
//
//	@param sessions []*dbmodel.Session
//	@return map[uint][]sessionEntry key 为首个 message ID
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func groupByFirstMessageID(sessions []*dbmodel.Session) map[uint][]sessionEntry {
	groups := make(map[uint][]sessionEntry)

	for _, s := range sessions {
		if len(s.MessageIDs) == 0 {
			continue
		}
		firstMsgID := s.MessageIDs[0]
		groups[firstMsgID] = append(groups[firstMsgID], sessionEntry{
			id:         s.ID,
			messageIDs: s.MessageIDs,
			toolIDs:    s.ToolIDs,
		})
	}

	for _, entries := range groups {
		slices.SortFunc(entries, func(a, b sessionEntry) int {
			if len(a.messageIDs) != len(b.messageIDs) {
				return len(b.messageIDs) - len(a.messageIDs)
			}
			return int(a.id) - int(b.id)
		})
	}

	return groups
}
```

> `int(a.id) - int(b.id)` 在 `uint` 超过 int 范围时会溢出，但 session ID 由自增主键产生，生产最大值约 5 万，安全。若需严格无溢出可改用 `cmp.Compare(a.id, b.id)`。

- [ ] **Step 6: 改写 `FindRedundantSessionsWithMerge` 使用分组与前缀**

替换 `FindRedundantSessionsWithMerge`、删除 `prepareSessionEntries` 与 `processEntryAgainstShorter`：

```go
// FindRedundantSessionsWithMerge 查找 MessageIDs 是其他 Session 前缀的冗余 Session，并返回 ToolIDs 合并信息
//
// 算法：
//
//  1. 按首个 message ID 分组，组内按 MessageIDs 长度降序、ID 升序排列
//
//  2. 组内扫描并维护 keeper 列表：成员若是某个 keeper 的前缀则判为冗余，
//     ToolIDs 并入首个匹配的 keeper；否则自身成为新 keeper（处理对话分叉）
//
//     @param sessions []*dbmodel.Session
//     @return MergeResult 包含需要删除的 Session ID 和 ToolIDs 合并映射
//     @author centonhuang
//     @update 2026-08-19 10:00:00
func FindRedundantSessionsWithMerge(sessions []*dbmodel.Session) MergeResult {
	result := MergeResult{
		RedundantIDs: make([]uint, 0),
		MergeMapping: make(map[uint]map[uint]struct{}),
	}

	for _, entries := range groupByFirstMessageID(sessions) {
		resolveGroup(entries, &result, nil)
	}

	return result
}

// resolveGroup 处理单个对话组：前缀成员判为冗余并把 ToolIDs 并入首个匹配的 keeper
//
//	absorbed 非 nil 时，记录吸收过冗余成员的 keeper ID（即真正的 merge target），
//	供 terminal 规则判定是否需要保护该 keeper。
//
//	@param entries []sessionEntry 组内条目，已按长度降序、ID 升序排列
//	@param result *MergeResult
//	@param absorbed map[uint]struct{} 可为 nil
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func resolveGroup(entries []sessionEntry, result *MergeResult, absorbed map[uint]struct{}) {
	keepers := make([]sessionEntry, 0, len(entries))

	for _, e := range entries {
		// keepers 按加入顺序即长度降序、ID 升序，首个匹配者即最长且 ID 最小的容器
		container, found := lo.Find(keepers, func(k sessionEntry) bool {
			return isPrefix(e.messageIDs, k.messageIDs)
		})
		if !found {
			keepers = append(keepers, e)
			continue
		}

		result.RedundantIDs = append(result.RedundantIDs, e.id)
		mergeToolIDsIntoMapping(result.MergeMapping, container.id, container.toolIDs, e.toolIDs)
		if absorbed != nil {
			absorbed[container.id] = struct{}{}
		}
	}
}
```

- [ ] **Step 7: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/session_dedup/... -v`
Expected: `TestFindRedundantSessions` 与 `TestFindRedundantSessionsWithMerge` 全部 PASS。`TestIsSubArray` 仍应 PASS（`IsSubArray` 在 Task 7 才删除）

- [ ] **Step 8: 提交**

```bash
git add internal/cron/session_dedup.go test/unit/session_dedup/
git commit -m "perf(cron): group sessions by first message id and use prefix matching"
```

## Task 6: 合并两段入口并落地选项 X

**Files:**
- Modify: `internal/cron/session_dedup.go`（新增 `FindRedundantSessions` 单一入口；`deduplicate` 接线）
- Test: `test/unit/session_dedup/fixtures/find_redundant_sessions_cases.json`
- Test: `test/unit/session_dedup/session_dedup_test.go`

**Interfaces:**
- Consumes: `groupByFirstMessageID`、`resolveGroup`、`isPrefix`（Task 5）；`ApplyMergeResult`（Task 4）
- Produces: `func FindRedundantSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint) MergeResult`（签名变更：新增第二参数）

- [ ] **Step 1: 为 fixture 增加 `terminal_msg_ids` 字段并新增三个决策用例**

在 `session_dedup_test.go` 的 `findRedundantSessionsCase` 结构体中新增字段：

```go
	TerminalMsgIDs []uint `json:"terminal_msg_ids"`
```

在 `find_redundant_sessions_cases.json` 追加三个用例（已有用例不加该字段，反序列化为 nil 即无终端消息）：

```json
    {
        "name": "group_keeper_protected_without_tools",
        "description": "The group keeper absorbed a redundant member, so it is a merge target and must be protected from the terminal rule even though every tool_ids is empty. Before this fix the keeper fell out of MergeMapping (both sides empty) and got deleted, wiping the whole conversation.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2, 3, 4], "tool_ids": []},
            {"id": 2, "message_ids": [1, 2, 3], "tool_ids": []}
        ],
        "terminal_msg_ids": [4],
        "expected_redundant_ids": [2]
    },
    {
        "name": "forked_keeper_not_protected",
        "description": "A fork branch that absorbed nothing is not a merge target, so the terminal rule still applies to it. Keeps parity with the pre-refactor behaviour.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2, 3], "tool_ids": [10]},
            {"id": 2, "message_ids": [1, 2, 4], "tool_ids": [20]},
            {"id": 3, "message_ids": [1, 2], "tool_ids": [30]}
        ],
        "terminal_msg_ids": [4],
        "expected_redundant_ids": [2, 3]
    },
    {
        "name": "singleton_terminal_tool_call",
        "description": "A lone snapshot whose last message is assistant+tool_calls is an interrupted branch and gets removed; its ToolIDs have nowhere to merge and are dropped.",
        "sessions": [
            {"id": 1, "message_ids": [1, 2], "tool_ids": [10]}
        ],
        "terminal_msg_ids": [2],
        "expected_redundant_ids": [1]
    }
```

推演 `forked_keeper_not_protected`：组内排序 `[1,2,3](id1)`、`[1,2,4](id2)`、`[1,2](id3)`。id1 成 keeper；id2 不是 id1 的前缀 → 也成 keeper；id3 是 id1 的前缀 → 冗余，并入 id1，故 `absorbed = {1}`。terminal 阶段：id1 在 absorbed 中受保护；id2 未吸收任何成员且末条 4 命中 → 冗余。结果 `[3, 2]`，排序后 `[2, 3]`。

- [ ] **Step 2: 改造测试调用**

`TestFindRedundantSessions` 中 `cron.FindRedundantSessions(sessions)` 改为 `cron.FindRedundantSessions(sessions, tc.TerminalMsgIDs)`；在 `caseNames` 末尾加入三个新用例名。

- [ ] **Step 3: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/session_dedup/ -run TestFindRedundantSessions -v`
Expected: 编译失败，`too many arguments in call to cron.FindRedundantSessions`

- [ ] **Step 4: 实现单一入口**

把 `FindRedundantSessions` 与 `FindRedundantSessionsWithMerge` 合并为一个入口（删除后者，`FindRedundantSessionsWithMerge` 的既有测试改调新入口）：

```go
// FindRedundantSessions 查找冗余 Session 并给出 ToolIDs 合并映射
//
// 算法：
//
//  1. 按首个 message ID 分组（同一对话的快照集合），组内按 MessageIDs 长度降序、ID 升序排列
//
//  2. 组内扫描并维护 keeper 列表：成员若是某个 keeper 的前缀则判为冗余，
//     ToolIDs 并入首个匹配的 keeper；否则自身成为新 keeper（处理对话分叉）
//
//  3. 未吸收任何冗余成员的 session（含分叉 keeper 与单例组成员），
//     若末条 message 属于 terminalMsgIDs（assistant 且 tool_calls 非空，
//     即对话在工具调用处中断）则判为冗余；吸收过冗余成员的 keeper 是 merge target，
//     受保护不被删除，否则并入它的 ToolIDs 会随之丢失
//
//     @param sessions []*dbmodel.Session
//     @param terminalMsgIDs []uint 已判定为 assistant+tool_calls 的 message ID
//     @return MergeResult
//     @author centonhuang
//     @update 2026-08-19 10:00:00
func FindRedundantSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint) MergeResult {
	result := MergeResult{
		RedundantIDs: make([]uint, 0),
		MergeMapping: make(map[uint]map[uint]struct{}),
	}
	absorbed := make(map[uint]struct{})

	groups := groupByFirstMessageID(sessions)
	for _, entries := range groups {
		resolveGroup(entries, &result, absorbed)
	}

	applyTerminalRule(groups, terminalMsgIDs, absorbed, &result)

	return result
}

// applyTerminalRule 对未吸收冗余成员的 session 应用终端 tool_call 规则
//
//	@param groups map[uint][]sessionEntry
//	@param terminalMsgIDs []uint
//	@param absorbed map[uint]struct{} 已吸收冗余成员的 merge target，受保护
//	@param result *MergeResult
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func applyTerminalRule(groups map[uint][]sessionEntry, terminalMsgIDs []uint, absorbed map[uint]struct{}, result *MergeResult) {
	if len(terminalMsgIDs) == 0 {
		return
	}

	terminalSet := lo.SliceToMap(terminalMsgIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })
	redundantSet := lo.SliceToMap(result.RedundantIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })

	for _, entries := range groups {
		for _, e := range entries {
			if _, protected := absorbed[e.id]; protected {
				continue
			}
			if _, already := redundantSet[e.id]; already {
				continue
			}
			if _, terminal := terminalSet[e.messageIDs[len(e.messageIDs)-1]]; !terminal {
				continue
			}
			result.RedundantIDs = append(result.RedundantIDs, e.id)
		}
	}
}
```

`deduplicate` 接线简化为（删除 `terminalExcludeIDs` 拼接、`FindTerminalToolCallSessions` 调用与 MergeMapping 二次合并共约 25 行）：

```go
	// 先按前缀分组去重，再对未吸收冗余的 session 应用终端 tool_call 规则
	terminalMsgIDs, err := c.loadTerminalToolCallMsgIDs(db, sessions, nil)
	if err != nil {
		log.Error("[SessionDeduplicateCron] Failed to load terminal tool call message ids", zap.Error(err))
		// 不 return，退化为仅执行前缀去重
	}

	mergeResult := FindRedundantSessions(sessions, terminalMsgIDs)
```

> `loadTerminalToolCallMsgIDs` 的 `excludeIDs` 传 nil：判定已下推 SQL 后单次查询成本约 1 KB，不再需要用 exclude 列表缩小候选集，保护逻辑统一由 `absorbed` 表达。
>
> `deduplicate` 开头的 `checkedCount := int64(len(sessions))` 与 `if len(sessions) < 2 { ... }` 早退逻辑**保持不变**——虽然单个 session 理论上也可能命中 terminal 规则，但这是改造前的既有行为，本次不改动以维持结果等价。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/session_dedup/... -v`
Expected: 全部 PASS

- [ ] **Step 6: 提交**

```bash
git add internal/cron/session_dedup.go test/unit/session_dedup/
git commit -m "refactor(cron): merge dedup entrypoints and protect merge targets consistently"
```

## Task 7: 删除死代码与过时 fixture

**Files:**
- Modify: `internal/cron/session_dedup.go`
- Delete: `test/unit/session_dedup/fixtures/is_sub_array_cases.json`
- Delete: `test/unit/session_dedup/fixtures/terminal_tool_call_cases.json`
- Modify: `test/unit/session_dedup/session_dedup_test.go`

**Interfaces:**
- Consumes: `FindRedundantSessions`（Task 6）
- Produces: 无（纯删除）

- [ ] **Step 1: 删除测试与 fixture**

1. 删除 `TestIsSubArray`、`isSubArrayCase`、`loadIsSubArrayCases`、`findIsSubArrayCase`，以及 `fixtures/is_sub_array_cases.json`
2. 删除 `TestFindTerminalToolCallSessions`、`terminalToolCallCase`、`loadTerminalToolCallCases`、`findTerminalToolCallCase`，以及 `fixtures/terminal_tool_call_cases.json`——其覆盖的场景已由 Task 6 的三个用例与 `find_redundant_sessions_cases.json` 中的合并用例承接
3. 把 `TestFindRedundantSessionsWithMerge` 的调用改为 `cron.FindRedundantSessions(sessions, nil)`，读取 `result.MergeMapping` 的断言逻辑不变

> Task 3 的 `terminal_tool_call_two_children_same_parent` 场景在新入口下等价为：同组内两个前缀成员并入同一 keeper，`MergeMapping` 累积三个 tool ID。把它作为用例 `two_prefix_members_same_keeper` 迁入 `find_redundant_sessions_cases.json`，并加入 `TestFindRedundantSessionsWithMerge` 的 `testCases`：
>
> ```json
>     {
>         "name": "two_prefix_members_same_keeper",
>         "description": "Two redundant members merging into the same keeper must accumulate, not overwrite, their ToolIDs.",
>         "sessions": [
>             {"id": 1, "message_ids": [1, 2, 3, 4, 5, 6], "tool_ids": [100]},
>             {"id": 2, "message_ids": [1, 2, 3], "tool_ids": [20]},
>             {"id": 3, "message_ids": [1, 2, 3, 4], "tool_ids": [30]}
>         ],
>         "expected_redundant_ids": [2, 3]
>     }
> ```
>
> 在 `TestFindRedundantSessionsWithMerge` 的 `testCases` 中加入 `{name: "two_prefix_members_same_keeper", expectedMergedToolIDs: map[uint][]uint{1: {20, 30, 100}}}`，并把该用例名加入 `TestFindRedundantSessions` 的 `caseNames`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/session_dedup/... -v`
Expected: 编译失败，`undefined: cron.FindTerminalToolCallSessions` 等——说明实现侧仍有待删除的死代码

- [ ] **Step 3: 删除实现侧死代码**

从 `internal/cron/session_dedup.go` 删除：

- `IsSubArray`（导出，滑动窗口子数组匹配）
- `isEqualSlice`
- `isSessionRedundant`
- `findParentSessionID`
- `FindTerminalToolCallSessions`
- `processTerminalToolCallSession`
- `mergeToolIDs`（其唯一剩余调用点随 `deduplicate` 的二次合并一起消失；若 `mergeToolIDsIntoMapping` 内部仍需要则保留，二者择一）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/session_dedup/... -v`
Expected: 全部 PASS

- [ ] **Step 5: 确认无残留引用与行数收敛**

```bash
grep -rn "IsSubArray\|FindTerminalToolCallSessions\|findParentSessionID\|isEqualSlice" --include="*.go" . | grep -v "^./.worktrees/"
wc -l internal/cron/session_dedup.go
```

Expected: grep 无输出；行数 < 300（改造前 583）

- [ ] **Step 6: 跑全量校验并提交**

```bash
go test -count=1 ./test/unit/... && make lint
git add internal/cron/session_dedup.go test/unit/session_dedup/
git commit -m "refactor(cron): drop subarray matching dead code and stale fixtures"
```

## Task 8: 生产验证

**Files:** 无代码改动

**Interfaces:**
- Consumes: 已部署的 PR1 + PR2
- Produces: 验证结论

- [ ] **Step 1: 部署后手动触发一次并读取审计**

服务提供手动触发能力（`Cron.Trigger`）。触发后查询审计记录：

```sql
SET statement_timeout = '30000ms';
SELECT started_at, duration_ms, status, trigger_source,
       metadata ->> 'checked_sessions_count' AS checked,
       metadata ->> 'deduped_sessions_count' AS deduped
FROM cron_call_audits
WHERE cron_name = 'SessionDeduplicateCron'
ORDER BY id DESC
LIMIT 10;
```

- [ ] **Step 2: 比对基线**

| 指标 | 改造前基线 | 期望 | 不达标处置 |
|---|---|---|---|
| `duration_ms` avg | 1586 | 150~250 | 300ms 以上则用 fgprof 定位 |
| `status` | success | success | 失败则查 CLS 日志 |
| `deduped_sessions_count` | 0~94/次 | **同量级** | **异常升高即回滚**：说明前缀判定误删 |

- [ ] **Step 3: 确认无孤儿 tool 引用**

```sql
SET statement_timeout = '60000ms';
WITH refs AS (
    SELECT DISTINCT (t.elem)::bigint AS tool_id
    FROM sessions s, LATERAL jsonb_array_elements_text(s.tool_ids::jsonb) AS t(elem)
    WHERE s.deleted_at = 0
)
SELECT count(*) AS dangling_tool_refs
FROM refs r
LEFT JOIN tools tl ON tl.id = r.tool_id AND tl.deleted_at = 0
WHERE tl.id IS NULL;
```

Expected: `0`。非 0 说明 ToolIDs 合并丢失了引用，需排查 `ApplyMergeResult`。

- [ ] **Step 4: 沉淀经验**

把可复用结论记入工程记忆：分组前提（同对话共享首 message ID 源于 checksum 复用）、SQL 下推为最大收益点、语义等价性的 SQL 验证方法（`starts_with(long, left(short,-1) || ',')` 做前缀判定、`btrim(txt,'[]')` + `strpos` 做子数组判定）。

---

## 自审记录

**Spec 覆盖检查**

| Spec 章节 | 对应 Task |
|---|---|
| 4.1.1 SQL 下推 | Task 1 |
| 4.1.2 纯函数签名 | Task 2 |
| 4.1.3 缺陷 A（MergeMapping 覆盖） | Task 2 实现 + Task 3 回归用例 |
| 4.1.4 缺陷 B（写回原子性） | Task 4 |
| 4.2.1 对话组 | Task 5 |
| 4.2.2 组内决策 + 选项 X | Task 5（分组/前缀）+ Task 6（terminal 规则/absorbed 保护） |
| 4.2.3 不引入 trie | 无 Task（明确不做，`resolveGroup` 用线性 keeper 扫描） |
| 4.2.4 删除死代码 | Task 7 |
| 5.1 fixture 变更与新增用例 | Task 5 Step 1-2、Task 6 Step 1、Task 7 Step 1 |
| 5.2 SQL 谓词验证 | Task 1 Step 6 |
| 5.3 语义等价性 | 已在设计阶段完成，无需 Task |
| 5.4 生产回归基线 | Task 8 |

**类型一致性检查**

- `FilterTerminalToolCallIDs(db, ids) ([]uint, error)`：Task 1 定义，Task 2 `loadTerminalToolCallMsgIDs` 消费 ✓
- `FindTerminalToolCallSessions(sessions, terminalMsgIDs, excludeIDs)`：Task 2 定义并被 Task 2 `deduplicate` 与 Task 3 用例消费，Task 7 删除 ✓
- `ApplyMergeResult(db, sessionDAO, result) (int, error)`：Task 4 定义，Task 4 `deduplicate` 消费，PR2 不改动 ✓
- `isPrefix`、`groupByFirstMessageID`、`resolveGroup(entries, result, absorbed)`：Task 5 定义，Task 6 `FindRedundantSessions` 与 `applyTerminalRule` 消费 ✓
- `FindRedundantSessions(sessions, terminalMsgIDs)`：Task 6 变更签名，同 Task 内更新全部调用点与测试 ✓
- `MergeResult`/`sessionEntry`/`mergeToolIDsIntoMapping`：既有类型，未改变字段 ✓

**已知过渡态**（有意为之，两个 PR 连续合入）

- `FindTerminalToolCallSessions` 在 PR1 存在、PR2 删除
- `FindRedundantSessionsWithMerge` 在 PR1 保持旧签名、Task 5 改内核、Task 6 合并入口后删除
