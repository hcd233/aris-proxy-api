# Session 前缀去重实时化 + Terminal 终态清理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 前缀去重从每小时 cron 迁移到 session 插入后实时执行（独立短事务 + FOR UPDATE）；terminal tool_call 清理保留为改名后的 `SessionTerminalCleanupCron`（24h 窗口扫描）。

**Architecture:** 算法（`FindRedundantSessions` 纯前缀规则 + `ApplyMergeResult`）整体提取到 `internal/infrastructure/repository/session_dedup.go`；`runMessageStoreTask` 主事务提交后在同一协程池 goroutine 内开独立短事务按首条消息 ID 锁组去重；cron 重写为 24h 窗口终态扫描。表达式索引生产手工 CONCURRENTLY 建，`created_at` 索引走 AutoMigrate。

**Tech Stack:** Go 1.x + GORM (PostgreSQL, jsonb 强转) + robfig/cron + samber/lo + sqlite 内存库（单测）+ sonic

**Spec:** `docs/superpowers/specs/2026-08-29-session-dedup-insert-time-design.md`

## Global Constraints

- 分支：直接在 `master` 开发（用户既定偏好，绕过 worktree 规范），每个 Task 一次提交。
- 编辑任何 Go 文件前先跑 `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <文件路径>` 并读完整输出，不得截断。
- 所有 shell 命令加 `rtk` 前缀（`rtk go test`、`rtk git add` 等）。
- 业务错误统一走 `internal/common/ierr`（禁 `errors.Join`）；DTO 层禁 `any`/`interface{}`。
- `message_ids` 是 JSON 文本列：SQL 里必须 `::jsonb` 强转；禁用裸 `?` jsonb 操作符（gorm 占位符冲突，fix #59）；禁 `ANY(sessions.message_ids)`（fix #58）。
- GORM `Updates(map)` 不触发 `serializer:json`：tool_ids 回写必须显式 `sonic.MarshalString`（think-extract 踩坑，`applyMergeInTx` 单点保留）。
- 表达式 `message_ids::jsonb->>0` 与生产索引 `idx_sessions_first_msg` 的索引表达式必须逐字一致。
- 测试运行：`rtk go test ./test/unit/...`；全量验证 `rtk go test ./...`。

---

### Task 1: 提取前缀去重算法到 repository 包

**Files:**
- Create: `internal/infrastructure/repository/session_dedup.go`
- Modify: `internal/cron/session_dedup.go`（瘦身为纯前缀去重 cron 壳，保持编译）
- Modify: `test/unit/session_dedup/session_dedup_test.go`
- Modify: `test/unit/session_dedup/apply_merge_result_test.go`
- Modify: `test/unit/session_dedup/fixtures/find_redundant_sessions_cases.json`

**Interfaces:**
- Produces: `repository.FindRedundantSessions(sessions []*dbmodel.Session) MergeResult`（无 terminalMsgIDs 参数）；`repository.ApplyMergeResult(db *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error)`；`repository.MergeResult{RedundantIDs []uint; MergeMapping map[uint]map[uint]struct{}}`；包内未导出 `applyMergeInTx(tx *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error)`（Task 3 复用）。
- 本 Task 后 cron 临时为"仅前缀"状态（terminal 清理由 Task 4 重写回归），同 PR 内合并，无中间发布。

- [ ] **Step 1: 跑 use-modern-go list**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/repository/session_dedup.go
```

- [ ] **Step 2: 创建 `internal/infrastructure/repository/session_dedup.go`**

从 `internal/cron/session_dedup.go` 搬运并做三处修改：`FindRedundantSessions` 去掉 `terminalMsgIDs` 参数并删除 `applyTerminalRule`；`resolveGroup` 去掉 `absorbed` 参数；`ApplyMergeResult` 拆出 `applyMergeInTx`。完整内容：

```go
// Package repository Session 前缀去重算法与写回
//
// 自 internal/cron/session_dedup.go 提取（2026-08-29）：前缀去重在 session 插入
// 事务提交后实时执行（internal/infrastructure/pool/store_pool.go），cron 仅承担
// terminal 终态清理。
//
//	author centonhuang
//	update 2026-08-29 10:00:00
package repository

import (
	"cmp"
	"slices"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

// MergeResult 表示Session去重后的合并结果
//
//	@author centonhuang
//	@update 2026-03-30 10:00:00
type MergeResult struct {
	// RedundantIDs 需要删除的Session ID列表
	RedundantIDs []uint
	// MergeMapping 长Session ID -> 需要合并的ToolIDs（来自被删除的短Session）
	MergeMapping map[uint]map[uint]struct{}
}

// sessionEntry 用于表示Session在去重过程中的内部数据结构
//
//	@author centonhuang
//	@update 2026-06-04 10:00:00
type sessionEntry struct {
	id         uint
	messageIDs []uint
	toolIDs    []uint
}

// FindRedundantSessions 查找冗余 Session 并给出 ToolIDs 合并映射（纯前缀规则）
//
// 算法：
//
//  1. 按首个 message ID 分组（同一对话的快照集合），组内按 MessageIDs 长度降序、ID 升序排列
//
//  2. 组内扫描并维护 keeper 列表：成员若是某个 keeper 的前缀则判为冗余，
//     ToolIDs 并入首个匹配的 keeper；否则自身成为新 keeper（处理对话分叉）
//
//     同一对话的所有快照必然共享首个 message ID：每次请求都把完整对话历史按 checksum
//     去重后落库，历史消息复用同一行，故第 k 轮的 MessageIDs 是第 k+1 轮的前缀。
//     跨组不可能存在冗余关系，分组把两两比较从 O(N²) 降到 Σ(组内²)。
//
//	MessageIDs 为空的 session 被跳过，不参与去重。
//
//	@param sessions []*dbmodel.Session
//	@return MergeResult 包含需要删除的 Session ID 和 ToolIDs 合并映射
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func FindRedundantSessions(sessions []*dbmodel.Session) MergeResult {
	result := MergeResult{
		RedundantIDs: make([]uint, 0),
		MergeMapping: make(map[uint]map[uint]struct{}),
	}

	groups := groupByFirstMessageID(sessions)
	for _, entries := range groups {
		resolveGroup(entries, &result)
	}

	return result
}

// groupByFirstMessageID 按首个 message ID 将 session 分组，组内按 MessageIDs 长度降序、ID 升序排列
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
			return cmp.Compare(a.id, b.id)
		})
	}

	return groups
}

