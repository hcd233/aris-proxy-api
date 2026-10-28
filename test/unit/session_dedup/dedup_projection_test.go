package session_dedup

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	repository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

// TestDedupProjection_KeepsOwnerForSessionTarget 回归：会话续写时 ID 漂移。
//
// 组查询按 SessionRepoFieldsDedup 投影读回同组会话，FindSessionTarget 又要用
// APIKeyID 做归属二次校验。投影漏掉 api_key_id 时读回的 APIKeyID 恒为 0，
// 真实 Key（ID ≥ 1）永远不匹配 → 找不到 keeper → 回退到前缀去重把旧行删掉、
// 保留新行，会话 ID 每轮都变。FindSessionTarget 的纯函数用例 fixture 默认
// APIKeyID=0，恰好与漏投影的零值相等，因此必须经真实投影读库来守护。
//
// 组查询本身的 ::jsonb 条件 sqlite 不可执行，这里只复用同一投影常量。
func TestDedupProjection_KeepsOwnerForSessionTarget(t *testing.T) {
	t.Parallel()
	db := newApplyTestDB(t, "dedup_projection_owner")

	const apiKeyID uint = 7
	existing := &dbmodel.Session{APIKeyID: apiKeyID, MessageIDs: []uint{1, 2}, ToolIDs: []uint{10}}
	existing.ID = 1
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("seed session: %v", err)
	}

	var rows []*dbmodel.Session
	if err := db.Select(constant.SessionRepoFieldsDedup).Find(&rows).Error; err != nil {
		t.Fatalf("load with dedup projection: %v", err)
	}

	target := repository.FindSessionTarget(rows, apiKeyID, []uint{1, 2, 3})
	if target == nil || target.ID != existing.ID {
		t.Fatalf("FindSessionTarget() = %+v, want session %d（投影必须带回 api_key_id，否则续写永远新建 ID）", target, existing.ID)
	}
}
