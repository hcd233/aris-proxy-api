package session_dedup

import (
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/cron"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	repository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/gorm"
)

// 终态清理 cron（4788ea54 SessionTerminalCleanupCron）行为回归：
// FindCreatedSince 窗口扫描、FilterTerminalToolCallIDs SQL 形态、删除链路
// （PickTerminalStuckSessions + BatchDeleteByField）此前零行为守护，
// 且迁移时删掉了源自生产 trace 77a87daf 的 TestMergeTargetProtectedFromTerminalRule，
// 本文件补齐这些守护并恢复其守卫语义（见 TestMergeTargetKeepsAbsorbedToolIDs）。
//
// sqlite 不支持 FilterTerminalToolCallIDs 的 ::jsonb 谓词，SQL 形态用
// postgres DryRun 钉住（同 dao_group_query_test.go 的先例），命中集合在
// 链路用例中代入。

// terminalDBSeq 保证内存库名唯一：cache=shared 的 sqlite 内存库按名字共享生命周期，
// -count=N 复跑时复用同名库会读到上一轮残留数据
var terminalDBSeq atomic.Uint64

// newTerminalCleanupDB 创建 sqlite 内存库并迁移 sessions 表（真实执行删除链路）
func newTerminalCleanupDB(t *testing.T) *gorm.DB {
	t.Helper()
	return newApplyTestDB(t, fmt.Sprintf("terminal_cleanup_%d", terminalDBSeq.Add(1)))
}

// seedTerminalSession 写入会话行并显式指定创建时间（窗口过滤用）
func seedTerminalSession(t *testing.T, db *gorm.DB, s *dbmodel.Session, createdAt time.Time) {
	t.Helper()
	s.CreatedAt = createdAt
	s.UpdatedAt = createdAt
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("seed session %d: %v", s.ID, err)
	}
}

// mustReloadSession 重新读库取会话行（含软删行）
func mustReloadSession(t *testing.T, db *gorm.DB, id uint) *dbmodel.Session {
	t.Helper()
	var row dbmodel.Session
	if err := db.Unscoped().Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatalf("reload session %d: %v", id, err)
	}
	return &row
}

// TestFindCreatedSince_WindowAndActiveFilter 真库验证窗口扫描语义：
// 只返回「窗口内创建且未软删」的会话，并载入 MessageIDs。
func TestFindCreatedSince_WindowAndActiveFilter(t *testing.T) {
	t.Parallel()
	db := newTerminalCleanupDB(t)
	now := time.Now().UTC()
	since := now.Add(-constant.CronTerminalCleanupWindow)

	inWindow := &dbmodel.Session{MessageIDs: []uint{1, 2}}
	inWindow.ID = 1
	outWindow := &dbmodel.Session{MessageIDs: []uint{3}}
	outWindow.ID = 2
	softDeleted := &dbmodel.Session{MessageIDs: []uint{4}, BaseModel: dbmodel.BaseModel{DeletedAt: now.Unix()}}
	softDeleted.ID = 3

	seedTerminalSession(t, db, inWindow, now.Add(-time.Hour))
	seedTerminalSession(t, db, outWindow, now.Add(-constant.CronTerminalCleanupWindow-time.Hour))
	seedTerminalSession(t, db, softDeleted, now.Add(-time.Hour))

	views, err := dao.GetSessionDAO().FindCreatedSince(db, since)
	if err != nil {
		t.Fatalf("FindCreatedSince() error = %v", err)
	}
	if len(views) != 1 || views[0].ID != 1 {
		t.Fatalf("FindCreatedSince() = %+v, want only session 1 (in-window, active)", views)
	}
	if !slices.Equal(views[0].MessageIDs, []uint{1, 2}) {
		t.Errorf("scan view MessageIDs = %v, want [1 2]", views[0].MessageIDs)
	}
}

