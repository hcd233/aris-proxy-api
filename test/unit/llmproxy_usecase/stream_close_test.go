package llmproxy_usecase

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TestStream_CloseWithoutReadClosesUpstreamBody 锁定 port.Stream 契约：
// Close 必须关闭上游 body，即使 Read 从未被调用（如 adapter 注册 stream writer 失败）。
// 修复前 native stream 的 Close 是 no-op（依赖 Read 内部关闭），未 Read 时上游 body
// 与 drain 守护 goroutine 只能等连接自然断掉，属资源泄漏隐患。
func TestStream_CloseWithoutReadClosesUpstreamBody(t *testing.T) {
	t.Parallel()
	proxy := &mockOpenAIProxy{}
	resolver := &mockResolver{resolveEndpoint: buildTestEndpoint(), resolveModel: buildTestModel()}
	uc := usecase.NewOpenAIUseCase(resolver, &mockListModels{}, proxy, &mockAnthropicProxyForOpenAI{}, &mockTaskSubmitter{}, nil, nil)

	stream := true
	req := &dto.OpenAIChatCompletionRequest{Body: &dto.OpenAIChatCompletionReq{
		Model:    "test-alias",
		Messages: []*dto.OpenAIChatCompletionMessageParam{{Role: enum.RoleUser, Content: &dto.OpenAIMessageContent{Text: "Hello"}}},
		Stream:   &stream,
	}}

	result, err := uc.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	streamResult, ok := result.(*port.StreamResult)
	if !ok {
		t.Fatalf("result = %T, want *port.StreamResult", result)
	}

	s, err := streamResult.Open(context.Background())
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	if proxy.chatStreamClosed {
		t.Fatal("upstream body must stay open until Stream.Close")
	}

	// 不调用 Read 直接 Close：覆盖 adapter 兜底关闭路径
	if err := s.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
	if !proxy.chatStreamClosed {
		t.Fatal("Stream.Close must close upstream body when Read is never called")
	}
}
