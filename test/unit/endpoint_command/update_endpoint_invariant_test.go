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

// endpointRepoStub 按预置聚合根应答 FindByID，并记录 Update 调用次数。
type endpointRepoStub struct {
	ep         *aggregate.Endpoint
	updateCall int
}

func (s *endpointRepoStub) FindByID(context.Context, uint, *uint) (*aggregate.Endpoint, error) {
	return s.ep, nil
}

func (s *endpointRepoStub) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return nil, nil
}

func (s *endpointRepoStub) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) {
	return 0, nil
}

func (s *endpointRepoStub) Update(_ context.Context, ep *aggregate.Endpoint) error {
	s.updateCall++
	s.ep = ep
	return nil
}

func (s *endpointRepoStub) Delete(context.Context, uint, *uint) error        { return nil }
func (s *endpointRepoStub) DeleteCascade(context.Context, uint, *uint) error { return nil }

func (s *endpointRepoStub) List(context.Context) ([]*aggregate.Endpoint, error) { return nil, nil }

func (s *endpointRepoStub) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}

func (s *endpointRepoStub) FindIDsByScope(context.Context, *uint, bool) ([]uint, error) {
	return nil, nil
}

var _ llmproxy.EndpointRepository = (*endpointRepoStub)(nil)

// inconsistentEndpoint 复刻事故行的持久化状态：OpenAI 侧齐全、Anthropic 能力未开且 baseURL 为空。
func inconsistentEndpoint() *aggregate.Endpoint {
	return aggregate.RehydrateEndpoint(6, "tokenhub", "https://tokenhub.example.com/v1", "", "sk-test", true, true, false, false, enum.EndpointAuthAPIKey)
}

// TestUpdateEndpoint_RejectsCapabilityWithoutBaseURL 回归 2026-10-12 tokenhub 事故：
// 只开 Anthropic 能力、不带 anthropic baseURL 的 PATCH 必须在写库前被拒，
// 否则该行变成读不出来的脏行——upstream 列表与代理路由连续报错，连修复入口都进不去。
func TestUpdateEndpoint_RejectsCapabilityWithoutBaseURL(t *testing.T) {
	t.Parallel()
	repo := &endpointRepoStub{ep: inconsistentEndpoint()}
	h := command.NewUpdateEndpointHandler(repo)

	err := h.Handle(t.Context(), port.UpdateEndpointCommand{
		EndpointID:              6,
		Name:                    lo.ToPtr("tokenhub"),
		SupportOpenAIResponse:   lo.ToPtr(true),
		SupportAnthropicMessage: lo.ToPtr(true),
	})
	if err == nil {
		t.Fatal("开启 Anthropic 能力却不给 anthropic baseURL 时必须拒绝")
	}
	if repo.updateCall != 0 {
		t.Fatalf("被拒绝的更新不得写库, repo.Update called %d times", repo.updateCall)
	}
}

// TestUpdateEndpoint_AllowsCapabilityWithBaseURL 同一请求补齐 baseURL 后应放行并落库。
func TestUpdateEndpoint_AllowsCapabilityWithBaseURL(t *testing.T) {
	t.Parallel()
	repo := &endpointRepoStub{ep: inconsistentEndpoint()}
	h := command.NewUpdateEndpointHandler(repo)

	err := h.Handle(t.Context(), port.UpdateEndpointCommand{
		EndpointID:              6,
		AnthropicBaseURL:        lo.ToPtr("https://tokenhub.example.com/v1"),
		SupportAnthropicMessage: lo.ToPtr(true),
	})
	if err != nil {
		t.Fatalf("补齐 baseURL 的更新不应被拒绝: %v", err)
	}
	if repo.updateCall != 1 {
		t.Fatalf("repo.Update called %d times, want 1", repo.updateCall)
	}
}
