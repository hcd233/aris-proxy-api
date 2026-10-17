package transport

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// newMockRNG 每次调用新建独立随机源。
//
// math/rand/v2 的 *rand.Rand（PCG 源）方法无锁读写内部状态，
// 并非并发安全（并发安全的只有包级函数）；共享实例在并发压测下是 data race。
// PCG 构造仅是几个 uint64 赋值，per-request 开销可忽略。
func newMockRNG() *rand.Rand {
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // G404 mock content is not security-sensitive
}

// mockMaxTokensField 请求体中与输出 token 上限相关的字段。
type mockMaxTokensField struct {
	MaxTokens       int `json:"max_tokens"`
	MaxOutputTokens int `json:"max_output_tokens"`
}

// IsMock 判断该上游端点是否为 mock 目标：开关开启且 upstream model 为 mock 标记。
func IsMock(ep vo.UpstreamEndpoint) bool {
	return config.MockUpstreamEnabled && ep.Model == constant.MockUpstreamModel
}

// mockStream 承载 mock 参数的假流。Read 恒返回 EOF——真实内容由 Read*Stream
// 依 Spec 本地生成，不经过字节流序列化往返。
type mockStream struct {
	io.Reader
	Spec  MockSpec
	Model string
}

// newMockStream 构造承载 mock 参数的假流。
func newMockStream(spec MockSpec, model string) *mockStream {
	return &mockStream{Reader: strings.NewReader(""), Spec: spec, Model: model}
}

// Close 实现 io.ReadCloser；假流无资源可释放。
func (s *mockStream) Close() error { return nil }

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
	return BuildChatCompletion(newSpecFromBody(body), ep.Model), nil
}

func (p *mockOpenAIProxy) OpenChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !IsMock(ep) {
		return p.inner.OpenChatCompletionStream(ctx, ep, body)
	}
	return newMockStream(newSpecFromBody(body), ep.Model), nil
}

func (p *mockOpenAIProxy) ReadChatCompletionStream(ctx context.Context, stream io.ReadCloser, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	ms, ok := stream.(*mockStream)
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
	return sonic.Marshal(BuildResponse(newSpecFromBody(body), ep.Model, mockID(constant.ResponseIDTemplate)))
}

// ForwardCreateDecision 不在 mock 范围（mock Endpoint 不开该能力），原样透传。
func (p *mockOpenAIProxy) ForwardCreateDecision(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error) {
	return p.inner.ForwardCreateDecision(ctx, ep, body)
}

func (p *mockOpenAIProxy) OpenCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !IsMock(ep) {
		return p.inner.OpenCreateResponseStream(ctx, ep, body)
	}
	return newMockStream(newSpecFromBody(body), ep.Model), nil
}

func (p *mockOpenAIProxy) ReadCreateResponseStream(ctx context.Context, stream io.ReadCloser, onEvent func(event string, data []byte) error) error {
	ms, ok := stream.(*mockStream)
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
	return BuildMessage(newSpecFromBody(body), ep.Model), nil
}

func (p *mockAnthropicProxy) OpenCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error) {
	if !IsMock(ep) {
		return p.inner.OpenCreateMessageStream(ctx, ep, body)
	}
	return newMockStream(newSpecFromBody(body), ep.Model), nil
}

func (p *mockAnthropicProxy) ReadCreateMessageStream(ctx context.Context, stream io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	ms, ok := stream.(*mockStream)
	if !ok {
		return p.inner.ReadCreateMessageStream(ctx, stream, onEvent)
	}
	defer func() { _ = stream.Close() }() //nolint:errcheck // ensure mock stream closed on return
	return emitMessageStream(ctx, ms, onEvent)
}

func (p *mockAnthropicProxy) ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error) {
	if !IsMock(ep) {
		return p.inner.ForwardCountTokens(ctx, ep, body)
	}
	return BuildTokensCount(estimateInputTokens(body)), nil
}

// emitChatStream 按 Spec 的 TTFT/TPS 时序吐出 Chat chunk 并聚合出 completion。
func emitChatStream(ctx context.Context, ms *mockStream, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error) {
	if err := waitDelay(ctx, ms.Spec.TTFT); err != nil {
		return nil, err
	}
	chunkID := mockID(constant.OpenAIChunkIDTemplate)
	created := time.Now().Unix()
	for i := range ChatStreamLen(ms.Spec) {
		if err := onChunk(BuildChatChunkAt(ms.Spec, ms.Model, chunkID, created, i)); err != nil {
			return nil, err
		}
		if err := waitDelay(ctx, ms.Spec.Interval); err != nil {
			return nil, err
		}
	}
	return BuildChatCompletion(ms.Spec, ms.Model), nil
}

// emitMessageStream 按 Spec 的 TTFT/TPS 时序吐出 Anthropic SSE 事件并聚合出 message。
// 返回 message 的 ID 与流内 message_start 的 ID 一致（同一响应对象）。
func emitMessageStream(ctx context.Context, ms *mockStream, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error) {
	if err := waitDelay(ctx, ms.Spec.TTFT); err != nil {
		return nil, err
	}
	messageID := mockID(constant.AnthropicMessageIDTemplate)
	for i := range MessageStreamLen(ms.Spec) {
		ev := BuildMessageEventAt(ms.Spec, ms.Model, messageID, i)
		if err := onEvent(dto.AnthropicSSEEvent{Event: ev.Event, Data: ev.Data}); err != nil {
			return nil, err
		}
		if err := waitDelay(ctx, ms.Spec.Interval); err != nil {
			return nil, err
		}
	}
	msg := BuildMessage(ms.Spec, ms.Model)
	msg.ID = messageID
	return msg, nil
}

// emitResponseStream 按时序吐出 Responses 事件。
func emitResponseStream(ctx context.Context, ms *mockStream, onEvent func(event string, data []byte) error) error {
	if err := waitDelay(ctx, ms.Spec.TTFT); err != nil {
		return err
	}
	responseID := mockID(constant.ResponseIDTemplate)
	itemID := mockID(constant.ResponseItemIDTemplate)
	for i := range ResponseStreamLen(ms.Spec) {
		ev := BuildResponseEventAt(ms.Spec, ms.Model, responseID, itemID, i)
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

// mockIDSeq mock 对象 ID 的进程内序号：纳秒时间戳在同一纳秒的多并发调用下会撞，
// 序号兜底保证唯一。
var mockIDSeq atomic.Uint64

// mockID 用给定模板生成 mock 对象 ID（纳秒时间戳 + 进程内原子序号）。
func mockID(template string) string {
	seq := strconv.FormatUint(mockIDSeq.Add(1), 10)
	return fmt.Sprintf(template, strconv.FormatInt(time.Now().UnixNano(), 10)+"-"+seq)
}

// newSpecFromBody 从请求体构造 mock 参数：随机内容/时序 + 输入 token 估算。
func newSpecFromBody(body []byte) MockSpec {
	spec := NewMockSpec(newMockRNG(), maxTokensFromBody(body))
	spec.InputTokens = estimateInputTokens(body)
	return spec
}

// estimateInputTokens 从请求 body 估算输入 token 数：按 1 token ≈ 4 字符粗估。
// body 为 JSON、含结构开销，故为粗估口径；对压测审计入账够用。
func estimateInputTokens(body []byte) int {
	return max(1, len(body)/constant.MockCharsPerToken)
}
