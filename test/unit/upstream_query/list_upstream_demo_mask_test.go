package upstream_query

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/upstream/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	useragg "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	llmvo "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// demo 脱敏守护：demo 视角（IsDemo）下 endpoint 的 base URL 与 model 的
// upstreamModel 走 MaskSecret 掩码，APIKey 只输出 MaskedAPIKey（恒掩码）；
// 非 demo 视角 base URL / upstreamModel 原样返回。
// 覆盖 d80f04d5 重构时删除未迁移的 model_query/list_models_demo_test.go 语义。

const (
	demoMaskOpenaiBaseURL    = "https://openai.example.com/v1"
	demoMaskAnthropicBaseURL = "https://anthropic.example.com/v1"
	demoMaskAPIKey           = "sk-demo-mask-key-123"
	demoMaskUpstreamModel    = "upstream-real-model"
)

// newDemoMaskFixture 单端点单模型：密钥类字段全部使用可辨识的明文常量
func newDemoMaskFixture(t *testing.T) port.ListUpstreamHandler {
	t.Helper()
	ep, err := llmagg.CreateEndpoint(epA, "ep-demo", demoMaskOpenaiBaseURL, demoMaskAnthropicBaseURL, demoMaskAPIKey, true, true, true, false)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	ep.SetUserID(uAlice)
	ep.SetTimestamps(testTime(), testTime())

	m, err := llmagg.CreateModel(201, llmvo.EndpointAlias("gpt-4o"), demoMaskUpstreamModel, epA, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	m.SetUserID(uAlice)
	m.SetTimestamps(testTime(), testTime())

	return newHandler(t,
		map[uint]*llmagg.Endpoint{epA: ep},
		[]*llmagg.Model{m},
		map[uint]*useragg.User{uAlice: alice(t)})
}

// handleDemoMaskGroup 取 fixture 唯一分组的 endpoint/model 视图
func handleDemoMaskGroup(t *testing.T, h port.ListUpstreamHandler, isDemo bool) (epView *port.UpstreamEndpointView, modelView *port.UpstreamModelView) {
	t.Helper()
	groups, _, _, err := h.Handle(context.Background(), port.ListUpstreamQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
		IsDemo:      isDemo,
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Models) != 1 {
		t.Fatalf("expected 1 group with 1 model, got %d groups", len(groups))
	}
	return groups[0].Endpoint, groups[0].Models[0]
}

// TestListUpstream_DemoMasksSecrets demo 视角：base URL / upstreamModel 掩码，
// APIKey 恒以 MaskedAPIKey 输出（掩码形态）。
func TestListUpstream_DemoMasksSecrets(t *testing.T) {
	t.Parallel()

	epView, modelView := handleDemoMaskGroup(t, newDemoMaskFixture(t), true)

	if epView.OpenaiBaseURL != commonutil.MaskSecret(demoMaskOpenaiBaseURL) {
		t.Errorf("demo OpenaiBaseURL = %q, want masked %q", epView.OpenaiBaseURL, commonutil.MaskSecret(demoMaskOpenaiBaseURL))
	}
	if epView.AnthropicBaseURL != commonutil.MaskSecret(demoMaskAnthropicBaseURL) {
		t.Errorf("demo AnthropicBaseURL = %q, want masked %q", epView.AnthropicBaseURL, commonutil.MaskSecret(demoMaskAnthropicBaseURL))
	}
	if epView.MaskedAPIKey != commonutil.MaskSecret(demoMaskAPIKey) {
		t.Errorf("demo MaskedAPIKey = %q, want masked %q", epView.MaskedAPIKey, commonutil.MaskSecret(demoMaskAPIKey))
	}
	if modelView.UpstreamModel != commonutil.MaskSecret(demoMaskUpstreamModel) {
		t.Errorf("demo UpstreamModel = %q, want masked %q", modelView.UpstreamModel, commonutil.MaskSecret(demoMaskUpstreamModel))
	}
	// 掩码后不得回吐明文
	if epView.OpenaiBaseURL == demoMaskOpenaiBaseURL || epView.AnthropicBaseURL == demoMaskAnthropicBaseURL ||
		epView.MaskedAPIKey == demoMaskAPIKey || modelView.UpstreamModel == demoMaskUpstreamModel {
		t.Errorf("demo view leaked raw secret: ep=%+v model.UpstreamModel=%q", epView, modelView.UpstreamModel)
	}
}

// TestListUpstream_NonDemoKeepsRawSecrets 非 demo 视角：base URL / upstreamModel
// 原样返回；MaskedAPIKey 字段本身恒为掩码输出，两个视角一致。
func TestListUpstream_NonDemoKeepsRawSecrets(t *testing.T) {
	t.Parallel()

	epView, modelView := handleDemoMaskGroup(t, newDemoMaskFixture(t), false)

	if epView.OpenaiBaseURL != demoMaskOpenaiBaseURL {
		t.Errorf("non-demo OpenaiBaseURL = %q, want raw %q", epView.OpenaiBaseURL, demoMaskOpenaiBaseURL)
	}
	if epView.AnthropicBaseURL != demoMaskAnthropicBaseURL {
		t.Errorf("non-demo AnthropicBaseURL = %q, want raw %q", epView.AnthropicBaseURL, demoMaskAnthropicBaseURL)
	}
	if modelView.UpstreamModel != demoMaskUpstreamModel {
		t.Errorf("non-demo UpstreamModel = %q, want raw %q", modelView.UpstreamModel, demoMaskUpstreamModel)
	}
	if epView.MaskedAPIKey != commonutil.MaskSecret(demoMaskAPIKey) {
		t.Errorf("non-demo MaskedAPIKey = %q, want masked %q", epView.MaskedAPIKey, commonutil.MaskSecret(demoMaskAPIKey))
	}
}
