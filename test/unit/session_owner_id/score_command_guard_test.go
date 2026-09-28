package session_owner_id

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/session/command"
	"github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// 命令层归属守卫的直接守护。
//
// E2E 里评分请求会先被 handler 层的元数据查询拦下，命令自身的归属判定因此
// 从未被真实执行过——它是纵深防御的第二道闸，必须单独有用例，否则被误删也
// 不会有任何测试变红。

// fakeScoreSessionRepo 嵌入接口，只覆写评分路径用到的方法
type fakeScoreSessionRepo struct {
	session.SessionRepository
	sess         *aggregate.Session
	scoreUpdated bool
	scoreDeleted bool
}

func (f *fakeScoreSessionRepo) FindByID(_ context.Context, _ uint) (*aggregate.Session, error) {
	return f.sess, nil
}

func (f *fakeScoreSessionRepo) UpdateScore(_ context.Context, _ uint, _ vo.SessionScore) error {
	f.scoreUpdated = true
	return nil
}

func (f *fakeScoreSessionRepo) DeleteScore(_ context.Context, _ uint) error {
	f.scoreDeleted = true
	return nil
}

// fakeOwnerKeys 嵌入接口，只覆写 LookupIDsByUserID
type fakeOwnerKeys struct {
	apikey.APIKeyRepository
	ids []uint
}

func (f *fakeOwnerKeys) LookupIDsByUserID(_ context.Context, _ uint) ([]uint, error) {
	return f.ids, nil
}

// sessionOwnedBy 构造归属给定 key ID、名称为同名攻击面 "shared-name" 的会话
func sessionOwnedBy(apiKeyID uint) *aggregate.Session {
	return aggregate.RestoreSession(1, vo.APIKeyOwnerID(apiKeyID), "shared-name",
		[]uint{1}, nil, nil, vo.SessionScore{}, time.Now(), time.Now())
}

// TestScoreCommandRejectsForeignOwner 同名不同 ID 的请求方不得评分，且不落库。
func TestScoreCommandRejectsForeignOwner(t *testing.T) {
	t.Parallel()
	repo := &fakeScoreSessionRepo{sess: sessionOwnedBy(1)}
	h := command.NewScoreSessionHandler(repo, &fakeOwnerKeys{ids: []uint{2}})

	_, err := h.Handle(context.Background(), port.ScoreSessionCommand{
		SessionID: 1, Score: 3, RequesterID: 20, RequesterPermission: enum.PermissionUser,
	})
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Fatalf("foreign owner score: err = %v, want ErrNoPermission", err)
	}
	if repo.scoreUpdated {
		t.Fatal("foreign owner score must not reach UpdateScore")
	}
}

// TestScoreCommandAllowsOwner 所有者可评分（防止修复过度收紧）。
func TestScoreCommandAllowsOwner(t *testing.T) {
	t.Parallel()
	repo := &fakeScoreSessionRepo{sess: sessionOwnedBy(1)}
	h := command.NewScoreSessionHandler(repo, &fakeOwnerKeys{ids: []uint{1}})

	if _, err := h.Handle(context.Background(), port.ScoreSessionCommand{
		SessionID: 1, Score: 3, RequesterID: 10, RequesterPermission: enum.PermissionUser,
	}); err != nil {
		t.Fatalf("owner score: %v", err)
	}
	if !repo.scoreUpdated {
		t.Fatal("owner score must reach UpdateScore")
	}
}

// TestDeleteScoreCommandRejectsForeignOwner 删除评分同理。
func TestDeleteScoreCommandRejectsForeignOwner(t *testing.T) {
	t.Parallel()
	repo := &fakeScoreSessionRepo{sess: sessionOwnedBy(1)}
	h := command.NewDeleteScoreSessionHandler(repo, &fakeOwnerKeys{ids: []uint{2}})

	err := h.Handle(context.Background(), port.DeleteScoreSessionCommand{
		SessionID: 1, RequesterID: 20, RequesterPermission: enum.PermissionUser,
	})
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Fatalf("foreign owner delete score: err = %v, want ErrNoPermission", err)
	}
	if repo.scoreDeleted {
		t.Fatal("foreign owner delete score must not reach DeleteScore")
	}
}

// TestScoreCommandRejectsOrphanSession 空归属会话对普通用户不可评分。
func TestScoreCommandRejectsOrphanSession(t *testing.T) {
	t.Parallel()
	repo := &fakeScoreSessionRepo{sess: sessionOwnedBy(0)}
	h := command.NewScoreSessionHandler(repo, &fakeOwnerKeys{ids: []uint{1, 2}})

	_, err := h.Handle(context.Background(), port.ScoreSessionCommand{
		SessionID: 1, Score: 3, RequesterID: 10, RequesterPermission: enum.PermissionUser,
	})
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Fatalf("orphan session score: err = %v, want ErrNoPermission", err)
	}
}
