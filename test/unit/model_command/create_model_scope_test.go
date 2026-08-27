// Package model_command 验证 model 创建/更新/删除命令的用户隔离与跨租户校验。
//
// 核心语义：model 归属从其 endpoint 带入（FindByID 以 scopeUserID 过滤），
// 普通用户对他人 endpoint 建 model → endpoint 查不到 → 400 校验拒绝。
package model_command

import (
	"context"
	"testing"

	"errors"
	"github.com/hcd233/aris-proxy-api/internal/application/model/command"
	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	useragg "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// scopedEndpointRepo 内存端点仓储：epA 归 user 101、epB 归 user 202。
type scopedEndpointRepo struct {
	byID map[uint]*aggregate.Endpoint
}

func newScopedEndpointRepo() *scopedEndpointRepo {
	epA, _ := aggregate.CreateEndpoint(1, "ep-a", "https://o.example.com", "https://a.example.com", "k", true, false, false)
	epB, _ := aggregate.CreateEndpoint(2, "ep-b", "https://o.example.com", "https://a.example.com", "k", true, false, false)
	epA.SetUserID(101)
	epB.SetUserID(202)
	return &scopedEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: epA, 2: epB}}
}

func (r *scopedEndpointRepo) FindByID(_ context.Context, id, scopeUserID uint) (*aggregate.Endpoint, error) {
	ep, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	if scopeUserID != 0 && ep.UserID() != scopeUserID {
		return nil, nil
	}
	return ep, nil
}
func (r *scopedEndpointRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return nil, nil
}
func (r *scopedEndpointRepo) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 0, nil
}
func (r *scopedEndpointRepo) Update(context.Context, *aggregate.Endpoint) error { return nil }
func (r *scopedEndpointRepo) Delete(context.Context, uint, uint) error          { return nil }
func (r *scopedEndpointRepo) DeleteCascade(context.Context, uint, uint) error   { return nil }
func (r *scopedEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}
func (r *scopedEndpointRepo) Paginate(context.Context, model.CommonParam, uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

type recordingModelRepo struct {
	gotOwner    uint
	gotEndpoint uint
}

func (r *recordingModelRepo) FindByAlias(context.Context, vo.EndpointAlias, uint) ([]*aggregate.Model, error) {
	return nil, nil
}
func (r *recordingModelRepo) FindByID(context.Context, uint, uint) (*aggregate.Model, error) {
	return nil, nil
}
func (r *recordingModelRepo) Create(_ context.Context, m *aggregate.Model, ownerUserID uint) (uint, error) {
	r.gotOwner = ownerUserID
	r.gotEndpoint = m.EndpointID()
	return 1, nil
}
func (r *recordingModelRepo) Update(context.Context, *aggregate.Model) error { return nil }
func (r *recordingModelRepo) Delete(context.Context, uint, uint) error       { return nil }
func (r *recordingModelRepo) DeleteByEndpointID(context.Context, uint) error { return nil }
func (r *recordingModelRepo) List(context.Context) ([]*aggregate.Model, error) {
	return nil, nil
}
func (r *recordingModelRepo) Paginate(context.Context, model.CommonParam, uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return nil, nil, nil
}

func (r *scopedEndpointRepo) FindIDsByScope(context.Context, uint) ([]uint, error) {
	return nil, nil
}

func (r *recordingModelRepo) ListByEndpointIDs(context.Context, []uint) ([]*aggregate.Model, error) {
	return nil, nil
}

var (
	_ llmproxy.EndpointRepository = (*scopedEndpointRepo)(nil)
	_ llmproxy.ModelRepository    = (*recordingModelRepo)(nil)
	_ identity.UserRepository     = (*noopUserRepo)(nil)
)

type noopUserRepo struct{}

func (f *noopUserRepo) Save(context.Context, *useragg.User) error { return nil }
func (f *noopUserRepo) FindByID(context.Context, uint) (*useragg.User, error) {
	return nil, nil
}
func (f *noopUserRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*useragg.User, error) {
	return map[uint]*useragg.User{}, nil
}
func (f *noopUserRepo) FindByGithubBindID(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *noopUserRepo) FindByGoogleBindID(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *noopUserRepo) FindByPermission(context.Context, enum.Permission) (*useragg.User, error) {
	return nil, nil
}
func (f *noopUserRepo) FindByName(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *noopUserRepo) ReplaceDemoUser(context.Context, uint) (uint, error) { return 0, nil }
func (f *noopUserRepo) TouchLastLogin(context.Context, uint) error          { return nil }
func (f *noopUserRepo) ListUsers(context.Context, model.CommonParam, enum.Permission) ([]*useragg.User, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *noopUserRepo) DeleteCascade(context.Context, uint) error { return nil }

func validCreateCmd(endpointID, scope uint) port.CreateModelCommand {
	return port.CreateModelCommand{
		ScopeUserID:   scope,
		Alias:         "gpt-test",
		UpstreamModel: "up-test",
		EndpointID:    endpointID,
		Capabilities:  []enum.InputModality{enum.InputModalityText},
	}
}

// 用例一：普通用户对自己 endpoint 建 model → 成功且归属=自己。
func TestCreateModel_OwnEndpoint(t *testing.T) {
	t.Parallel()
	repo := &recordingModelRepo{}
	h := command.NewCreateModelHandler(newScopedEndpointRepo(), repo)

	if _, err := h.Handle(context.Background(), validCreateCmd(1, 101)); err != nil {
		t.Fatalf("create on own endpoint failed: %v", err)
	}
	if repo.gotOwner != 101 || repo.gotEndpoint != 1 {
		t.Fatalf("owner=%d endpoint=%d, want owner=101 endpoint=1", repo.gotOwner, repo.gotEndpoint)
	}
}

// 用例二：普通用户对他人 endpoint 建 model → 拒绝。
func TestCreateModel_CrossTenantRejected(t *testing.T) {
	t.Parallel()
	h := command.NewCreateModelHandler(newScopedEndpointRepo(), &recordingModelRepo{})

	_, err := h.Handle(context.Background(), validCreateCmd(2, 101))
	if err == nil {
		t.Fatal("cross-tenant create must be rejected")
	}
	if !errors.Is(err, ierr.ErrDataNotExists) {
		t.Fatalf("want ErrDataNotExists, got %v", err)
	}
}

// admin（scope=0）可对任意用户端点建 model。
func TestCreateModel_AdminScope(t *testing.T) {
	t.Parallel()
	repo := &recordingModelRepo{}
	h := command.NewCreateModelHandler(newScopedEndpointRepo(), repo)

	if _, err := h.Handle(context.Background(), validCreateCmd(2, 0)); err != nil {
		t.Fatalf("admin create failed: %v", err)
	}
	if repo.gotOwner != 202 {
		t.Fatalf("admin-created model owner = %d, want 202 (inherited from endpoint)", repo.gotOwner)
	}
}