// resolveGroup 处理单个对话组：前缀成员判为冗余并把 ToolIDs 并入首个匹配的 keeper
//
//	keepers 按加入顺序（即长度降序、ID 升序）遍历，故「首个匹配的 keeper」
//	是最长且 ID 最小的容器，与保留较早 Session 的既有语义一致。
//
//	@param entries []sessionEntry 组内条目，已按长度降序、ID 升序排列
//	@param result *MergeResult
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func resolveGroup(entries []sessionEntry, result *MergeResult) {
	keepers := make([]sessionEntry, 0, len(entries))

	for _, e := range entries {
		container, found := lo.Find(keepers, func(k sessionEntry) bool {
			return isPrefix(e.messageIDs, k.messageIDs)
		})
		if !found {
			keepers = append(keepers, e)
			continue
		}

		result.RedundantIDs = append(result.RedundantIDs, e.id)
		mergeToolIDsIntoMapping(result.MergeMapping, container.id, container.toolIDs, e.toolIDs)
	}
}

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

// mergeToolIDsIntoMapping 将 target 和 source 的 ToolIDs 合并到 mapping 中指定 targetID 的条目
//
//	@param mapping map[uint]map[uint]struct{}
//	@param targetID uint
//	@param targetToolIDs []uint
//	@param sourceToolIDs []uint
//	@author centonhuang
//	@update 2026-06-04 10:00:00
func mergeToolIDsIntoMapping(mapping map[uint]map[uint]struct{}, targetID uint, targetToolIDs, sourceToolIDs []uint) {
	if len(targetToolIDs) == 0 && len(sourceToolIDs) == 0 {
		return
	}
	if mapping[targetID] == nil {
		mapping[targetID] = make(map[uint]struct{})
	}
	for _, tid := range targetToolIDs {
		mapping[targetID][tid] = struct{}{}
	}
	for _, tid := range sourceToolIDs {
		mapping[targetID][tid] = struct{}{}
	}
}

// ApplyMergeResult 在单个事务内写回 ToolIDs 合并结果并软删冗余 Session
//
// 原子性是必需的：若 tool_ids 更新失败却仍执行删除，被删 session 的 tool 引用
// 会永久丢失。任一步骤失败即整体回滚。
//
//	@param db *gorm.DB
//	@param sessionDAO *dao.SessionDAO
//	@param result MergeResult
//	@return int 成功合并 ToolIDs 的 Session 数
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func ApplyMergeResult(db *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error) {
	var mergedCount int
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		mergedCount, err = applyMergeInTx(tx, sessionDAO, result)
		return err
	})
	if err != nil {
		return 0, err
	}
	return mergedCount, nil
}

// applyMergeInTx 在已开启的事务内执行合并写回，不自行开启事务，
// 供 ApplyMergeResult（自管事务）与 DeduplicateSessionGroup（复用外层事务）共用。
//
// tool_ids 列为 text 类型（GORM serializer:json），Updates(map) 不触发序列化，
// 必须显式 sonic.MarshalString 后存 JSON 字符串。
//
//	@param tx *gorm.DB
//	@param sessionDAO *dao.SessionDAO
//	@param result MergeResult
//	@return int 成功合并 ToolIDs 的 Session 数
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func applyMergeInTx(tx *gorm.DB, sessionDAO *dao.SessionDAO, result MergeResult) (int, error) {
	mergedCount := 0
	for sessionID, toolIDSet := range result.MergeMapping {
		if len(toolIDSet) == 0 {
			continue
		}

		// 将集合转换为排序后的切片，保证写入内容稳定
		mergedToolIDs := lo.Keys(toolIDSet)
		slices.Sort(mergedToolIDs)

		toolIDsJSON, err := sonic.MarshalString(mergedToolIDs)
		if err != nil {
			return 0, err
		}
		if err := sessionDAO.Update(tx, &dbmodel.Session{ID: sessionID}, map[string]any{
			constant.FieldToolIDs: toolIDsJSON,
		}); err != nil {
			return 0, err
		}
		mergedCount++
	}

	if err := sessionDAO.BatchDeleteByField(tx, constant.WhereFieldID, result.RedundantIDs); err != nil {
		return 0, err
	}

	return mergedCount, nil
}
```

- [ ] **Step 3: 瘦身 `internal/cron/session_dedup.go`**

改动点（保持 cron 壳与 Start/Stop/StopGracefully/Trigger 不动）：
1. struct 删除 `messageDAO *dao.MessageDAO` 字段，`NewSessionDeduplicateCron` 删除 `messageDAO: dao.GetMessageDAO(),` 行；
2. `deduplicate` 删除 `terminalMsgIDs` 段与 `loadTerminalToolCallMsgIDs` 整个方法，`FindRedundantSessions`/`ApplyMergeResult` 改调 `repository` 包：

```go
	mergeResult := repository.FindRedundantSessions(sessions)

	if len(mergeResult.RedundantIDs) == 0 {
		log.Info("[SessionDeduplicateCron] No redundant sessions found", zap.Int("total", len(sessions)))
		return &commonmodel.CronCallAuditMetadata{
			CheckedSessions: checkedCount,
		}, nil
	}

	mergedCount, err := repository.ApplyMergeResult(db, c.sessionDAO, mergeResult)
```

3. import 增加 `"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"`，删除不再使用的 `cmp`/`slices`/`sonic`/`lo`；
4. 文件头注释改为「Session前缀去重定时任务（terminal 清理见 session_terminal_cleanup.go，Task 4 落地）」。

- [ ] **Step 4: 迁移测试到 repository 包**

`test/unit/session_dedup/session_dedup_test.go`：
1. import 中 `"github.com/hcd233/aris-proxy-api/internal/cron"` 改为 `repository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"`；
2. `cron.FindRedundantSessions(sessions, nil)` → `repository.FindRedundantSessions(sessions)`；`cron.FindRedundantSessions(sessions, tc.TerminalMsgIDs)` → `repository.FindRedundantSessions(sessions)`；
3. 删除 struct 字段 `TerminalMsgIDs []uint` 及其注释；
4. `caseNames` 删除 `"group_keeper_protected_without_tools"`、`"forked_keeper_not_protected"`、`"singleton_terminal_tool_call"` 三项；
5. 整体删除 `TestMergeTargetProtectedFromTerminalRule` 函数（含全部注释）。

