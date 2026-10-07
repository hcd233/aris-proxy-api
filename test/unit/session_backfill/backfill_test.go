// Package session_backfill 验证 sessions/traces 的 api_key_id 存量回填语义。
//
// 回填规则（宁漏勿越）：仅当 api_key_id=0、api_key_name 非空、且该名称在
// proxy_api_keys **全表（含已软删）** 中恰好对应一条记录、该记录未软删时，
// 才回填为该 key 的 ID。
//
// 为什么唯一性要把已软删的 key 也算上：若名称 X 当前只有用户 A 的一个
// 存活 key，但历史上用户 B 也有过同名 key（已删），那么名称为 X 的历史
// 会话可能属于 B——此时归属无法确定，必须跳过，不得猜测。回填错误会把
// 他人会话直接划给 A，比「保持仅 admin 可见」危害大得多。
//
// 另外：空归属与歧义名不推断；已有归属不覆盖；软删会话不回填（不可见，
// 回填只增加写量）。连续执行两次结果一致（幂等）。
package session_backfill

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// deletedAtStamp 软删标记（BaseModel.DeletedAt 为 int64，0 = 存活）
const deletedAtStamp int64 = 1700000000

type sessionSeed struct {
	name      string
	apiKeyID  uint
	deletedAt int64
	want      uint
	why       string
}

func newBackfillDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"),
		&gorm.Config{TranslateError: true, Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	// cache=private 的内存库按连接隔离：限单连接保证读写共库
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&dbmodel.ProxyAPIKey{}, &dbmodel.Session{}, &dbmodel.Trace{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	keys := []*dbmodel.ProxyAPIKey{
		{ID: 1, UserID: 1, Name: "uniq-key", Key: "sk-1"},
		{ID: 2, UserID: 1, Name: "dup-key", Key: "sk-2"},
		{ID: 3, UserID: 2, Name: "dup-key", Key: "sk-3"},
		{ID: 4, UserID: 3, Name: "del-key", Key: "sk-4", DeletedAt: deletedAtStamp},
		{ID: 5, UserID: 4, Name: "half-del", Key: "sk-5"},
		{ID: 6, UserID: 5, Name: "half-del", Key: "sk-6", DeletedAt: deletedAtStamp},
	}
	if err := db.Create(keys).Error; err != nil {
		t.Fatalf("seed keys: %v", err)
	}
	return db
}

func sessionCases() []sessionSeed {
	return []sessionSeed{
		{name: "uniq-key", want: 1, why: "名称全表唯一且存活 → 回填"},
		{name: "", want: 0, why: "空归属不推断"},
		{name: "dup-key", want: 0, why: "跨用户同名（两条存活）→ 歧义跳过"},
		{name: "ghost-key", want: 0, why: "无对应 key → 跳过"},
		{name: "uniq-key", apiKeyID: 7, want: 7, why: "已有归属不覆盖"},
		{name: "del-key", want: 0, why: "唯一对应的 key 已软删 → 跳过"},
		{name: "half-del", want: 0, why: "存活 1 + 已删 1 同名 → 历史归属歧义跳过"},
		{name: "uniq-key", deletedAt: deletedAtStamp, want: 0, why: "软删会话不回填"},
	}
}

func seedSessions(t *testing.T, db *gorm.DB, cases []sessionSeed) []uint {
	t.Helper()
	ids := make([]uint, 0, len(cases))
	for _, c := range cases {
		row := &dbmodel.Session{APIKeyName: c.name, APIKeyID: c.apiKeyID, MessageIDs: []uint{1}, ToolIDs: []uint{}}
		row.DeletedAt = c.deletedAt
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed session: %v", err)
		}
		ids = append(ids, row.ID)
	}
	return ids
}

func apiKeyIDOf(t *testing.T, db *gorm.DB, sessionID uint) uint {
	t.Helper()
	var row dbmodel.Session
	if err := db.Select("api_key_id").Where("id = ?", sessionID).Take(&row).Error; err != nil {
		t.Fatalf("reload session %d: %v", sessionID, err)
	}
	return row.APIKeyID
}

// TestBackfillOnlyFillsUnambiguous 只回填能唯一确定归属的行。
func TestBackfillOnlyFillsUnambiguous(t *testing.T) {
	t.Parallel()
	db := newBackfillDB(t)
	cases := sessionCases()
	ids := seedSessions(t, db, cases)

	sessions, _, err := repository.BackfillSessionAPIKeyID(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if sessions != 1 {
		t.Errorf("backfilled sessions = %d, want 1（仅第一条满足条件）", sessions)
	}
	for i, c := range cases {
		if got := apiKeyIDOf(t, db, ids[i]); got != c.want {
			t.Errorf("case %d (%s, name=%q): api_key_id = %d, want %d", i, c.why, c.name, got, c.want)
		}
	}
}

// TestBackfillIdempotent 连续执行两次结果一致，第二次无写入。
//
// 迁移可能被重复执行（重试、多次部署），非幂等会造成数据漂移。
func TestBackfillIdempotent(t *testing.T) {
	t.Parallel()
	db := newBackfillDB(t)
	cases := sessionCases()
	ids := seedSessions(t, db, cases)
	ctx := context.Background()

	if _, _, err := repository.BackfillSessionAPIKeyID(ctx, db); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	first := make([]uint, len(ids))
	for i, id := range ids {
		first[i] = apiKeyIDOf(t, db, id)
	}

	sessions, traces, err := repository.BackfillSessionAPIKeyID(ctx, db)
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if sessions != 0 || traces != 0 {
		t.Errorf("second run affected (sessions=%d, traces=%d), want (0, 0)", sessions, traces)
	}
	for i, id := range ids {
		if got := apiKeyIDOf(t, db, id); got != first[i] {
			t.Errorf("session %d drifted after second run: %d -> %d", id, first[i], got)
		}
	}
}

// TestBackfillTraces traces 表按同一规则回填。
func TestBackfillTraces(t *testing.T) {
	t.Parallel()
	db := newBackfillDB(t)
	traces := []*dbmodel.Trace{
		{SessionID: "t-uniq", APIKeyName: "uniq-key", Agent: "codex"},
		{SessionID: "t-dup", APIKeyName: "dup-key", Agent: "codex"},
	}
	if err := db.Create(traces).Error; err != nil {
		t.Fatalf("seed traces: %v", err)
	}

	_, affected, err := repository.BackfillSessionAPIKeyID(context.Background(), db)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if affected != 1 {
		t.Errorf("backfilled traces = %d, want 1", affected)
	}
	var got []dbmodel.Trace
	if err := db.Order("id").Find(&got).Error; err != nil {
		t.Fatalf("reload traces: %v", err)
	}
	if got[0].APIKeyID != 1 {
		t.Errorf("t-uniq api_key_id = %d, want 1", got[0].APIKeyID)
	}
	if got[1].APIKeyID != 0 {
		t.Errorf("t-dup api_key_id = %d, want 0（跨用户同名不推断）", got[1].APIKeyID)
	}
}
