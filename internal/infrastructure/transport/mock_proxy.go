package transport

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// mockRNG mock 随机源；math/rand/v2 的 *rand.Rand 并发安全，可被并发请求共享。
// 内容仅为压测用随机字符，非安全用途，故不使用 crypto/rand。
var mockRNG = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // mock content is not security-sensitive

// mockMaxTokensField 请求体中与输出 token 上限相关的字段。
type mockMaxTokensField struct {
	MaxTokens       int `json:"max_tokens"`
	MaxOutputTokens int `json:"max_output_tokens"`
}

// IsMock 判断该上游端点是否为 mock 目标：开关开启且 upstream model 为 mock 标记。
func IsMock(ep vo.UpstreamEndpoint) bool {
	return config.MockUpstreamEnabled && ep.Model == constant.MockUpstreamModel
}

// MockStream 承载 mock 参数的假流。Read 恒返回 EOF——真实内容由 Read*Stream
// 依 Spec 本地生成，不经过字节流序列化往返。
type MockStream struct {
	io.Reader
	Spec  MockSpec
	Model string
}

// NewMockStream 构造承载 mock 参数的假流。
func NewMockStream(spec MockSpec, model string) *MockStream {
	return &MockStream{Reader: strings.NewReader(""), Spec: spec, Model: model}
}

// Close 实现 io.ReadCloser；假流无资源可释放。
func (s *MockStream) Close() error { return nil }

// mockOpenAIProxy 包装真实 OpenAIProxyPort，命中 mock 标记时本地生成响应。
type mockOpenAIProxy struct{ inner usecase.OpenAIProxyPort }

// NewMockOpenAIProxy 构造带 mock 能力的 OpenAI 上游代理。
func NewMockOpenAIProxy(inner usecase.OpenAIProxyPort) usecase.OpenAIProxyPort {
	return &mockOpenAIProxy{inner: inner}
}

func (p *mockOpenAIProxy) ForwardChatCompletion(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error) {
	if !IsMock(ep) {
		return p.inner.ForwardChatCompletion(ctx, ep, body)
	}
	return BuildChatCompletion(NewMockSpec(mockRNG, maxTokensFromBody(body)), ep.Model), nil
}

func (p *mockOpenAIProxy) OpenChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !IsMock(ep) {
		return p.inner.OpenChatCompletionStream(ctx, ep, body)
	}
	return NewMockStream(NewMockSpec(mockRNG, maxTokensFromBody(body)), ep.Model), nil
}

func (p *mockOpenAIProxy) ReadChatCompletionStream(ctx context.Context, stream io.ReadCloser, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	ms, ok := stream.(*MockStream)
	if !ok {
		return p.inner.ReadChatCompletionStream(ctx, stream, onChunk)
	}
	defer func() { _ = stream.Close() }() //nolint:errcheck // ensure mock stream closed on return
	return emitChatStream(ctx, ms, onChunk)
}

func (p *mockOpenAIProxy) ForwardCreateResponse(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	if !IsMock(ep) {
		return p.inner.ForwardCreateResponse(ctx, ep, body)
	}
	return sonic.Marshal(BuildResponse(NewMockSpec(mockRNG, maxTokensFromBody(body)), ep.Model))
}

// ForwardCreateDecision 不在 mock 范围（mock Endpoint 不开该能力），原样透传。
func (p *mockOpenAIProxy) ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	return p.inner.ForwardCreateDecision(ctx, ep, body)
}

func (p *mockOpenAIProxy) OpenCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !IsMock(ep) {
		return p.inner.OpenCreateResponseStream(ctx, ep, body)
	}
	return NewMockStream(NewMockSpec(mockRNG, maxTokensFromBody(body)), ep.Model), nil
}

func (p *mockOpenAIProxy) ReadCreateResponseStream(ctx context.Context, stream io.ReadCloser, onEvent func(event string, data []byte) error) error {
	ms, ok := stream.(*MockStream)
	if !ok {
		return p.inner.ReadCreateResponseStream(ctx, stream, onEvent)
	}
	defer func() { _ = stream.Close() }() //nolint:errcheck // ensure mock stream closed on return
	return emitResponseStream(ctx, ms, onEvent)
}

// mockAnthropicProxy 包装真实 AnthropicProxyPort，命中 mock 标记时本地生成响应。
type mockAnthropicProxy struct{ inner usecase.AnthropicProxyPort }