`test/unit/session_dedup/apply_merge_result_test.go`：import `cron` → `repository`（同上别名），`cron.MergeResult` → `repository.MergeResult`，`cron.ApplyMergeResult` → `repository.ApplyMergeResult`。

`test/unit/session_dedup/fixtures/find_redundant_sessions_cases.json`：删除 name 为 `group_keeper_protected_without_tools`、`forked_keeper_not_protected`、`singleton_terminal_tool_call` 的三个 case 对象（含其 `terminal_msg_ids` 字段）。

- [ ] **Step 5: 跑测试验证**

```bash
rtk go test ./test/unit/session_dedup/... -v
```

Expected: 全部 PASS（迁移后的前缀用例 + ApplyMergeResult 两个 sqlite 用例）。

- [ ] **Step 6: 全量编译与单测**

```bash
rtk go build ./... && rtk go test ./test/unit/...
```

Expected: 编译通过、单测全绿。

- [ ] **Step 7: Commit**

```bash
rtk git add internal/infrastructure/repository/session_dedup.go internal/cron/session_dedup.go test/unit/session_dedup/
rtk git commit -m "refactor(session): 提取前缀去重算法到 repository 包，移除 terminal 规则（迁移至独立 cron）"
```

---

### Task 2: SessionDAO 组查询与窗口查询 + SQL 护栏

**Files:**
- Modify: `internal/infrastructure/database/dao/session.go`
- Modify: `internal/common/constant/sql.go`
- Test: `test/unit/session_dedup/dao_group_query_test.go`（Create）

**Interfaces:**
- Consumes: `constant.SessionRepoFieldsDedup = []string{FieldID, FieldMessageIDs, FieldToolIDs}`（已存在）
- Produces: `(*SessionDAO).FindGroupForUpdate(db *gorm.DB, firstMessageID uint) ([]*dbmodel.Session, error)`；`(*SessionDAO).GroupForUpdateQuery(db *gorm.DB, firstMessageID uint) *gorm.DB`（DryRun 护栏用）；`(*SessionDAO).FindCreatedSince(db *gorm.DB, since time.Time) ([]*dbmodel.Session, error)`；`(*SessionDAO).CreatedSinceQuery(db *gorm.DB, since time.Time) *gorm.DB`；`constant.SessionRepoFieldsTerminalScan = []string{FieldID, FieldMessageIDs}`；`constant.SessionFirstMessageIDCondition = "message_ids::jsonb->>0 = ?"`（Task 3/4 依赖）。

- [ ] **Step 1: 跑 use-modern-go list**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/database/dao/session.go
```

- [ ] **Step 2: 写失败测试 `test/unit/session_dedup/dao_group_query_test.go`**

```go
// Package session_dedup SessionDAO 组查询与窗口查询的 SQL 形态护栏
//
// 两个查询都依赖 PostgreSQL 专有语法（::jsonb 强转），sqlite 无法真实执行，
// 故用 DryRun 钉住生成的 SQL 形态，行为由 e2e 覆盖
// （同 MessageDAO.FilterTerminalToolCallIDs 的先例）。
//
// 关键不变量：SessionFirstMessageIDCondition 的表达式必须与生产索引
// idx_sessions_first_msg 的索引表达式 (message_ids::jsonb->>0) 逐字一致，
// 否则插入路径的组查询退化为全表 JSON 扫描。
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
package session_dedup

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newSQLShapeDB 打开 sqlite DryRun 会话：只构建 SQL 不执行
func newSQLShapeDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	return db.Session(&gorm.Session{DryRun: true})
}

// assertSQLContains 断言 DryRun 生成的 SQL 包含全部期望片段
func assertSQLContains(t *testing.T, sqlText string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(sqlText, want) {
			t.Errorf("generated SQL missing %q\ngot: %s", want, sqlText)
		}
	}
}

// TestGroupForUpdateQuery_SQLShape 钉住组查询形态：
// ::jsonb 强转（索引命中前提）、FOR UPDATE（同组并发串行化）、deleted_at = 0（活跃行）。
func TestGroupForUpdateQuery_SQLShape(t *testing.T) {
	t.Parallel()
	dry := newSQLShapeDB(t)

	var out []*dbmodel.Session
	q := dao.GetSessionDAO().GroupForUpdateQuery(dry, 42).Find(&out)

	assertSQLContains(t, q.Statement.SQL.String(), []string{
		"message_ids::jsonb->>0",
		"FOR UPDATE",
		"deleted_at = 0",
	})
}

// TestCreatedSinceQuery_SQLShape 钉住窗口扫描形态：
// created_at >= 窗口条件（依赖 idx_sessions_created_at）、
// 列收窄为 id + message_ids（不含 tool_ids）。
func TestCreatedSinceQuery_SQLShape(t *testing.T) {
	t.Parallel()
	dry := newSQLShapeDB(t)

	var out []*dbmodel.Session
	q := dao.GetSessionDAO().CreatedSinceQuery(dry, time.Unix(0, 0).UTC()).Find(&out)

	sqlText := q.Statement.SQL.String()
	assertSQLContains(t, sqlText, []string{"created_at >=", "deleted_at = 0"})
	if strings.Contains(sqlText, "tool_ids") {
		t.Errorf("window scan must not select tool_ids\ngot: %s", sqlText)
	}
}

// TestSessionFirstMessageIDCondition_MatchesIndexExpression 钉住常量表达式与
// 生产索引表达式逐字一致（去掉括号与占位符后比较）。
func TestSessionFirstMessageIDCondition_MatchesIndexExpression(t *testing.T) {
	t.Parallel()
	if constant.SessionFirstMessageIDCondition != "message_ids::jsonb->>0 = ?" {
		t.Errorf("SessionFirstMessageIDCondition = %q, must be %q to match idx_sessions_first_msg",
			constant.SessionFirstMessageIDCondition, "message_ids::jsonb->>0 = ?")
	}
}
```

注意：import 需补 `"time"` 与 `"github.com/hcd233/aris-proxy-api/internal/common/constant"`。

- [ ] **Step 3: 跑测试确认失败**

```bash
rtk go test ./test/unit/session_dedup/ -run "SQLShape|MatchesIndex" -v
```

Expected: FAIL（`GroupForUpdateQuery` / `CreatedSinceQuery` / `SessionFirstMessageIDCondition` 未定义）。

- [ ] **Step 4: 实现 DAO 方法与常量**

`internal/common/constant/sql.go` 在 `SessionRepoFieldsDedup`（约 124 行）旁新增：

```go
	// SessionRepoFieldsTerminalScan 终态清理窗口扫描的查询列（不需要 tool_ids）
	SessionRepoFieldsTerminalScan = []string{FieldID, FieldMessageIDs}

	// SessionFirstMessageIDCondition 按首条消息 ID 查同组会话的条件。
	// 表达式必须与生产索引 idx_sessions_first_msg 的索引表达式
	// ((message_ids::jsonb->>0) WHERE deleted_at = 0) 逐字一致，改动即退化全表扫描。
	SessionFirstMessageIDCondition = "message_ids::jsonb->>0 = ?"
