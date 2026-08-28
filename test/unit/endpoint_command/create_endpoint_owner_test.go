// Package endpoint_command 验证 endpoint 创建命令的归属计算与用户隔离。
package endpoint_command

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/command"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

type capturingEndpointRepo struct {
	gotOwner uint
}

func (f *capturingEndpointRepo) FindByID(context.Context, uint, uint) (*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *capturingEndpointRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *capturingEndpointRepo) Create(_ context.Context, _ *aggregate.Endpoint, ownerUserID uint) (uint, error) {
	f.gotOwner = ownerUserID
	return 1, nil
}
func (f *capturingEndpointRepo) Update(context.Context, *aggregate.Endpoint) error { return nil }
func (f *capturingEndpointRepo) Delete(context.Context, uint, uint) error          { return nil }
func (f *capturingEndpointRepo) DeleteCascade(context.Context, uint, uint) error   { return nil }
func (f *capturingEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *capturingEndpointRepo) Paginate(context.Context, model.CommonParam, uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

func (f *capturingEndpointRepo) FindIDsByScope(context.Context, uint) ([]uint, error) {
	return nil, nil
}

var _ llmproxy.EndpointRepository = (*capturingEndpointRepo)(nil)

func TestCreateEndpoint_RejectsZeroOwner(t *testing.T) {
	t.Parallel()
	repo := &capturingEndpointRepo{}
	h := command.NewCreateEndpointHandler(repo)

	cmd := port.CreateEndpointCommand{OwnerUserID: 0, Name: "ep", APIKey: "k", OpenaiBaseURL: "https://o.example.com"}
	cmd.SupportOpenAIChatCompletion = true
	if _, err := h.Handle(context.Background(), cmd); err == nil {
		t.Fatal("zero owner must be rejected")
	}
}

func TestCreateEndpoint_PassesOwnerToRepo(t *testing.T) {
	t.Parallel()
	repo := &capturingEndpointRepo{}
	h := command.NewCreateEndpointHandler(repo)

	cmd := port.CreateEndpointCommand{
		OwnerUserID:                 303,
		Name:                        "ep",
		APIKey:                      "k",
		OpenaiBaseURL:               "https://o.example.com",
		SupportOpenAIChatCompletion: true,
	}
	if _, err := h.Handle(context.Background(), cmd); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if repo.gotOwner != 303 {
		t.Fatalf("repo got owner %d, want 303", repo.gotOwner)
	}
}