// NewMockAnthropicProxy 构造带 mock 能力的 Anthropic 上游代理。
func NewMockAnthropicProxy(inner usecase.AnthropicProxyPort) usecase.AnthropicProxyPort {
	return &mockAnthropicProxy{inner: inner}
}

func (p *mockAnthropicProxy) ForwardCreateMessage(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error) {
	if !IsMock(ep) {
		return p.inner.ForwardCreateMessage(ctx, ep, body)
	}
	return BuildMessage(NewMockSpec(mockRNG, maxTokensFromBody(body)), ep.Model), nil
}

func (p *mockAnthropicProxy) OpenCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !IsMock(ep) {
		return p.inner.OpenCreateMessageStream(ctx, ep, body)
	}
	return NewMockStream(NewMockSpec(mockRNG, maxTokensFromBody(body)), ep.Model), nil
}

func (p *mockAnthropicProxy) ReadCreateMessageStream(ctx context.Context, stream io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	ms, ok := stream.(*MockStream)
	if !ok {
		return p.inner.ReadCreateMessageStream(ctx, stream, onEvent)
	}
	defer func() { _ = stream.Close() }() //nolint:errcheck // ensure mock stream closed on return
	if err := waitDelay(ctx, ms.Spec.TTFT); err != nil {
		return nil, err
	}
	for _, ev := range BuildMessageStream(ms.Spec, ms.Model, mockMessageID()) {
		if err := onEvent(dto.AnthropicSSEEvent{Event: ev.Event, Data: ev.Data}); err != nil {
			return nil, err
		}
		if err := waitDelay(ctx, ms.Spec.Interval); err != nil {
			return nil, err
		}
	}
	return BuildMessage(ms.Spec, ms.Model), nil
}

func (p *mockAnthropicProxy) ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error) {
	if !IsMock(ep) {
		return p.inner.ForwardCountTokens(ctx, ep, body)
	}
	return BuildTokensCount(NewMockSpec(mockRNG, maxTokensFromBody(body))), nil
}

// emitChatStream 按 Spec 的 TTFT/TPS 时序吐出 Chat chunk 并聚合出 completion。
func emitChatStream(ctx context.Context, ms *MockStream, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	if err := waitDelay(ctx, ms.Spec.TTFT); err != nil {
		return nil, err
	}
	for _, chunk := range BuildChatStream(ms.Spec, ms.Model, mockChunkID()) {
		if err := onChunk(chunk); err != nil {
			return nil, err
		}
		if err := waitDelay(ctx, ms.Spec.Interval); err != nil {
			return nil, err
		}
	}
	return BuildChatCompletion(ms.Spec, ms.Model), nil
}

// emitResponseStream 按时序吐出 Responses 事件。
func emitResponseStream(ctx context.Context, ms *MockStream, onEvent func(event string, data []byte) error) error {
	if err := waitDelay(ctx, ms.Spec.TTFT); err != nil {
		return err
	}
	for _, ev := range BuildResponseStream(ms.Spec, ms.Model, mockResponseID()) {
		if err := onEvent(ev.Event, ev.Data); err != nil {
			return err
		}
		if err := waitDelay(ctx, ms.Spec.Interval); err != nil {
			return err
		}
	}
	return nil
}

// waitDelay 等待一段时序间隔（TTFT 或 1/TPS）；ctx 取消则立即返回。
func waitDelay(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// maxTokensFromBody 从请求体提取输出 token 上限（max_tokens / max_output_tokens），缺失返回 0。
func maxTokensFromBody(body []byte) int {
	var req mockMaxTokensField
	if err := sonic.Unmarshal(body, &req); err != nil {
		return 0
	}
	return max(req.MaxTokens, req.MaxOutputTokens)
}

// mockChunkID 生成 Chat 流式 chunk ID。
func mockChunkID() string {
	return fmt.Sprintf(constant.OpenAIChunkIDTemplate, strconv.FormatInt(time.Now().UnixNano(), 10))
}

// mockResponseID 生成 Responses 响应 ID。
func mockResponseID() string {
	return fmt.Sprintf(constant.ResponseIDTemplate, strconv.FormatInt(time.Now().UnixNano(), 10))
}

// mockMessageID 生成 Anthropic 消息 ID。
func mockMessageID() string {
	return fmt.Sprintf(constant.AnthropicMessageIDTemplate, strconv.FormatInt(time.Now().UnixNano(), 10))
}