```

`internal/infrastructure/database/dao/session.go` 新增（import 补 `strconv`、`time`、`gorm.io/gorm/clause`）：

```go
// GroupForUpdateQuery 构造同组会话的行锁查询（不执行）
//
//	首条消息 ID 是同一对话快照集合的分组键：历史消息按 checksum 去重复用行，
//	第 k 轮的 MessageIDs 是第 k+1 轮的前缀。查询结果不含 MessageIDs 为空的行
//	（'[]' 的 ->>0 为 NULL），与去重算法跳过空 MessageIDs 的语义一致。
//
//	@param db *gorm.DB
//	@param firstMessageID uint 组键（首条消息 ID）
//	@return *gorm.DB
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) GroupForUpdateQuery(db *gorm.DB, firstMessageID uint) *gorm.DB {
	return db.Select(constant.SessionRepoFieldsDedup).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.SessionFirstMessageIDCondition, strconv.FormatUint(uint64(firstMessageID), 10)).
		Clauses(clause.Locking{Strength: "UPDATE"})
}

// FindGroupForUpdate 锁定并返回同组（首条消息 ID 相同）的全部活跃会话
//
//	FOR UPDATE 串行化同组并发写入（插入路径去重与多副本并发）。
//	::jsonb 为 PG 专有语法，sqlite 不可用，行为由 e2e 覆盖。
//
//	@param db *gorm.DB
//	@param firstMessageID uint 组键
//	@return []*dbmodel.Session
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) FindGroupForUpdate(db *gorm.DB, firstMessageID uint) ([]*dbmodel.Session, error) {
	var models []*dbmodel.Session
	if err := dao.GroupForUpdateQuery(db, firstMessageID).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// CreatedSinceQuery 构造 24h 窗口扫描查询（不执行）
//
//	@param db *gorm.DB
//	@param since time.Time 创建时间下界
//	@return *gorm.DB
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) CreatedSinceQuery(db *gorm.DB, since time.Time) *gorm.DB {
	return db.Select(constant.SessionRepoFieldsTerminalScan).
		Where(constant.DBConditionDeletedAtZero).
		Where("created_at >= ?", since)
}

// FindCreatedSince 查询指定时间之后创建的活跃会话（终态清理扫描窗口）
//
//	依赖 idx_sessions_created_at（Session 模型重声明 CreatedAt 挂 tag，AutoMigrate 自动建）。
//
//	@param db *gorm.DB
//	@param since time.Time 创建时间下界
//	@return []*dbmodel.Session
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) FindCreatedSince(db *gorm.DB, since time.Time) ([]*dbmodel.Session, error) {
	var models []*dbmodel.Session
	if err := dao.CreatedSinceQuery(db, since).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}
```

- [ ] **Step 5: 跑测试确认通过**

```bash
rtk go test ./test/unit/session_dedup/ -run "SQLShape|MatchesIndex" -v
```

Expected: 3 个用例 PASS。

- [ ] **Step 6: Commit**

```bash
rtk git add internal/infrastructure/database/dao/session.go internal/common/constant/sql.go test/unit/session_dedup/dao_group_query_test.go
rtk git commit -m "feat(session-dao): 新增同组行锁查询与 24h 窗口扫描查询及 SQL 形态护栏"
```

---

### Task 3: 插入路径实时去重（repository 入口 + pool 接线）

**Files:**
- Modify: `internal/infrastructure/repository/session_dedup.go`（追加 `DeduplicateSessionGroup`）
- Modify: `internal/infrastructure/pool/store_pool.go`

**Interfaces:**
- Consumes: Task 1 的 `FindRedundantSessions` / `applyMergeInTx`；Task 2 的 `FindGroupForUpdate`。
- Produces: `repository.DeduplicateSessionGroup(db *gorm.DB, firstMessageID uint) (int, error)`——返回软删的冗余会话数。

- [ ] **Step 1: 跑 use-modern-go list**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/pool/store_pool.go
```

- [ ] **Step 2: 在 `internal/infrastructure/repository/session_dedup.go` 末尾追加**

```go
// DeduplicateSessionGroup 在单个短事务内锁定同组会话并执行前缀去重
//
// 由 session 插入路径在主事务提交后调用（store_pool）：组行 FOR UPDATE 串行化
// 同组并发写入；空组竞态下并发首插各留一份，下次同组插入自愈。
// ::jsonb 为 PG 专有语法，sqlite 不可用，行为由 e2e 覆盖。
//
//	@param db *gorm.DB
//	@param firstMessageID uint 组键（新会话的首条消息 ID）
//	@return int 软删的冗余会话数
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func DeduplicateSessionGroup(db *gorm.DB, firstMessageID uint) (int, error) {
	var deduped int
	err := db.Transaction(func(tx *gorm.DB) error {
		sessions, err := dao.GetSessionDAO().FindGroupForUpdate(tx, firstMessageID)
		if err != nil {
			return err
		}

		result := FindRedundantSessions(sessions)
		if len(result.RedundantIDs) == 0 {
			return nil
		}
		deduped = len(result.RedundantIDs)

		_, err = applyMergeInTx(tx, dao.GetSessionDAO(), result)
		return err
	})
	if err != nil {
		return 0, err
	}
	return deduped, nil
}
```

- [ ] **Step 3: 修改 `internal/infrastructure/pool/store_pool.go`**

1. `runMessageStoreTask` 事务成功分支（`log.Info("[StorePool] Messages stored successfully")` 之后）追加去重调用：

