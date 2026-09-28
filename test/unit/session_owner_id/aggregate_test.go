package session_owner_id

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// TestCreateSessionRequiresOwnerID 归属 ID 为 0 时必须拒绝创建。
//
// ID 归属是鉴权唯一依据，0 表示身份缺失，落库会产生对普通用户不可见的
// 孤儿会话（正是 2026-07 起 2.6k 条存量的成因）。
func TestCreateSessionRequiresOwnerID(t *testing.T) {
	t.Parallel()
	_, err := aggregate.CreateSession(vo.APIKeyOwnerID(0), "any-name",
		[]uint{1}, nil, nil, time.Now())
	if err == nil {
		t.Error("CreateSession with owner id 0 must fail")
	}
}

// TestCreateSessionAllowsEmptyOwnerName 名称为空不阻塞创建。
//
// name 已降级为纯展示字段，不参与鉴权，不应成为写入的硬门槛。
func TestCreateSessionAllowsEmptyOwnerName(t *testing.T) {
	t.Parallel()
	s, err := aggregate.CreateSession(vo.APIKeyOwnerID(3), "",
		[]uint{1}, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("CreateSession with empty name must succeed, got %v", err)
	}
	if s.OwnerID().Uint() != 3 {
		t.Errorf("OwnerID = %d, want 3", s.OwnerID().Uint())
	}
}

// TestIsOwnedByIDExactMatch 归属判定按 ID 精确匹配。
//
// 同名不同 ID 必须判为非所有者——这正是缺陷 A 的核心：按名称判定时
// 两个用户的同名 Key 会互相命中。
func TestIsOwnedByIDExactMatch(t *testing.T) {
	t.Parallel()
	s, err := aggregate.CreateSession(vo.APIKeyOwnerID(5), "shared-name",
		[]uint{1}, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if !s.IsOwnedByID(5) {
		t.Error("IsOwnedByID(5) must be true for owner id 5")
	}
	if s.IsOwnedByID(6) {
		t.Error("IsOwnedByID(6) must be false for owner id 5")
	}
	if s.OwnerName() != "shared-name" {
		t.Errorf("OwnerName = %q, want shared-name", s.OwnerName())
	}
}

// TestZeroOwnerIDNeverMatches 归属 0（存量空归属）不得匹配任何真实 key。
//
// 自增主键从 1 起，故 0 天然不匹配；本用例锁定该不变量，
// 并覆盖「请求方 ID 为 0（认证缺失）」的反向情形。
func TestZeroOwnerIDNeverMatches(t *testing.T) {
	t.Parallel()
	orphan := aggregate.RestoreSession(1, vo.APIKeyOwnerID(0), "", []uint{1}, nil,
		nil, vo.SessionScore{}, time.Now(), time.Now())
	for _, id := range []uint{0, 1, 2, 100} {
		if orphan.IsOwnedByID(id) {
			t.Errorf("session with owner id 0 must not be owned by %d", id)
		}
	}

	// 反向：归属正常但请求方 ID 为 0（认证缺失）同样不得放行
	owned := aggregate.RestoreSession(2, vo.APIKeyOwnerID(9), "k", []uint{1}, nil,
		nil, vo.SessionScore{}, time.Now(), time.Now())
	if owned.IsOwnedByID(0) {
		t.Error("requester id 0 must never be treated as owner")
	}
}

// TestAPIKeyOwnerIDIsEmpty 值对象的空语义。
func TestAPIKeyOwnerIDIsEmpty(t *testing.T) {
	t.Parallel()
	if !vo.APIKeyOwnerID(0).IsEmpty() {
		t.Error("APIKeyOwnerID(0).IsEmpty() must be true")
	}
	if vo.APIKeyOwnerID(1).IsEmpty() {
		t.Error("APIKeyOwnerID(1).IsEmpty() must be false")
	}
}
