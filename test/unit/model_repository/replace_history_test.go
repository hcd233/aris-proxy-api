package model_repository

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newHistorySyncDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&dbmodel.ProxyAPIKey{}, &dbmodel.ModelCallAudit{}, &dbmodel.Session{}, &dbmodel.Message{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedHistorySyncData 种子：userA 有 key（含已删 key）、audit/session/message 历史均为 oldID；
// userB 有同名 modelId 的 audit 与独立 message（隔离对照）。
func seedHistorySyncData(t *testing.T, db *gorm.DB) {
	t.Helper()
	keyA := &dbmodel.ProxyAPIKey{UserID: 1, Name: "key-a", Key: "key-a-val"}
	keyADel := &dbmodel.ProxyAPIKey{UserID: 1, Name: "key-a-del", Key: "key-a-del-val", DeletedAt: 100}
	keyB := &dbmodel.ProxyAPIKey{UserID: 2, Name: "key-b", Key: "key-b-val"}
	if err := db.Create([]*dbmodel.ProxyAPIKey{keyA, keyADel, keyB}).Error; err != nil {
		t.Fatal(err)
	}
	msgA := &dbmodel.Message{ModelID: "old-id", CheckSum: "m1"}
	msgB := &dbmodel.Message{ModelID: "old-id", CheckSum: "m2"}
	if err := db.Create([]*dbmodel.Message{msgA, msgB}).Error; err != nil {
		t.Fatal(err)
	}
	sessHit := &dbmodel.Session{
		APIKeyName: "key-a",
		MessageIDs: []uint{msgA.ID},
		ModelIDs:   []string{"old-id", "other-id"},
	}
	sessNoHit := &dbmodel.Session{APIKeyName: "key-a", ModelIDs: []string{"other-id"}}
	if err := db.Create([]*dbmodel.Session{sessHit, sessNoHit}).Error; err != nil {
		t.Fatal(err)
	}
	audits := []*dbmodel.ModelCallAudit{
		{APIKeyID: keyA.ID, ModelID: "old-id"},
		{APIKeyID: keyADel.ID, ModelID: "old-id"}, // 已删 key 的历史也要替换
		{APIKeyID: keyB.ID, ModelID: "old-id"},    // B 的，不能动
		{APIKeyID: keyA.ID, ModelID: "other-id"},  // 非 old 的不动
	}
	if err := db.Create(audits).Error; err != nil {
		t.Fatal(err)
	}
}

func TestReplaceHistoricalModelID(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	seedHistorySyncData(t, db)
	repo := repository.NewModelRepository(db)

	counts, err := repo.ReplaceHistoricalModelID(context.Background(), 1, "old-id", "new-id")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if counts.AuditCount != 2 || counts.SessionCount != 1 || counts.MessageCount != 1 {
		t.Fatalf("counts mismatch: %+v (want audit=2 session=1 message=1)", counts)
	}

	// audit：A 的两条（含已删 key）变 new-id，B 的仍是 old-id
	var auditNew, auditOldB int64
	db.Model(&dbmodel.ModelCallAudit{}).Where("model_id = ?", "new-id").Count(&auditNew)
	db.Model(&dbmodel.ModelCallAudit{}).
		Where("model_id = ? AND api_key_id IN (SELECT id FROM proxy_api_keys WHERE user_id = 2)", "old-id").
		Count(&auditOldB)
	if auditNew != 2 || auditOldB != 1 {
		t.Fatalf("audit isolation broken: new=%d old(B)=%d", auditNew, auditOldB)
	}

	// session：数组逐元素替换，other-id 保留
	var sessHit dbmodel.Session
	if err := db.Where("api_key_name = ? AND model_ids LIKE ?", "key-a", "%new-id%").
		First(&sessHit).Error; err != nil {
		t.Fatalf("find hit session: %v", err)
	}
	if len(sessHit.ModelIDs) != 2 || sessHit.ModelIDs[0] != "new-id" || sessHit.ModelIDs[1] != "other-id" {
		t.Fatalf("session model_ids = %v, want [new-id other-id]", sessHit.ModelIDs)
	}

	// message：A 会话引用的变 new-id；B 的独立消息仍是 old-id
	var msgNew, msgOldB int64
	db.Model(&dbmodel.Message{}).Where("model_id = ? AND check_sum = ?", "new-id", "m1").Count(&msgNew)
	db.Model(&dbmodel.Message{}).Where("model_id = ? AND check_sum = ?", "old-id", "m2").Count(&msgOldB)
	if msgNew != 1 || msgOldB != 1 {
		t.Fatalf("message isolation broken: new=%d old(B)=%d", msgNew, msgOldB)
	}
}

func TestReplaceHistoricalModelIDNoHit(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	repo := repository.NewModelRepository(db)

	counts, err := repo.ReplaceHistoricalModelID(context.Background(), 99, "old-id", "new-id")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	var zero llmproxy.ModelIDSyncCounts
	if counts != zero {
		t.Fatalf("counts = %+v, want zero", counts)
	}
}
