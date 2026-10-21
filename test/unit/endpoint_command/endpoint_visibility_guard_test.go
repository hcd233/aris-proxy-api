package endpoint_command

import (
	"context"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

// fakeEndpointRepo 可注入单个端点的仓储 fake（守卫用例专用，勿与 capturingEndpointRepo 合并）。
type fakeEndpointRepo struct {
	ep      *aggregate.Endpoint
	updated bool
	deleted bool
}

func (f *fakeEndpointRepo) FindByID(context.Context, uint, *uint) (*aggregate.Endpoint, error) {
	return f.ep, nil
}

func (f *fakeEndpointRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return nil, nil
}

func (f *fakeEndpointRepo) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 1, nil
}

func (f *fakeEndpointRepo) Update(context.Context, *aggregate.Endpoint) error {
	f.updated = true
	return nil
}

func (f *fakeEndpointRepo) Delete(context.Context, uint, *uint) error {
	f.deleted = true
	return nil
}

func (f *fakeEndpointRepo) DeleteCascade(context.Context, uint, *uint) error {
	f.deleted = true
	return nil
}

func (f *fakeEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}

func (f *fakeEndpointRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *fakeEndpointRepo) FindIDsByScope(context.Context, *uint, bool) ([]uint, error) {
	return nil, nil
}

var _ llmproxy.EndpointRepository = (*fakeEndpointRepo)(nil)

func publicEndpoint(t *testing.T) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(7, "ep", "https://o.example.com", "", "k", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := ep.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatal(err)
	}
	return ep
}

func TestUpdateEndpoint_NonAdminCannotModifyPublic(t *testing.T) {
	t.Parallel()
	repo := &fakeEndpointRepo{ep: publicEndpoint(t)}
	handler := command.NewUpdateEndpointHandler(repo)

	name := "hijack"
	err := handler.Handle(t.Context(), port.UpdateEndpointCommand{
		ScopeUserID: lo.ToPtr(uint(101)),
		EndpointID:  7,
		Name:        &name,
		IsAdmin:     false,
	})
	if err == nil {
		t.Fatal("non-admin must not modify a public endpoint")
	}
	if repo.updated {
		t.Fatal("repo must not be written when guard rejects")
	}
}

func TestUpdateEndpoint_AdminCanModifyPublic(t *testing.T) {
	t.Parallel()
	repo := &fakeEndpointRepo{ep: publicEndpoint(t)}
	handler := command.NewUpdateEndpointHandler(repo)

	name := "ok"
	if err := handler.Handle(t.Context(), port.UpdateEndpointCommand{EndpointID: 7, Name: &name, IsAdmin: true}); err != nil {
		t.Fatalf("admin update failed: %v", err)
	}
	if !repo.updated {
		t.Fatal("admin update must reach repo")
	}
}

func TestDeleteEndpoint_NonAdminCannotDeletePublic(t *testing.T) {
	t.Parallel()
	repo := &fakeEndpointRepo{ep: publicEndpoint(t)}
	handler := command.NewDeleteEndpointHandler(repo)

	if err := handler.Handle(t.Context(), port.DeleteEndpointCommand{ScopeUserID: lo.ToPtr(uint(101)), EndpointID: 7, IsAdmin: false}); err == nil {
		t.Fatal("non-admin must not delete a public endpoint")
	}
	if repo.deleted {
		t.Fatal("repo must not delete when guard rejects")
	}
}

func TestUpdateEndpoint_WithoutVisibilityKeepsCurrent(t *testing.T) {
	t.Parallel()
	repo := &fakeEndpointRepo{ep: publicEndpoint(t)}
	handler := command.NewUpdateEndpointHandler(repo)

	name := "rename"
	if err := handler.Handle(t.Context(), port.UpdateEndpointCommand{EndpointID: 7, Name: &name, IsAdmin: true}); err != nil {
		t.Fatal(err)
	}
	if repo.ep.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatal("update without visibility must preserve current value")
	}
}

// TestUpdateEndpoint_AdminCannotChangeVisibilityOfOthersEndpoint 可见性只能由归属者本人修改：
// admin 不得把他人（含普通用户）的端点发布为公开，否则等于跨租户共享他人的上游密钥/额度。
func TestUpdateEndpoint_AdminCannotChangeVisibilityOfOthersEndpoint(t *testing.T) {
	t.Parallel()
	ep := publicEndpoint(t)
	ep.SetUserID(202)
	repo := &fakeEndpointRepo{ep: ep}
	handler := command.NewUpdateEndpointHandler(repo)

	visibility := enum.EndpointVisibilityPublic
	err := handler.Handle(t.Context(), port.UpdateEndpointCommand{
		EndpointID:     7,
		IsAdmin:        true,
		OperatorUserID: 1,
		Visibility:     &visibility,
	})
	if err == nil {
		t.Fatal("admin must not change visibility of someone else's endpoint")
	}
	if repo.updated {
		t.Fatal("repo must not be written when ownership guard rejects")
	}
}

// TestUpdateEndpoint_OwnerCanChangeVisibility 归属者本人可以切换公开/私有。
func TestUpdateEndpoint_OwnerCanChangeVisibility(t *testing.T) {
	t.Parallel()
	ep := publicEndpoint(t)
	ep.SetUserID(202)
	repo := &fakeEndpointRepo{ep: ep}
	handler := command.NewUpdateEndpointHandler(repo)

	visibility := enum.EndpointVisibilityPrivate
	if err := handler.Handle(t.Context(), port.UpdateEndpointCommand{
		EndpointID:     7,
		IsAdmin:        true,
		OperatorUserID: 202,
		Visibility:     &visibility,
	}); err != nil {
		t.Fatalf("owner change visibility: %v", err)
	}
	if ep.Visibility() != enum.EndpointVisibilityPrivate {
		t.Fatalf("visibility = %q, want private", ep.Visibility())
	}
	if !repo.updated {
		t.Fatal("owner update must reach repo")
	}
}
