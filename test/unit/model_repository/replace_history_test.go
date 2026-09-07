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

// seedCrossTenantKeyConflict 构造跨用户同名 key 场景：
// user 1（同步发起者）与 user 2 都持有名为 "shared" 的 key，user 1 另有独占名 "solo"；
// user 1 自己也有一个 shared 名下的 session（也无法区分归属）。
func seedCrossTenantKeyConflict(t *testing.T, db *gorm.DB) {
	t.Helper()
	keys := []*dbmodel.ProxyAPIKey{
		{UserID: 1, Name: "shared", Key: "shared-a-val"},
		{UserID: 1, Name: "solo", Key: "solo-val"},
		{UserID: 2, Name: "shared", Key: "shared-b-val"},
	}
	if err := db.Create(keys).Error; err != nil {
		t.Fatal(err)
	}
	msgShared := &dbmodel.Message{ModelID: "old-id", CheckSum: "m-shared"}
	if err := db.Create(msgShared).Error; err != nil {
		t.Fatal(err)
	}
	sessions := []*dbmodel.Session{
		{APIKeyName: "shared", ModelIDs: []string{"old-id"}},                                   // user 2 的 key 产生
		{APIKeyName: "shared", MessageIDs: []uint{msgShared.ID}, ModelIDs: []string{"old-id"}}, // user 1 自己的 shared 名 session
		{APIKeyName: "solo", ModelIDs: []string{"old-id"}},                                     // user 1 独占名
	}
	if err := db.Create(sessions).Error; err != nil {
		t.Fatal(err)
	}
}

// TestReplaceHistoricalModelID_SkipsCrossTenantSameNameKey 跨用户同名 key 名下的 session 必须整体跳过。
//
// api_key_name 是 session 唯一的归属维度，而 key 名仅 (user_id, name, deleted_at) 复合唯一、
// 跨用户可重名；同名冲突时无法区分 session 归属，必须宁漏勿越——冲突名的 session 一律不替换，
// 否则会改写他人的会话数据（含其引用的消息）。
func TestReplaceHistoricalModelID_SkipsCrossTenantSameNameKey(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	seedCrossTenantKeyConflict(t, db)
	repo := repository.NewModelRepository(db)

	counts, err := repo.ReplaceHistoricalModelID(context.Background(), 1, "old-id", "new-id")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if counts.SessionCount != 1 {
		t.Fatalf("session count = %d, want 1 (solo only; shared-name must be skipped)", counts.SessionCount)
	}

	// 两个 shared 名 session（含 user 1 自己的）都保持原值
	var sharedRows []dbmodel.Session
	if err := db.Where("api_key_name = ?", "shared").Find(&sharedRows).Error; err != nil {
		t.Fatal(err)
	}
	if len(sharedRows) != 2 {
		t.Fatalf("shared sessions = %d, want 2", len(sharedRows))
	}
	for _, row := range sharedRows {
		if len(row.ModelIDs) != 1 || row.ModelIDs[0] != "old-id" {
			t.Fatalf("shared-name session must stay untouched, got %v", row.ModelIDs)
		}
	}
	// solo 名 session 正常替换
	var solo dbmodel.Session
	if err := db.Where("api_key_name = ?", "solo").First(&solo).Error; err != nil {
		t.Fatal(err)
	}
	if len(solo.ModelIDs) != 1 || solo.ModelIDs[0] != "new-id" {
		t.Fatalf("solo-name session must be replaced, got %v", solo.ModelIDs)
	}
	// 被跳过 session 引用的消息不得连带替换
	var msgShared int64
	db.Model(&dbmodel.Message{}).Where("check_sum = ? AND model_id = ?", "m-shared", "old-id").Count(&msgShared)
	if msgShared != 1 {
		t.Fatalf("message referenced by skipped session must stay old, got replaced=%d", 1-msgShared)
	}
}

// TestReplaceHistoricalModelID_ModelIDWithJSONEscapedBytes modelId 含 JSON 需转义字节（" \ &）时，
// session 的 LIKE 预过滤必须按存储字节（JSON 转义形式）构造模式，不得漏匹配。
func TestReplaceHistoricalModelID_ModelIDWithJSONEscapedBytes(t *testing.T) {
	t.Parallel()
	for _, oldID := range []string{`gpt"4`, `gpt\4`, `a&b`} {
		t.Run(oldID, func(t *testing.T) {
			t.Parallel()
			db := newHistorySyncDB(t)
			key := &dbmodel.ProxyAPIKey{UserID: 1, Name: "key-a", Key: "v"}
			if err := db.Create(key).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&dbmodel.Session{APIKeyName: "key-a", ModelIDs: []string{oldID, "keep"}}).Error; err != nil {
				t.Fatal(err)
			}
			repo := repository.NewModelRepository(db)

			counts, err := repo.ReplaceHistoricalModelID(context.Background(), 1, oldID, "new-id")
			if err != nil {
				t.Fatalf("replace %q: %v", oldID, err)
			}
			if counts.SessionCount != 1 {
				t.Fatalf("oldID %q: session count = %d, want 1 (LIKE prefilter missed JSON-escaped bytes)", oldID, counts.SessionCount)
			}
			var sess dbmodel.Session
			if err := db.Where("api_key_name = ?", "key-a").First(&sess).Error; err != nil {
				t.Fatal(err)
			}
			if len(sess.ModelIDs) != 2 || sess.ModelIDs[0] != "new-id" || sess.ModelIDs[1] != "keep" {
				t.Fatalf("oldID %q: session model_ids = %v, want [new-id keep]", oldID, sess.ModelIDs)
			}
		})
	}
}

// TestReplaceHistoricalModelID_ModelIDWithLikeWildcards modelId 含 LIKE 通配符（% _）时
// 预过滤不得放大语义：精确等值替换必须只命中完全相等的元素，不得误伤 gpt-4x / aXb。
func TestReplaceHistoricalModelID_ModelIDWithLikeWildcards(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	key := &dbmodel.ProxyAPIKey{UserID: 1, Name: "key-a", Key: "v"}
	if err := db.Create(key).Error; err != nil {
		t.Fatal(err)
	}
	sessions := []*dbmodel.Session{
		{APIKeyName: "key-a", ModelIDs: []string{"100%"}},
		{APIKeyName: "key-a", ModelIDs: []string{"100X"}},
		{APIKeyName: "key-a", ModelIDs: []string{"a_b"}},
		{APIKeyName: "key-a", ModelIDs: []string{"aXb"}},
	}
	if err := db.Create(sessions).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewModelRepository(db)

	for _, oldID := range []string{"100%", "a_b"} {
		counts, err := repo.ReplaceHistoricalModelID(context.Background(), 1, oldID, "new-id")
		if err != nil {
			t.Fatalf("replace %q: %v", oldID, err)
		}
		if counts.SessionCount != 1 {
			t.Fatalf("oldID %q: session count = %d, want exact 1", oldID, counts.SessionCount)
		}
	}
	var remain100X, remainAXb int64
	db.Model(&dbmodel.Session{}).Where("model_ids LIKE ?", `%100X%`).Count(&remain100X)
	db.Model(&dbmodel.Session{}).Where("model_ids LIKE ?", `%aXb%`).Count(&remainAXb)
	if remain100X != 1 || remainAXb != 1 {
		t.Fatalf("wildcard-adjacent ids must stay, got 100X=%d aXb=%d", remain100X, remainAXb)
	}
}
