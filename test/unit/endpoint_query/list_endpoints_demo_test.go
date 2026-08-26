// Package endpoint_query 验证 endpoint 列表查询的 demo 视角脱敏。
package endpoint_query

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	useragg "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

type fakeEndpointRepo struct {
	endpoints []*aggregate.Endpoint
	page      *model.PageInfo
}

func (f *fakeEndpointRepo) Paginate(_ context.Context, _ model.CommonParam, _ uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return f.endpoints, f.page, nil
}
func (f *fakeEndpointRepo) FindByID(context.Context, uint, uint) (*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *fakeEndpointRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *fakeEndpointRepo) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 0, nil
}
func (f *fakeEndpointRepo) Update(context.Context, *aggregate.Endpoint) error   { return nil }
func (f *fakeEndpointRepo) Delete(context.Context, uint, uint) error            { return nil }
func (f *fakeEndpointRepo) DeleteCascade(context.Context, uint, uint) error     { return nil }
func (f *fakeEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) { return nil, nil }

func (f *fakeEndpointRepo) FindIDsByScope(context.Context, uint) ([]uint, error) {
	return nil, nil
}

var _ llmproxy.EndpointRepository = (*fakeEndpointRepo)(nil)

const (
	endpointName     = "prod-endpoint"
	openaiBaseURL    = "https://openai.example.com/v1"
	anthropicBaseURL = "https://anthropic.example.com/v1"
)

func newDemoFixture() (port.ListEndpointsHandler, error) {
	ep, err := aggregate.CreateEndpoint(1, endpointName, openaiBaseURL, anthropicBaseURL, "sk-secret-key-123", true, true, true)
	if err != nil {
		return nil, err
	}
	return query.NewListEndpointsHandler(&fakeEndpointRepo{
		endpoints: []*aggregate.Endpoint{ep},
		page:      &model.PageInfo{Page: 1, PageSize: 20, Total: 1},
	}, &fakeUserRepo{}), nil
}

// TestListEndpoints_DemoMasksBaseURLs demo 视角须脱敏 BaseURL，Name 保持明文（APIKey 已脱敏）。
func TestListEndpoints_DemoMasksBaseURLs(t *testing.T) {
	t.Parallel()

	h, err := newDemoFixture()
	if err != nil {
		t.Fatalf("fixture failed: %v", err)
	}

	views, page, err := h.Handle(context.Background(), port.ListEndpointsQuery{IsDemo: true})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if page == nil || page.Total != 1 {
		t.Fatalf("unexpected page: %+v", page)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	v := views[0]
	if v.OpenaiBaseURL != commonutil.MaskSecret(openaiBaseURL) {
		t.Fatalf("OpenaiBaseURL not masked: got %q, want %q", v.OpenaiBaseURL, commonutil.MaskSecret(openaiBaseURL))
	}
	if v.AnthropicBaseURL != commonutil.MaskSecret(anthropicBaseURL) {
		t.Fatalf("AnthropicBaseURL not masked: got %q, want %q", v.AnthropicBaseURL, commonutil.MaskSecret(anthropicBaseURL))
	}
	// demo 视角 Name 不脱敏（供演示辨认 endpoint）。
	if v.Name != endpointName {
		t.Fatalf("Endpoint.Name unexpectedly masked: got %q, want %q", v.Name, endpointName)
	}
	if v.MaskedAPIKey != commonutil.MaskSecret("sk-secret-key-123") {
		t.Fatalf("MaskedAPIKey unexpected: %q", v.MaskedAPIKey)
	}
}

// TestListEndpoints_NonDemoKeepsRawValues 非 demo 视角应保持原始值（回归护栏）。
func TestListEndpoints_NonDemoKeepsRawValues(t *testing.T) {
	t.Parallel()

	h, err := newDemoFixture()
	if err != nil {
		t.Fatalf("fixture failed: %v", err)
	}

	views, _, err := h.Handle(context.Background(), port.ListEndpointsQuery{IsDemo: false})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	v := views[0]
	if v.OpenaiBaseURL != openaiBaseURL {
		t.Fatalf("OpenaiBaseURL unexpectedly changed: got %q, want %q", v.OpenaiBaseURL, openaiBaseURL)
	}
	if v.AnthropicBaseURL != anthropicBaseURL {
		t.Fatalf("AnthropicBaseURL unexpectedly changed: got %q, want %q", v.AnthropicBaseURL, anthropicBaseURL)
	}
}

// fakeUserRepo 空实现：demo 脱敏测试不触发 username 回填路径。
type fakeUserRepo struct{}

func (f *fakeUserRepo) FindByID(context.Context, uint) (*useragg.User, error) { return nil, nil }
func (f *fakeUserRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*useragg.User, error) {
	return map[uint]*useragg.User{}, nil
}
func (f *fakeUserRepo) Create(context.Context, *useragg.User) (uint, error) { return 0, nil }
func (f *fakeUserRepo) Update(context.Context, *useragg.User) error         { return nil }
func (f *fakeUserRepo) Save(context.Context, *useragg.User) error           { return nil }
func (f *fakeUserRepo) FindByGithubBindID(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) FindByGoogleBindID(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) FindByPermission(context.Context, enum.Permission) (*useragg.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) FindByName(context.Context, string) (*useragg.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) ReplaceDemoUser(context.Context, uint) (uint, error) { return 0, nil }
func (f *fakeUserRepo) TouchLastLogin(context.Context, uint) error          { return nil }
func (f *fakeUserRepo) ListUsers(context.Context, model.CommonParam, enum.Permission) ([]*useragg.User, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *fakeUserRepo) DeleteCascade(context.Context, uint) error { return nil }

var _ identity.UserRepository = (*fakeUserRepo)(nil)