```go
	err := db.Transaction(func(tx *gorm.DB) error {
		// ……现有消息/工具/session 写入逻辑保持不变……
	})
	if err != nil {
		log.Error("[StorePool] Transaction failed", zap.Error(err))
		return
	}
	log.Info("[StorePool] Messages stored successfully")
	pm.deduplicateNewSession(task.Ctx, log, messageIDs)
```

2. 文件末尾追加方法（import 补 `"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"`）：

```go
// deduplicateNewSession 主事务提交后对同组会话执行前缀去重（best-effort）
//
// 主数据（messages/tools/session）已提交，去重失败仅记日志、不重试：
// 残留重复由该对话下次插入时的去重自愈。task.Ctx 经 CopyContextValues
// 已脱离请求生命周期，事务提交后使用安全。
//
//	@receiver pm *PoolManager
//	@param ctx context.Context 脱离请求生命周期的存储上下文
//	@param log *zap.Logger
//	@param messageIDs []uint 新会话的消息 ID 列表，空则短路
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (pm *PoolManager) deduplicateNewSession(ctx context.Context, log *zap.Logger, messageIDs []uint) {
	if len(messageIDs) == 0 {
		return
	}
	deduped, err := repository.DeduplicateSessionGroup(pm.db.WithContext(ctx), messageIDs[0])
	if err != nil {
		log.Error("[StorePool] Session deduplication failed, will self-heal on next insert", zap.Error(err))
		return
	}
	if deduped > 0 {
		log.Info("[StorePool] Session deduplicated", zap.Int("deduped", deduped))
	}
}
```

3. import 补 `"context"`（若无）。

- [ ] **Step 4: 编译与全量单测**

```bash
rtk go build ./... && rtk go test ./test/unit/...
```

Expected: 通过。说明：`DeduplicateSessionGroup` 依赖 `::jsonb` 无法在 sqlite 单测中执行，行为由 Task 5 的 e2e 覆盖（同 `FilterTerminalToolCallIDs` 先例）。

- [ ] **Step 5: Commit**

```bash
rtk git add internal/infrastructure/repository/session_dedup.go internal/infrastructure/pool/store_pool.go
rtk git commit -m "feat(session): 插入事务提交后实时前缀去重（独立短事务 + FOR UPDATE）"
```

---

### Task 4: SessionTerminalCleanupCron 重写（重命名 + 24h 窗口 + created_at 索引）

**Files:**
- Create: `internal/cron/session_terminal_cleanup.go`
- Delete: `internal/cron/session_dedup.go`
- Modify: `internal/common/constant/session.go`
- Modify: `internal/common/constant/cron.go`
- Modify: `internal/config/config.go`
- Modify: `internal/cron/cron.go`（注册表）
- Modify: `test/unit/cron/cron_test.go`（config 字段改名，编译级引用）
- Modify: `internal/infrastructure/database/model/session.go`（重声明 CreatedAt 挂索引）
- Test: `test/unit/session_dedup/terminal_cleanup_test.go`（Create）
- Test: `test/unit/db_index/db_index_test.go`（追加用例）
- Modify: `CONTEXT.md`（领域词条）

**Interfaces:**
- Consumes: Task 2 的 `FindCreatedSince`；`dao.GetMessageDAO().FilterTerminalToolCallIDs`（现有）。
- Produces: `cron.NewSessionTerminalCleanupCron(db *gorm.DB, cache *redis.Client) Cron`；常量 `constant.CronModuleSessionTerminalCleanup`、`constant.CronSpecSessionTerminalCleanup`、`constant.CronDescriptionSessionTerminalCleanup`；config 字段 `config.CronSessionTerminalCleanupEnabled`（key `cron.session.terminal_cleanup.enabled`）；包内纯函数 `pickTerminalStuckSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint) []uint`。

- [ ] **Step 1: 跑 use-modern-go list**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/cron/session_terminal_cleanup.go
```

- [ ] **Step 2: 写失败测试 `test/unit/session_dedup/terminal_cleanup_test.go`**

```go
// Package session_dedup SessionTerminalCleanupCron 纯函数用例
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
package session_dedup

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/cron"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

func TestPickTerminalStuckSessions(t *testing.T) {
	t.Parallel()

	sessions := []*dbmodel.Session{
		{MessageIDs: []uint{1, 2, 3}}, // 末条 3 命中 terminal -> 删除
		{MessageIDs: []uint{4, 5}},    // 末条 5 未命中 -> 保留
		{MessageIDs: []uint{}},         // 空 MessageIDs -> 跳过
	}
	sessions[0].ID = 11
	sessions[1].ID = 12
	sessions[2].ID = 13

	got := cron.PickTerminalStuckSessions(sessions, []uint{3, 99})
	if !slices.Equal(got, []uint{11}) {
		t.Errorf("PickTerminalStuckSessions() = %v, want [11]", got)
	}

	if got := cron.PickTerminalStuckSessions(sessions, nil); len(got) != 0 {
		t.Errorf("PickTerminalStuckSessions() with nil terminal = %v, want empty", got)
	}
}
```

注意：`PickTerminalStuckSessions` 需导出（外部测试包调用），实现时导出并在注释标明「导出以便外部测试包验证」。

- [ ] **Step 3: 跑测试确认失败**

```bash
rtk go test ./test/unit/session_dedup/ -run TestPickTerminalStuckSessions -v
```

Expected: FAIL（未定义）。

- [ ] **Step 4: 创建 `internal/cron/session_terminal_cleanup.go`，删除 `internal/cron/session_dedup.go`**

```go
// Package cron Session终态清理定时任务
//
// 扫描最近 24 小时内末条消息为 assistant+tool_calls（对话中断于工具调用处）的
// 会话并软删。前缀去重已迁移至 session 插入路径实时执行
// （internal/infrastructure/repository/session_dedup.go），本任务不再做前缀解析。
//
// 稳态语义与旧算法等价：旧实现中 absorbed 保护仅使 merge target 多存活一个
// cron 周期（下一轮成单例组后仍被删除）。
//
//	author centonhuang
//	update 2026-08-29 10:00:00
package cron