// TestFilterTerminalToolCallIDs_SQLShape 钉住筛选 SQL 形态：候选 id IN、活跃行
// 过滤，以及 assistant+tool_calls 的 ::jsonb 判定谓词（PG 专有，sqlite 不可执行，
// 真实行为由 e2e 覆盖）。
func TestFilterTerminalToolCallIDs_SQLShape(t *testing.T) {
	t.Parallel()
	dry := newSQLShapeDB(t)

	// DAO 内部自建查询链，SQL 无法从返回值取得；DryRun 下 Find 走完
	// BuildQuerySQL 即返回，用回调捕获生成的 SQL
	var captured string
	if err := dry.Callback().Query().After("gorm:query").
		Register("test:capture_terminal_filter_sql", func(tx *gorm.DB) {
			captured = tx.Statement.SQL.String()
		}); err != nil {
		t.Fatalf("register sql capture callback: %v", err)
	}

	if _, err := dao.GetMessageDAO().FilterTerminalToolCallIDs(dry, []uint{80, 81}); err != nil {
		t.Fatalf("FilterTerminalToolCallIDs() error = %v", err)
	}
	if captured == "" {
		t.Fatal("sql capture callback got no SQL")
	}
	assertSQLContains(t, captured, []string{
		// GORM DryRun 会把 `id IN ?` 展开为 `id IN ($1,...)`，故断言到展开前缀
		"id IN (",
		constant.DBConditionDeletedAtZero,
		constant.DBJSONConditionAssistantRole,
		constant.DBJSONConditionHasToolCalls,
	})
}

// TestMergeTargetKeepsAbsorbedToolIDs 恢复被删回归 TestMergeTargetProtectedFromTerminalRule
// （生产 trace 77a87daf）的守卫语义。
//
// 场景：session 3 ([10,20,30]) 是 session 2 ([10,20,30,40,50]) 的前缀被并入
// session 2；session 1 在 [10,20] 之后走 70.. 构成对话分叉，session 2 既是
// 分叉分支又是 merge target。被删回归防的数据丢失面：merge target 被删时
// 并入它的 ToolIDs 随之丢失（#85）。
//
// 新架构（4788ea54）下前缀去重在插入期实时执行，终态清理只做 24h 窗口删除，
// merge target 若末条命中 terminal 会在稳态被删除（旧实现只是多存活一个 cron
// 周期，见 session_terminal_cleanup.go 的稳态语义说明）。因此等价守卫落在
// 合并写回上：吸收发生当轮 merge target 必须存活、并入的 ToolIDs 必须落库。
func TestMergeTargetKeepsAbsorbedToolIDs(t *testing.T) {
	t.Parallel()

	sessions := []*dbmodel.Session{
		{ID: 1, MessageIDs: []uint{10, 20, 70, 80, 90, 100}, ToolIDs: []uint{100}},
		{ID: 2, MessageIDs: []uint{10, 20, 30, 40, 50}, ToolIDs: []uint{200}},
		{ID: 3, MessageIDs: []uint{10, 20, 30}, ToolIDs: []uint{30}},
	}

	result := repository.FindRedundantSessions(sessions)

	if !slices.Contains(result.RedundantIDs, 3) {
		t.Errorf("session 3 ([10,20,30] 是 session 2 的前缀) 应判冗余, got %v", result.RedundantIDs)
	}
	if slices.Contains(result.RedundantIDs, 2) {
		t.Errorf("session 2 是 merge target 不得被去重删除, got %v", result.RedundantIDs)
	}
	if slices.Contains(result.RedundantIDs, 1) {
		t.Errorf("session 1 是分叉分支不得判冗余, got %v", result.RedundantIDs)
	}
	if _, ok := result.MergeMapping[2]; !ok {
		t.Errorf("session 3 的 ToolIDs 应并入 merge target session 2, got mapping=%v", result.MergeMapping)
	}

	db := newTerminalCleanupDB(t)
	for _, s := range sessions {
		seedTerminalSession(t, db, s, time.Now().UTC().Add(-time.Hour))
	}
	if _, err := repository.ApplyMergeResult(db, dao.GetSessionDAO(), result); err != nil {
		t.Fatalf("ApplyMergeResult() error = %v", err)
	}

	target := mustReloadSession(t, db, 2)
	if target.DeletedAt != 0 {
		t.Error("merge target 在合并当轮必须存活（被删回归守卫语义）")
	}
	if !slices.Equal(target.ToolIDs, []uint{30, 200}) {
		t.Errorf("merge target tool_ids = %v, want [30 200]（吸收的 ToolIDs 必须落库，否则随删除丢失）", target.ToolIDs)
	}
	if absorbed := mustReloadSession(t, db, 3); absorbed.DeletedAt == 0 {
		t.Error("被吸收的 session 3 应被软删")
	}
}

