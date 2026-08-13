// Package trace_repository 验证 traceRepository.InsertEvent 生成的
// ON CONFLICT 语句与部分唯一索引 predicate 兼容。
//
// 背景（Critical，2026-08-12 review 发现）：
//   - model/trace.go dedup_key 唯一索引是部分索引
//     `uniq_trace_event_dedup ... WHERE dedup_key <> ”`。
//   - token_count 分支此前用 `clause.OnConflict{Columns: [{dedup_key}]}`
//     生成 `ON CONFLICT ("dedup_key") DO UPDATE ...`，缺少 index predicate。
//     PostgreSQL 对部分唯一索引做 arbiter 推断必须给出匹配的 index_predicate，
//     否则每条 INSERT 抛 42P10（无匹配唯一约束），token_count 覆盖特性在生产不可用。
//   - 修复：给冲突目标补 `TargetWhere`（`dedup_key <> ”`）。
package trace_repository

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// captureLogger 捕获 GORM 执行的最终 SQL（含占位符未替换，仅用于断言语句形态）。
type captureLogger struct {
	gormlogger.Interface
	mu   sync.Mutex
	sqls []string
}

func (l *captureLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	l.mu.Lock()
	l.sqls = append(l.sqls, sql)
	l.mu.Unlock()
}

func (l *captureLogger) lastSQL() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.sqls) == 0 {
		return ""
	}
	return l.sqls[len(l.sqls)-1]
}

func newCaptureDB(t *testing.T) (*gorm.DB, *captureLogger) {
	t.Helper()
	logger := &captureLogger{}
	db, err := gorm.Open(sqlite.Open("file:dryrun?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("failed to open capture db: %v", err)
	}
	return db, logger
}

// TestInsertEventTokenCountOnConflictHasPredicate
// token_count 事件（覆盖写入语义）的 ON CONFLICT 必须携带部分唯一索引 predicate。
func TestInsertEventTokenCountOnConflictHasPredicate(t *testing.T) {
	t.Parallel()
	db, logger := newCaptureDB(t)
	repo := repository.NewTraceRepository(db)
	ctx := context.Background()

	if _, err := repo.InsertEvent(ctx, &trace.TraceEvent{
		SessionID:  "s1",
		RecordType: constant.TraceRecordTypeEventMsg,
		Event:      constant.TraceEventTokenCount,
		DedupKey:   "token:s1:1",
		Payload:    []byte(`{}`),
	}); err != nil {
		t.Fatalf("InsertEvent failed: %v", err)
	}

	sql := logger.lastSQL()
	t.Logf("generated SQL: %s", sql)

	if !strings.Contains(sql, "ON CONFLICT") {
		t.Fatalf("expected ON CONFLICT clause, got SQL: %s", sql)
	}
	// 部分唯一索引必须带 predicate，否则 PG 42P10 拒绝
	if !strings.Contains(sql, "dedup_key <> ''") {
		t.Fatalf("expected partial-index predicate in ON CONFLICT, got SQL: %s", sql)
	}
	if !strings.Contains(sql, "DO UPDATE") {
		t.Fatalf("expected DO UPDATE (覆盖写入) semantics, got SQL: %s", sql)
	}
}

// TestInsertEventRegularDedupDoNothing
// 常规 dedup 事件走 ON CONFLICT DO NOTHING（无目标列），不要求 predicate。
func TestInsertEventRegularDedupDoNothing(t *testing.T) {
	t.Parallel()
	db, logger := newCaptureDB(t)
	repo := repository.NewTraceRepository(db)
	ctx := context.Background()

	if _, err := repo.InsertEvent(ctx, &trace.TraceEvent{
		SessionID:  "s1",
		RecordType: constant.TraceRecordTypeHookEvent,
		Event:      constant.TraceEventPostToolUse,
		DedupKey:   "hook:s1:1",
		Payload:    []byte(`{}`),
	}); err != nil {
		t.Fatalf("InsertEvent failed: %v", err)
	}

	sql := logger.lastSQL()
	t.Logf("generated SQL: %s", sql)

	if !strings.Contains(sql, "ON CONFLICT DO NOTHING") {
		t.Fatalf("expected ON CONFLICT DO NOTHING, got SQL: %s", sql)
	}
}