import (
	"context"
	"fmt"
	"time"

	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SessionTerminalCleanupCron Session终态清理定时任务，删除最近 24h 内中断于 assistant tool_call 的会话
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
type SessionTerminalCleanupCron struct {
	cron       *cron.Cron
	db         *gorm.DB
	locker     *lock.RedisLocker
	lockKey    string
	sessionDAO *dao.SessionDAO
	messageDAO *dao.MessageDAO
}

// NewSessionTerminalCleanupCron 创建Session终态清理定时任务
//
//	@return Cron
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func NewSessionTerminalCleanupCron(db *gorm.DB, cache *redis.Client) Cron {
	return &SessionTerminalCleanupCron{
		cron: cron.New(
			cron.WithLogger(newCronLoggerAdapter(constant.CronModuleSessionTerminalCleanup)),
		),
		db:         db,
		locker:     lock.NewLocker(cache),
		sessionDAO: dao.GetSessionDAO(),
		messageDAO: dao.GetMessageDAO(),
	}
}

// Stop 停止Session终态清理定时任务
//
//	@receiver c *SessionTerminalCleanupCron
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) Stop() {
	if c.cron != nil {
		ctx := c.cron.Stop()
		<-ctx.Done()
	}
}

// StopGracefully 仅停止调度，不等待运行中任务完成
//
//	@receiver c *SessionTerminalCleanupCron
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) StopGracefully() {
	if c.cron != nil {
		c.cron.Stop()
	}
}

// Start 启动Session终态清理定时任务
//
//	@receiver c *SessionTerminalCleanupCron
//	@param spec string cron 表达式
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) Start(spec string) error {
	c.lockKey = fmt.Sprintf(constant.CronLockKeyTemplate, constant.CronModuleSessionTerminalCleanup)
	entryID, err := c.cron.AddFunc(spec, wrapCronFunc(constant.CronModuleSessionTerminalCleanup, c.locker, c.lockKey, LockOptions{}, c.cleanup, constant.CronTriggerSourceScheduled))
	if err != nil {
		logger.Logger().Error("[SessionTerminalCleanupCron] Add func error", zap.Error(err))
		return err
	}

	logger.Logger().Info("[SessionTerminalCleanupCron] Add func success", zap.Int("entryID", int(entryID)))

	c.cron.Start()

	return nil
}

// Trigger 手动触发一次 Session 终态清理
//
//	@receiver c *SessionTerminalCleanupCron
//	@return bool
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) Trigger() bool {
	return TriggerWithLock(constant.CronModuleSessionTerminalCleanup, c.locker, c.lockKey, LockOptions{}, c.cleanup)
}

// TerminalCleanupWindow 终态清理扫描窗口（最近 24 小时）
const TerminalCleanupWindow = 24 * time.Hour

// cleanup 执行Session终态清理
//
//	@receiver c *SessionTerminalCleanupCron
//	@param ctx context.Context
//	@return *commonmodel.CronCallAuditMetadata
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (c *SessionTerminalCleanupCron) cleanup(ctx context.Context) (*commonmodel.CronCallAuditMetadata, error) {
	log := logger.WithCtx(ctx)
	db := c.db.WithContext(ctx)

	sessions, err := c.sessionDAO.FindCreatedSince(db, time.Now().UTC().Add(-TerminalCleanupWindow))
	if err != nil {
		log.Error("[SessionTerminalCleanupCron] Failed to load recent sessions", zap.Error(err))
		return nil, err
	}
	checkedCount := int64(len(sessions))

	lastMsgIDs := lo.FilterMap(sessions, func(s *dbmodel.Session, _ int) (uint, bool) {
		if len(s.MessageIDs) == 0 {
			return 0, false
		}
		return s.MessageIDs[len(s.MessageIDs)-1], true
	})
	terminalMsgIDs, err := c.messageDAO.FilterTerminalToolCallIDs(db, lo.Uniq(lastMsgIDs))
	if err != nil {
		log.Error("[SessionTerminalCleanupCron] Failed to filter terminal tool call message ids", zap.Error(err))
		return nil, err
	}

	victimIDs := PickTerminalStuckSessions(sessions, terminalMsgIDs)
	if len(victimIDs) == 0 {
		log.Info("[SessionTerminalCleanupCron] No terminal stuck sessions", zap.Int64("checked", checkedCount))
		return &commonmodel.CronCallAuditMetadata{
			CheckedSessions: checkedCount,
		}, nil
	}

	if err := c.sessionDAO.BatchDeleteByField(db, constant.WhereFieldID, victimIDs); err != nil {
		log.Error("[SessionTerminalCleanupCron] Failed to delete terminal stuck sessions", zap.Error(err))
		return nil, err
	}

	log.Info("[SessionTerminalCleanupCron] Terminal cleanup completed",
		zap.Int64("checked", checkedCount),
		zap.Int("deleted", len(victimIDs)))

	return &commonmodel.CronCallAuditMetadata{
		CheckedSessions: checkedCount,
		DedupedSessions: int64(len(victimIDs)),
	}, nil
}

// PickTerminalStuckSessions 取末条消息命中 terminalMsgIDs 的会话 ID
//
//	导出以便外部测试包验证。
//
//	@param sessions []*dbmodel.Session
//	@param terminalMsgIDs []uint 已由 SQL 判定为 assistant+tool_calls 的 message ID
//	@return []uint
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func PickTerminalStuckSessions(sessions []*dbmodel.Session, terminalMsgIDs []uint) []uint {
	terminalSet := lo.SliceToMap(terminalMsgIDs, func(id uint) (uint, struct{}) { return id, struct{}{} })
	return lo.FilterMap(sessions, func(s *dbmodel.Session, _ int) (uint, bool) {
		if len(s.MessageIDs) == 0 {
			return 0, false
		}
		if _, ok := terminalSet[s.MessageIDs[len(s.MessageIDs)-1]]; !ok {
			return 0, false
		}
		return s.ID, true
	})
}
```

- [ ] **Step 5: 常量、config、注册表、既有 cron 单测**

`internal/common/constant/session.go` 整体替换为：

```go
package constant

import "time"

const (
	SessionDetailCacheTTL = 60 * time.Minute

	CronModuleSessionTerminalCleanup = "SessionTerminalCleanupCron"

	CronSpecSessionTerminalCleanup = "0 * * * *"
)
```

`internal/common/constant/cron.go`：`CronDescriptionSessionDeduplicate` 替换为：

```go
	CronDescriptionSessionTerminalCleanup = "Scan sessions created in the last 24h interrupted at an assistant tool_call and remove them"
