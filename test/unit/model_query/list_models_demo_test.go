// Package model_query 验证 model 列表查询的 demo 视角脱敏。
package model_query

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/application/model/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

type fakeModelRepo struct {
	models []*aggregate.Model
	page   *model.PageInfo
}

func (f *fakeModelRepo) Paginate(_ context.Context, _ model.CommonParam, _ uint) ([]*aggregate.Model, *model.PageInfo, error) {
	return f.models, f.page, nil
}
func (f *fakeModelRepo) FindByAlias(context.Context, vo.EndpointAlias, uint) ([]*aggregate.Model, error) {
	return nil, nil
}
func (f *fakeModelRepo) FindByID(context.Context, uint, uint) (*aggregate.Model, error) {
	return nil, nil
}
func (f *fakeModelRepo) Create(context.Context, *aggregate.Model, uint) (uint, error) { return 0, nil }
func (f *fakeModelRepo) Update(context.Context, *aggregate.Model) error               { return nil }
func (f *fakeModelRepo) Delete(context.Context, uint, uint) error                     { return nil }
func (f *fakeModelRepo) DeleteByEndpointID(context.Context, uint) error               { return nil }
func (f *fakeModelRepo) List(context.Context) ([]*aggregate.Model, error)             { return nil, nil }

type fakeEndpointRepo struct {
	endpoints map[uint]*aggregate.Endpoint
}

func (f *fakeEndpointRepo) BatchFindByIDs(_ context.Context, _ []uint) (map[uint]*aggregate.Endpoint, error) {
	return f.endpoints, nil
}
func (f *fakeEndpointRepo) FindByID(context.Context, uint, uint) (*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *fakeEndpointRepo) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 0, nil
}
func (f *fakeEndpointRepo) Update(context.Context, *aggregate.Endpoint) error   { return nil }
func (f *fakeEndpointRepo) Delete(context.Context, uint, uint) error            { return nil }
func (f *fakeEndpointRepo) DeleteCascade(context.Context, uint, uint) error     { return nil }
func (f *fakeEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) { return nil, nil }
func (f *fakeEndpointRepo) Paginate(context.Context, model.CommonParam, uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

var _ llmproxy.ModelRepository = (*fakeModelRepo)(nil)
var _ llmproxy.EndpointRepository = (*fakeEndpointRepo)(nil)

const (
	upstreamModel    = "upstream-real-model"
	endpointName     = "prod-endpoint"
	openaiBaseURL    = "https://openai.example.com/v1"
	anthropicBaseURL = "https://anthropic.example.com/v1"
)

func newDemoFixture() (port.ListModelsHandler, error) {
	ep, err := aggregate.CreateEndpoint(1, endpointName, openaiBaseURL, anthropicBaseURL, "sk-secret-key-123", true, true, true)
	if err != nil {
		return nil, err
	}
	m, err := aggregate.CreateModel(1, vo.EndpointAlias("gpt-4o"), upstreamModel, 1, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		return nil, err
	}

	return query.NewListModelsHandler(
		&fakeModelRepo{models: []*aggregate.Model{m}, page: &model.PageInfo{Page: 1, PageSize: 20, Total: 1}},
		&fakeEndpointRepo{endpoints: map[uint]*aggregate.Endpoint{1: ep}},
	), nil
}

// TestListModels_DemoMasksUpstreamAndEndpoint demo 视角须脱敏 UpstreamModel 及嵌套 Endpoint 的 Name/BaseURL。
func TestListModels_DemoMasksUpstreamAndEndpoint(t *testing.T) {
	t.Parallel()

	h, err := newDemoFixture()
	if err != nil {
		t.Fatalf("fixture failed: %v", err)
	}

	views, page, err := h.Handle(context.Background(), port.ListModelsQuery{IsDemo: true})
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
	if v.UpstreamModel != commonutil.MaskSecret(upstreamModel) {
		t.Fatalf("UpstreamModel not masked: got %q, want %q", v.UpstreamModel, commonutil.MaskSecret(upstreamModel))
	}
	if v.Endpoint == nil {
		t.Fatalf("expected nested endpoint, got nil")
	}
	if v.Endpoint.Name != commonutil.MaskSecret(endpointName) {
		t.Fatalf("Endpoint.Name not masked: got %q, want %q", v.Endpoint.Name, commonutil.MaskSecret(endpointName))
	}
	if v.Endpoint.OpenaiBaseURL != commonutil.MaskSecret(openaiBaseURL) {
		t.Fatalf("Endpoint.OpenaiBaseURL not masked: got %q, want %q", v.Endpoint.OpenaiBaseURL, commonutil.MaskSecret(openaiBaseURL))
	}
	if v.Endpoint.AnthropicBaseURL != commonutil.MaskSecret(anthropicBaseURL) {
		t.Fatalf("Endpoint.AnthropicBaseURL not masked: got %q, want %q", v.Endpoint.AnthropicBaseURL, commonutil.MaskSecret(anthropicBaseURL))
	}
	// APIKey 原本就只输出 MaskedAPIKey，demo 视角不应额外变化。
	if v.Endpoint.MaskedAPIKey != commonutil.MaskSecret("sk-secret-key-123") {
		t.Fatalf("Endpoint.MaskedAPIKey unexpected: %q", v.Endpoint.MaskedAPIKey)
	}
}

// TestListModels_NonDemoKeepsRawValues 非 demo 视角应保持原始值（回归护栏）。
func TestListModels_NonDemoKeepsRawValues(t *testing.T) {
	t.Parallel()

	h, err := newDemoFixture()
	if err != nil {
		t.Fatalf("fixture failed: %v", err)
	}

	views, _, err := h.Handle(context.Background(), port.ListModelsQuery{IsDemo: false})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	v := views[0]
	if v.UpstreamModel != upstreamModel {
		t.Fatalf("UpstreamModel unexpectedly changed: got %q, want %q", v.UpstreamModel, upstreamModel)
	}
	if v.Endpoint == nil {
		t.Fatalf("expected nested endpoint, got nil")
	}
	if v.Endpoint.Name != endpointName {
		t.Fatalf("Endpoint.Name unexpectedly changed: got %q, want %q", v.Endpoint.Name, endpointName)
	}
	if v.Endpoint.OpenaiBaseURL != openaiBaseURL {
		t.Fatalf("Endpoint.OpenaiBaseURL unexpectedly changed: got %q, want %q", v.Endpoint.OpenaiBaseURL, openaiBaseURL)
	}
	if v.Endpoint.AnthropicBaseURL != anthropicBaseURL {
		t.Fatalf("Endpoint.AnthropicBaseURL unexpectedly changed: got %q, want %q", v.Endpoint.AnthropicBaseURL, anthropicBaseURL)
	}
}
