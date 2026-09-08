package model_repository

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
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
		&dbmodel.Model{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedModel 写入一行归属 userID、业务 ID 为 oldID 的模型行，并返回已改名到 new-id 的聚合
// （模拟命令层完成领域更新后传入仓储的聚合状态：聚合当前值即 new-id，oldID 显式传入）。
func seedModel(t *testing.T, db *gorm.DB, userID uint, oldID string) *aggregate.Model {
	t.Helper()
	row := &dbmodel.Model{UserID: userID, ModelID: oldID, Alias: "gpt-x"}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed model row: %v", err)
	}
	agg, err := aggregate.CreateModel(row.ID, vo.EndpointAlias("gpt-x"), "up-x", 1, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("create aggregate: %v", err)
	}
	agg.SetUserID(userID)
	agg.SetModelID("new-id")
	return agg
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

func TestUpdateWithHistorySync(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	seedHistorySyncData(t, db)
	agg := seedModel(t, db, 1, "old-id")
	repo := repository.NewModelRepository(db)

	counts, err := repo.UpdateWithHistorySync(t.Context(), agg, "old-id")
	if err != nil {
		t.Fatalf("update with history sync: %v", err)
	}
	if counts.AuditCount != 2 || counts.SessionCount != 1 || counts.MessageCount != 1 {
		t.Fatalf("counts mismatch: %+v (want audit=2 session=1 message=1)", counts)
	}

	// model 行：业务 ID 已改为 new-id
	var modelRow dbmodel.Model
	if err := db.First(&modelRow, agg.AggregateID()).Error; err != nil {
		t.Fatalf("find model row: %v", err)
	}
	if modelRow.ModelID != "new-id" {
		t.Fatalf("model id = %q, want new-id", modelRow.ModelID)
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

func TestUpdateWithHistorySync_NoHit(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	agg := seedModel(t, db, 99, "old-id")
	repo := repository.NewModelRepository(db)

	counts, err := repo.UpdateWithHistorySync(t.Context(), agg, "old-id")
	if err != nil {
		t.Fatalf("update with history sync: %v", err)
	}
	var zero llmproxy.ModelIDSyncCounts
	if counts != zero {
		t.Fatalf("counts = %+v, want zero", counts)
	}
}

// TestUpdateWithHistorySync_RollbackKeepsOldModelIDOnSyncFailure 历史替换失败时模型改名必须整体回滚。
//
// 若不原子，模型本体已改名而历史停留旧 ID：同样的更新请求重试时新旧 ID 相等、
// 同步条件不再触发，历史将永久无法同步（2026-09-08 CR P1）。删掉 message 表强制
// 替换阶段失败，断言事务回滚后 model 行保持旧 ID。
func TestUpdateWithHistorySync_RollbackKeepsOldModelIDOnSyncFailure(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	seedHistorySyncData(t, db)
	agg := seedModel(t, db, 1, "old-id")
	if err := db.Migrator().DropTable(&dbmodel.Message{}); err != nil {
		t.Fatalf("drop messages table: %v", err)
	}
	repo := repository.NewModelRepository(db)

	if _, err := repo.UpdateWithHistorySync(t.Context(), agg, "old-id"); err == nil {
		t.Fatal("history sync failure must propagate as error")
	}
	var row dbmodel.Model
	if err := db.First(&row, agg.AggregateID()).Error; err != nil {
		t.Fatalf("find model row: %v", err)
	}
	if row.ModelID != "old-id" {
		t.Fatalf("model id = %q, want old-id (rename must roll back when sync fails)", row.ModelID)
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

// TestUpdateWithHistorySync_SkipsCrossTenantSameNameKey 跨用户同名 key 名下的 session 必须整体跳过。
//
// api_key_name 是 session 唯一的归属维度，而 key 名仅 (user_id, name, deleted_at) 复合唯一、
// 跨用户可重名；同名冲突时无法区分 session 归属，必须宁漏勿越——冲突名的 session 一律不替换，
// 否则会改写他人的会话数据（含其引用的消息）。
func TestUpdateWithHistorySync_SkipsCrossTenantSameNameKey(t *testing.T) {
	t.Parallel()
	db := newHistorySyncDB(t)
	seedCrossTenantKeyConflict(t, db)
	agg := seedModel(t, db, 1, "old-id")
	repo := repository.NewModelRepository(db)

	counts, err := repo.UpdateWithHistorySync(t.Context(), agg, "old-id")
	if err != nil {
		t.Fatalf("update with history sync: %v", err)
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

// TestUpdateWithHistorySync_ModelIDWithJSONEscapedBytes modelId 含 JSON 需转义字节（" \ &）时，
// session 的 LIKE 预过滤必须按存储字节（JSON 转义形式）构造模式，不得漏匹配。
func TestUpdateWithHistorySync_ModelIDWithJSONEscapedBytes(t *testing.T) {
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
			agg := seedModel(t, db, 1, oldID)
			repo := repository.NewModelRepository(db)

			counts, err := repo.UpdateWithHistorySync(t.Context(), agg, oldID)
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

// TestUpdateWithHistorySync_ModelIDWithLikeWildcards modelId 含 LIKE 通配符（% _）时
// 预过滤不得放大语义：精确等值替换必须只命中完全相等的元素，不得误伤 gpt-4x / aXb。
func TestUpdateWithHistorySync_ModelIDWithLikeWildcards(t *testing.T) {
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
	agg := seedModel(t, db, 1, "old-id")
	repo := repository.NewModelRepository(db)

	for _, oldID := range []string{"100%", "a_b"} {
		counts, err := repo.UpdateWithHistorySync(t.Context(), agg, oldID)
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