// TestTerminalCleanupChain_DeletesOnlyTerminalHits 组合「插入期去重 -> 24h 窗口
// 终态清理」全链路，钉住删除语义：命中 terminal 的会话被删，merge target 与
// 未命中者存活。
//
// 链路复刻 SessionTerminalCleanupCron.cleanup 的步骤；FilterTerminalToolCallIDs
// 的 ::jsonb 谓词为 PG 专有（SQL 形态由 TestFilterTerminalToolCallIDs_SQLShape
// 守护），其命中集合在链路中代入。
func TestTerminalCleanupChain_DeletesOnlyTerminalHits(t *testing.T) {
	t.Parallel()
	db := newTerminalCleanupDB(t)
	now := time.Now().UTC()

	// 21: merge target（吸收 25），末条 50 为普通 assistant（非 terminal）-> 存活
	// 25: 21 的前缀 -> 去重合并进 21
	// 22: 末条 80 为 assistant+tool_calls（terminal）-> 终态删除
	// 23: 末条 91 非 terminal -> 未命中者存活
	sessions := []*dbmodel.Session{
		{ID: 21, MessageIDs: []uint{10, 20, 30, 40, 50}, ToolIDs: []uint{200}},
		{ID: 25, MessageIDs: []uint{10, 20, 30}, ToolIDs: []uint{30}},
		{ID: 22, MessageIDs: []uint{60, 70, 80}, ToolIDs: []uint{400}},
		{ID: 23, MessageIDs: []uint{90, 91}, ToolIDs: []uint{500}},
	}
	for _, s := range sessions {
		seedTerminalSession(t, db, s, now.Add(-time.Hour))
	}

	// 阶段一：插入期前缀去重（repository.FindRedundantSessions + ApplyMergeResult）
	merge := repository.FindRedundantSessions(sessions)
	if !slices.Contains(merge.RedundantIDs, 25) {
		t.Fatalf("session 25 是 21 的前缀应判冗余, got %v", merge.RedundantIDs)
	}
	if _, err := repository.ApplyMergeResult(db, dao.GetSessionDAO(), merge); err != nil {
		t.Fatalf("ApplyMergeResult() error = %v", err)
	}

	// 阶段二：终态清理链路（cleanup 的 FindCreatedSince -> Filter -> Pick -> BatchDelete）
	views, err := dao.GetSessionDAO().FindCreatedSince(db, now.Add(-constant.CronTerminalCleanupWindow))
	if err != nil {
		t.Fatalf("FindCreatedSince() error = %v", err)
	}
	gotIDs := make([]uint, 0, len(views))
	for _, v := range views {
		gotIDs = append(gotIDs, v.ID)
	}
	slices.Sort(gotIDs)
	if !slices.Equal(gotIDs, []uint{21, 22, 23}) {
		t.Fatalf("window scan ids = %v, want [21 22 23]（软删的 25 不得进扫描）", gotIDs)
	}

	terminalIDs := []uint{80} // FilterTerminalToolCallIDs 代入值：22 的末条命中 terminal
	victims := cron.PickTerminalStuckSessions(views, terminalIDs)
	if !slices.Equal(victims, []uint{22}) {
		t.Fatalf("terminal victims = %v, want [22]", victims)
	}
	if err := dao.GetSessionDAO().BatchDeleteByField(db, constant.WhereFieldID, victims); err != nil {
		t.Fatalf("BatchDeleteByField() error = %v", err)
	}

	if hit := mustReloadSession(t, db, 22); hit.DeletedAt == 0 {
		t.Error("命中 terminal 的 session 22 必须被删除")
	}
	target := mustReloadSession(t, db, 21)
	if target.DeletedAt != 0 {
		t.Error("merge target session 21 必须存活")
	}
	if !slices.Equal(target.ToolIDs, []uint{30, 200}) {
		t.Errorf("merge target tool_ids = %v, want [30 200]", target.ToolIDs)
	}
	if survivor := mustReloadSession(t, db, 23); survivor.DeletedAt != 0 {
		t.Error("未命中 terminal 的 session 23 必须存活")
	}
}
