package transport_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// fakeOpenAIProxy 记录内层代理是否被调用，用于验证非 mock 请求零透传改变。
type fakeOpenAIProxy struct{ called bool }

var _ usecase.OpenAIProxyPort = (*fakeOpenAIProxy)(nil)

func (f *fakeOpenAIProxy) ForwardChatCompletion(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error) {
	f.called = true
	return &dto.OpenAIChatCompletion{Object: "chat.completion"}, nil
}
func (f *fakeOpenAIProxy) OpenChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	f.called = true
	return io.NopCloser(strings.NewReader("")), nil
}
func (f *fakeOpenAIProxy) ReadChatCompletionStream(ctx context.Context, stream io.ReadCloser, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	f.called = true
	return nil, nil
}
func (f *fakeOpenAIProxy) ForwardCreateResponse(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	f.called = true
	return []byte(`{"object":"response"}`), nil
}
func (f *fakeOpenAIProxy) ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	f.called = true
	return []byte(`{}`), nil
}
func (f *fakeOpenAIProxy) OpenCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	f.called = true
	return io.NopCloser(strings.NewReader("")), nil
}
func (f *fakeOpenAIProxy) ReadCreateResponseStream(ctx context.Context, stream io.ReadCloser, onEvent func(event string, data []byte) error) error {
	f.called = true
	return nil
}

// fakeAnthropicProxy 同上，用于 Anthropic 装饰器。
type fakeAnthropicProxy struct{ called bool }

var _ usecase.AnthropicProxyPort = (*fakeAnthropicProxy)(nil)

func (f *fakeAnthropicProxy) ForwardCreateMessage(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error) {
	f.called = true
	return &dto.AnthropicMessage{Type: "message"}, nil
}
func (f *fakeAnthropicProxy) OpenCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	f.called = true
	return io.NopCloser(strings.NewReader("")), nil
}
func (f *fakeAnthropicProxy) ReadCreateMessageStream(ctx context.Context, stream io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	f.called = true
	return nil, nil
}
func (f *fakeAnthropicProxy) ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error) {
	f.called = true
	return &dto.AnthropicTokensCount{}, nil
}

func mockEndpoint() vo.UpstreamEndpoint {
	return vo.NewUpstreamEndpointFromCredential(constant.MockUpstreamModel, "k", "http://mock.invalid")
}

func realEndpoint() vo.UpstreamEndpoint {
	return vo.NewUpstreamEndpointFromCredential("gpt-4o", "k", "http://real.example")
}

// TestIsMock_DisabledByDefault 验证开关关闭时即使模型是 mock 标记也不走 mock。
func TestIsMock_DisabledByDefault(t *testing.T) { //nolint:paralleltest // modifies global config
	if transport.IsMock(mockEndpoint()) {
		t.Fatal("IsMock must be false when switch is off")
	}
}

// TestIsMock_EnabledWithMarker 验证开关开启 + mock 标记才走 mock。
func TestIsMock_EnabledWithMarker(t *testing.T) { //nolint:paralleltest // modifies global config
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	if !transport.IsMock(mockEndpoint()) {
		t.Fatal("IsMock must be true when switch on and model is mock marker")
	}
	if transport.IsMock(realEndpoint()) {
		t.Fatal("IsMock must be false for real model")
	}
}

// TestMockOpenAIProxy_UsesMock 验证 mock 目标走本地生成、不触达内层代理。
func TestMockOpenAIProxy_UsesMock(t *testing.T) { //nolint:paralleltest // modifies global config
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	inner := &fakeOpenAIProxy{}
	p := transport.NewMockOpenAIProxy(inner)

	rsp, err := p.ForwardChatCompletion(t.Context(), mockEndpoint(), []byte(`{"model":"mock-chat"}`))
	if err != nil {
		t.Fatal(err)
	}
	if rsp == nil || rsp.Object != "chat.completion" {
		t.Fatalf("mock response malformed: %+v", rsp)
	}
	if inner.called {
		t.Fatal("inner proxy must not be called for mock target")
	}
}

// TestMockOpenAIProxy_Passthrough 验证真实模型原样透传到内层代理。
func TestMockOpenAIProxy_Passthrough(t *testing.T) { //nolint:paralleltest // modifies global config
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	inner := &fakeOpenAIProxy{}
	p := transport.NewMockOpenAIProxy(inner)

	if _, err := p.ForwardChatCompletion(t.Context(), realEndpoint(), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if !inner.called {
		t.Fatal("inner proxy must be called for real target")
	}
}

// TestMockAnthropicProxy_UsesMock 验证 Anthropic 装饰器的 mock 分发。
func TestMockAnthropicProxy_UsesMock(t *testing.T) { //nolint:paralleltest // modifies global config
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	inner := &fakeAnthropicProxy{}
	p := transport.NewMockAnthropicProxy(inner)

	msg, err := p.ForwardCreateMessage(t.Context(), mockEndpoint(), []byte(`{"model":"mock-messages"}`))
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || len(msg.Content) == 0 {
		t.Fatalf("mock message malformed: %+v", msg)
	}
	if inner.called {
		t.Fatal("inner proxy must not be called for mock target")
	}
}

// TestMockAnthropicProxy_CountTokens 验证 count_tokens 对 mock 目标返回估算值。
func TestMockAnthropicProxy_CountTokens(t *testing.T) { //nolint:paralleltest // modifies global config
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	inner := &fakeAnthropicProxy{}
	p := transport.NewMockAnthropicProxy(inner)

	count, err := p.ForwardCountTokens(t.Context(), mockEndpoint(), []byte(`{"model":"mock-messages"}`))
	if err != nil {
		t.Fatal(err)
	}
	if count == nil || count.InputTokens <= 0 {
		t.Fatalf("mock count_tokens must return >0, got %+v", count)
	}
	if inner.called {
		t.Fatal("inner proxy must not be called for mock target")
	}
}
