// Package endpoint_query 验证 endpoint 列表查询的 demo 视角脱敏。
package endpoint_query

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/port"
	"github.com/hcd233/aris-proxy-api/internal/application/endpoint/query"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
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
	}), nil
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
