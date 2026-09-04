// Package model_command 的 update 命令回归：换绑 endpoint 必须校验目标归属（CR C2）。
//
// 缺陷背景：update 未注入 endpointRepo，用户 A 可把自己的 model 挂到用户 B 的
// endpoint 上——model 会出现在 B 的 upstream 分组视图（ListByEndpointIDs 不做
// 二次 scope 过滤），形成跨租户信息泄露。
package model_command

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/command"
	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

type updateModelRepo struct {
	model   *aggregate.Model
	updated *aggregate.Model
}

func (r *updateModelRepo) FindByID(_ context.Context, id uint, scope *uint) (*aggregate.Model, error) {
	if r.model == nil || r.model.AggregateID() != id {
		return nil, nil
	}
	if scope != nil && r.model.UserID() != *scope {
		return nil, nil
	}
	return r.model, nil
}
func (r *updateModelRepo) Update(_ context.Context, m *aggregate.Model) error {
	r.updated = m
	return nil
}
func (r *updateModelRepo) FindByAlias(context.Context, vo.EndpointAlias, *uint) ([]*aggregate.Model, error) {
	return nil, nil
}
func (r *updateModelRepo) Create(context.Context, *aggregate.Model, uint) (uint, error) {
	return 0, nil
}
func (r *updateModelRepo) Delete(context.Context, uint, *uint) error      { return nil }
func (r *updateModelRepo) DeleteByEndpointID(context.Context, uint) error { return nil }
func (r *updateModelRepo) List(context.Context) ([]*aggregate.Model, error) {
	return nil, nil
}
func (r *updateModelRepo) ListByEndpointIDs(context.Context, []uint) ([]*aggregate.Model, error) {
	return nil, nil
}
func (r *updateModelRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

var _ llmproxy.ModelRepository = (*updateModelRepo)(nil)

func (r *updateModelRepo) ReplaceHistoricalModelID(context.Context, uint, string, string) (llmproxy.ModelIDSyncCounts, error) {
	return llmproxy.ModelIDSyncCounts{}, nil
}

func (r *updateModelRepo) PaginateWithFilter(context.Context, model.CommonParam, llmproxy.ModelListFilter, *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

func mustOwnedModel(t *testing.T) *aggregate.Model {
	t.Helper()
	m, err := aggregate.CreateModel(1, vo.EndpointAlias("gpt-x"), "up-x", 1, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	m.SetUserID(101) // 全部用例的 model 归属固定为用户 101
	return m
}

func updateCmd(scope, endpointID *uint) port.UpdateModelCommand {
	upstream := "up-x-new"
	return port.UpdateModelCommand{
		ScopeUserID:   scope,
		ID:            1,
		EndpointID:    endpointID,
		UpstreamModel: &upstream,
	}
}

// 用户 A（101）把 model 换绑到用户 B（202）的 endpoint → 拒绝。
// scope 内查不到他人 endpoint，返回 ErrDataNotExists（不泄露资源存在性）。
func TestUpdateModel_CrossTenantEndpointRejected(t *testing.T) {
	t.Parallel()
	repo := &updateModelRepo{model: mustOwnedModel(t)}
	h := command.NewUpdateModelHandler(newScopedEndpointRepo(), repo)

	_, err := h.Handle(context.Background(), updateCmd(uptr(101), uptr(2)))
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("cross-tenant endpoint swap must be ErrDataNotExists, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("model must not be persisted on rejection")
	}
}

// admin 全量视角也不能把 model 挂到不同 owner 的 endpoint（防误操作造跨用户挂载）。
func TestUpdateModel_AdminOwnerMismatchRejected(t *testing.T) {
	t.Parallel()
	repo := &updateModelRepo{model: mustOwnedModel(t)}
	h := command.NewUpdateModelHandler(newScopedEndpointRepo(), repo)

	_, err := h.Handle(context.Background(), updateCmd(nil, uptr(2)))
	if !errors.Is(err, ierr.ErrNoPermission) {
		t.Fatalf("admin mismatched swap must be ErrNoPermission, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("model must not be persisted on rejection")
	}
}

// 换绑到自己名下的 endpoint → 成功。
func TestUpdateModel_OwnEndpointSwapAllowed(t *testing.T) {
	t.Parallel()
	repo := &updateModelRepo{model: mustOwnedModel(t)}
	h := command.NewUpdateModelHandler(newScopedEndpointRepo(), repo)

	if _, err := h.Handle(context.Background(), updateCmd(uptr(101), uptr(1))); err != nil {
		t.Fatalf("own endpoint swap should succeed: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("update should be persisted")
	}
}

// 目标 endpoint 不存在（scope 内不可见）→ ErrDataNotExists。
func TestUpdateModel_TargetEndpointNotFound(t *testing.T) {
	t.Parallel()
	repo := &updateModelRepo{model: mustOwnedModel(t)}
	h := command.NewUpdateModelHandler(newScopedEndpointRepo(), repo)

	_, err := h.Handle(context.Background(), updateCmd(uptr(101), uptr(999)))
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("want ErrDataNotExists, got %v", err)
	}
}

// 不换绑 endpoint（EndpointID=nil）时不触发归属校验，直接更新成功。
func TestUpdateModel_WithoutEndpointSwapNoCheck(t *testing.T) {
	t.Parallel()
	repo := &updateModelRepo{model: mustOwnedModel(t)}
	h := command.NewUpdateModelHandler(newScopedEndpointRepo(), repo)

	if _, err := h.Handle(context.Background(), updateCmd(uptr(101), nil)); err != nil {
		t.Fatalf("update without endpoint swap should succeed: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("update should be persisted")
	}
}