```

`internal/config/config.go` 三处：
1. struct 字段（原 `CronSessionDeduplicateEnabled bool` 及其注释，约 169-171 行）替换为：

```go
	// CronSessionTerminalCleanupEnabled bool 是否启用 Session 终态清理定时任务
	//
	//	@update 2026-08-29 10:00:00
	CronSessionTerminalCleanupEnabled bool
```

2. `config.SetDefault("cron.session.deduplicate.enabled", true)`（约 291 行）替换为：

```go
	config.SetDefault("cron.session.terminal_cleanup.enabled", true)
```

3. `CronSessionDeduplicateEnabled = config.GetBool("cron.session.deduplicate.enabled")`（约 366 行）替换为：

```go
	CronSessionTerminalCleanupEnabled = config.GetBool("cron.session.terminal_cleanup.enabled")
```

4. `test/unit/cron/cron_test.go` 中全部 `config.CronSessionDeduplicateEnabled`（9 处）替换为 `config.CronSessionTerminalCleanupEnabled`（编译级引用，用 replace_all）。
   注意：`test/unit/cron/cron_handler_test.go` 中的 `"SessionDeduplicateCron"` 字符串是 handler 层 mock 夹具的任意名字，与注册表无耦合，保持不动。

`internal/cron/cron.go` 注册表第一项（约 153-162 行）替换为：

```go
		{
			Name:        constant.CronModuleSessionTerminalCleanup,
			Type:        constant.CronTypeFunctional,
			Spec:        constant.CronSpecSessionTerminalCleanup,
			Description: constant.CronDescriptionSessionTerminalCleanup,
			Enabled:     func() bool { return config.CronSessionTerminalCleanupEnabled },
			Factory: func(db *gorm.DB, _ *pool.PoolManager, cache *redis.Client, _ conversation.ThinkExtractRepository) Cron {
				return NewSessionTerminalCleanupCron(db, cache)
			},
		},
```

- [ ] **Step 6: Session 模型重声明 CreatedAt 挂索引**

`internal/infrastructure/database/model/session.go` 的 `Session` struct 在 `BaseModel` 之后插入一行（`time` 已 import）：

```go
	// CreatedAt 重声明以为 sessions 单表挂索引（直接改 BaseModel 会波及全部继承表），
	// 覆盖终态清理 24h 窗口扫描与会话列表默认排序
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;index:idx_sessions_created_at;comment:创建时间"`
```

- [ ] **Step 7: 追加 db_index 用例**

`test/unit/db_index/db_index_test.go` 末尾追加：

```go
// TestSessionAutoMigrateCreatedAtIndex 验证 sessions 表 created_at 索引生成。
// 终态清理 cron 的 24h 窗口扫描（SessionDAO.FindCreatedSince）依赖它避免全表扫描。
func TestSessionAutoMigrateCreatedAtIndex(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Session{}); err != nil {
		t.Fatalf("failed to migrate sessions: %v", err)
	}

	const indexName = "idx_sessions_created_at"
	var indexSQL string
	if err := db.Raw(
		"SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = 'sessions' AND name = ?",
		indexName,
	).Scan(&indexSQL).Error; err != nil {
		t.Fatalf("query index %q: %v", indexName, err)
	}
	if indexSQL == "" {
		t.Fatalf("index %q missing", indexName)
	}
	gotCols := parseIndexColumns(t, indexSQL)
	if !equalStrings(gotCols, []string{"created_at"}) {
		t.Errorf("index %q columns = %v, want [created_at]", indexName, gotCols)
	}
}
```

- [ ] **Step 8: CONTEXT.md 领域词条**

在 `CONTEXT.md` 的 session 相关词汇区追加两条（找不到 session 小节时加在词汇表末尾）：

```markdown
- 前缀去重（insert-time dedup）：session 插入事务提交后，按首条消息 ID 锁定同组会话执行前缀规则——冗余快照实时软删、ToolIDs 并入 keeper（`repository.DeduplicateSessionGroup`）。
- 终态清理（terminal cleanup）：每小时扫描最近 24 小时内末条消息为 assistant+tool_calls（对话中断于工具调用处）的会话并删除（`SessionTerminalCleanupCron`）。
```

- [ ] **Step 9: 跑测试确认通过**

```bash
rtk go test ./test/unit/session_dedup/ ./test/unit/db_index/ -v && rtk go build ./...
```

Expected: 全部 PASS（含 Task 1/2 既有用例）、编译通过。

- [ ] **Step 10: Commit**

```bash
rtk git add internal/cron/ internal/common/constant/ internal/config/config.go internal/infrastructure/database/model/session.go test/unit/session_dedup/terminal_cleanup_test.go test/unit/db_index/db_index_test.go test/unit/cron/cron_test.go CONTEXT.md
rtk git commit -m "feat(cron): SessionDeduplicateCron 重命名为 SessionTerminalCleanupCron 并改为 24h 终态窗口扫描"
```

---

### Task 5: e2e 用例（部署后验证）

**Files:**
- Create: `test/e2e/session_dedup/session_dedup_test.go`

**Interfaces:**
- Consumes: `POST /api/openai/v1/chat/completions`（Bearer API_KEY）；`GET /api/web/v1/session/list?keyword=<marker>`（Bearer JWT_TOKEN，响应 `{ sessions, pageInfo }`，summary 字段 `messageCount`）。
- Produces: 部署验证入口 `TestE2E_SessionPrefixDedup_RealtimeAfterInsert`。

- [ ] **Step 1: 写测试**

```go
// Package session_dedup 验证 session 前缀去重在插入后实时生效。
//
// 环境变量（缺省 skip）：
//   - BASE_URL   API 根地址
//   - API_KEY    代理密钥（OpenAI 协议）
//   - JWT_TOKEN  管理员 JWT（session/list）
//
// 流程：同一对话连续两轮 chat completions（轮次1消息带随机标记）→
// 轮次1快照应被轮次2快照实时取代 → 以标记为 keyword 轮询 session/list，
// 最终只剩 1 条且 messageCount == 轮次2消息数（含 assistant 回复）。
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
package session_dedup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const e2eHTTPTimeout = 90 * time.Second

type sessionSummary struct {
	ID           uint `json:"id"`
	MessageCount int  `json:"messageCount"`
}
type listSessionsRsp struct {
	Sessions []sessionSummary `json:"sessions"`
}
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type chatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

