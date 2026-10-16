package transport_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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

// TestMockProxy_ConcurrentSpecNoRace 回归：并发请求下 mock 参数生成无 data race。
//
// 旧实现共享全局 *rand.Rand（PCG 源无锁），并发压测（mock 的主场景）下是
// data race；本用例需在 `go test -race` 下运行才能暴露，常规跑只验证功能。
func TestMockProxy_ConcurrentSpecNoRace(t *testing.T) { //nolint:paralleltest // modifies global config
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	op := transport.NewMockOpenAIProxy(&fakeOpenAIProxy{})
	ap := transport.NewMockAnthropicProxy(&fakeAnthropicProxy{})

	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			rsp, err := op.ForwardChatCompletion(t.Context(), mockEndpoint(), []byte(`{"model":"mock-chat","max_tokens":32}`))
			if err != nil || rsp == nil {
				t.Errorf("chat mock failed: rsp=%v err=%v", rsp, err)
			}
			msg, err := ap.ForwardCreateMessage(t.Context(), mockEndpoint(), []byte(`{"model":"mock-messages"}`))
			if err != nil || msg == nil {
				t.Errorf("message mock failed: msg=%v err=%v", msg, err)
			}
		})
	}
	wg.Wait()
}

// TestMockAnthropicStream_MessageIDConsistent 验证流式返回的 message ID
// 与流内 message_start 事件的 ID 一致（同一响应对象），且每次请求唯一。
func TestMockAnthropicStream_MessageIDConsistent(t *testing.T) { //nolint:paralleltest // modifies global config
	config.MockUpstreamEnabled = true
	t.Cleanup(func() { config.MockUpstreamEnabled = false })

	p := transport.NewMockAnthropicProxy(&fakeAnthropicProxy{})

	var seenIDs []string
	for range 2 {
		stream, err := p.OpenCreateMessageStream(t.Context(), mockEndpoint(), []byte(`{"model":"mock-messages"}`))
		if err != nil {
			t.Fatal(err)
		}
		var startID string
		msg, err := p.ReadCreateMessageStream(t.Context(), stream, func(ev dto.AnthropicSSEEvent) error {
			if ev.Event == enum.AnthropicSSEEventTypeMessageStart {
				var payload struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				}
				if err := sonic.Unmarshal(ev.Data, &payload); err != nil {
					return err
				}
				startID = payload.Message.ID
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if startID == "" {
			t.Fatal("message_start event missing message id")
		}
		if msg.ID != startID {
			t.Fatalf("final message id %q != stream message_start id %q", msg.ID, startID)
		}
		seenIDs = append(seenIDs, msg.ID)
	}
	if seenIDs[0] == seenIDs[1] {
		t.Fatalf("message id must be unique per request, both got %q", seenIDs[0])
	}
}