func mustE2EEnv(t *testing.T) (baseURL, apiKey, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || apiKey == "" || jwtToken == "" {
		t.Skip("BASE_URL, API_KEY and JWT_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey, jwtToken
}

// postOnce 发送一轮非流式 chat completions 并返回 assistant 回复内容
func postOnce(t *testing.T, baseURL, apiKey string, messages []chatMessage) string {
	t.Helper()
	body, err := sonic.Marshal(map[string]any{
		"model":      "gpt-5.5",
		"messages":   messages,
		"stream":     false,
		"max_tokens": 10,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: e2eHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send chat completion: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chat completion status = %d, body: %s", resp.StatusCode, string(respBody))
	}
	var completion struct {
		Choices []chatChoice `json:"choices"`
	}
	if err := sonic.Unmarshal(respBody, &completion); err != nil {
		t.Fatalf("unmarshal completion: %v", err)
	}
	if len(completion.Choices) == 0 {
		t.Fatalf("no choices in response: %s", string(respBody))
	}
	return completion.Choices[0].Message.Content
}

// listSessionsByKeyword 以 keyword 过滤会话列表
func listSessionsByKeyword(t *testing.T, baseURL, jwtToken, keyword string) []sessionSummary {
	t.Helper()
	q := url.Values{}
	q.Set("page", "1")
	q.Set("pageSize", "20")
	q.Set("keyword", keyword)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		baseURL+"/api/web/v1/session/list?"+q.Encode(), http.NoBody)
	if err != nil {
		t.Fatalf("build list request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	client := &http.Client{Timeout: e2eHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send list request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read list response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session list status = %d, body: %s", resp.StatusCode, string(respBody))
	}
	var rsp listSessionsRsp
	if err := sonic.Unmarshal(respBody, &rsp); err != nil {
		t.Fatalf("unmarshal list rsp: %v", err)
	}
	return rsp.Sessions
}

func TestE2E_SessionPrefixDedup_RealtimeAfterInsert(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, jwtToken := mustE2EEnv(t)
	marker := fmt.Sprintf("dedup-e2e-%d", time.Now().UnixNano())

	turn1 := []chatMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Reply with exactly: OK " + marker},
	}
	assistantContent := postOnce(t, baseURL, apiKey, turn1)

	turn2 := append(append([]chatMessage{}, turn1...),
		chatMessage{Role: "assistant", Content: assistantContent},
		chatMessage{Role: "user", Content: "Thanks " + marker},
	)
	postOnce(t, baseURL, apiKey, turn2)

	// store 消息 = 请求消息 + assistant 回复
	expectedCount := len(turn2) + 1

	// 去重完成后 keyword 只命中轮次2快照；轮次1快照（messageCount 更小）应已软删
	deadline := time.Now().Add(90 * time.Second)
	for {
		sessions := listSessionsByKeyword(t, baseURL, jwtToken, marker)
		if len(sessions) == 1 && sessions[0].MessageCount == expectedCount {
			t.Logf("deduped: single session id=%d messageCount=%d", sessions[0].ID, sessions[0].MessageCount)
			return
		}
		if time.Now().After(deadline) {
			counts := make([]int, 0, len(sessions))
			for _, s := range sessions {
				counts = append(counts, s.MessageCount)
			}
			t.Fatalf("dedup not settled within 90s: sessions=%d want=1, messageCounts=%v want=[%d]",
				len(sessions), counts, expectedCount)
		}
		time.Sleep(3 * time.Second)
	}
}
```

- [ ] **Step 2: 编译确认**

```bash
rtk go build ./test/e2e/session_dedup/ && rtk go vet ./test/e2e/session_dedup/
```

Expected: 通过（无 BASE_URL 时测试在运行期 skip，不在编译期暴露）。

- [ ] **Step 3: Commit**

```bash
rtk git add test/e2e/session_dedup/
rtk git commit -m "test(e2e): session 前缀去重实时性验证用例"
```

---

### Task 6: 全量验证 + ponytail-review + 生产 runbook

**Files:**
- Modify: `docs/superpowers/plans/2026-08-29-session-dedup-insert-time.md`（本文件，勾选进度）

- [ ] **Step 1: 全量测试与 lint**

```bash
rtk go test ./...
golangci-lint run
```

Expected: 全绿（若 lint 命令不同，以 Makefile / `docs/agents/commands.md` 为准）。

- [ ] **Step 2: ponytail-review 审查 diff**

对 `git diff fe5a6c2a..HEAD`（或本次首个提交的父提交）运行 `ponytail-review`，逐条处理可删项（重点检查：Task 1 是否残留未用符号、Task 4 旧 cron 是否删净）。

- [ ] **Step 3: 沉淀 serena memory**

用 `serena_write_memory` 记录本次可复用经验（至少含：`applyMergeInTx` 拆分动机、DryRun SQL 护栏模式、`PickTerminalStuckSessions` 导出供测模式、cron_jobs 表旧模块名残留无害结论）。

- [ ] **Step 4: 生产 runbook（合并/推送前向用户逐步确认执行）**

按 `login-prod-server` skill SSH 到 `api.lvlvko.top`：

```sql
-- 1. 生产建表达式索引（必须先于代码部署；2.5k 行 CONCURRENTLY 秒级）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_first_msg
    ON sessions ((message_ids::jsonb->>0)) WHERE deleted_at = 0;
-- 验证命中（拿一条真实首条消息 ID）：
EXPLAIN SELECT id FROM sessions WHERE deleted_at = 0 AND (message_ids::jsonb->>0) = '<firstMsgID>' FOR UPDATE;
```

2. 手动触发一次现行去重，保证交接起点干净：
   `POST /api/web/v1/cron/trigger?name=SessionDeduplicateCron`（管理员 JWT）。
3. 推送 master：`rtk git push`，`gh run watch` 跟踪 `docker-publish.yml` 至部署完成。
4. 部署后跑 e2e：

```bash
BASE_URL=https://api.lvlvko.top API_KEY=<key> JWT_TOKEN=<jwt> rtk go test ./test/e2e/session_dedup/ -v
```

5. 验证：cron 列表出现 `SessionTerminalCleanupCron`；下一整点后查生产日志确认其执行；cron_jobs 表中旧名 `SessionDeduplicateCron` 残留记录无害（历史审计），无需迁移。

- [ ] **Step 5: 收尾提交**

```bash
rtk git add docs/superpowers/plans/2026-08-29-session-dedup-insert-time.md
rtk git commit -m "docs(plans): session 前缀去重实时化实施计划进度收尾"
```
