// Package usecase LLMProxy 域用例层 — 端口定义
package usecase

import (
	"context"
	"io"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TaskSubmitter 异步任务提交端口
//
// 由 infrastructure/pool.PoolManager 实现，避免 application 层直接依赖 infrastructure。
// usecase 通过该接口将模型调用审计和消息存储任务提交到协程池异步执行。
type TaskSubmitter interface {
	SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error
	SubmitMessageStoreTask(task *dto.MessageStoreTask) error
}

// AnthropicProxyPort Anthropic 协议上游代理端口
//
// 由 infrastructure/transport.anthropicProxy 实现。
// 定义在 application 层，确保 application 层不直接依赖 infrastructure 层的接口定义。
type AnthropicProxyPort interface {
	ForwardCreateMessage(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicMessage, error)
	// OpenCreateMessageStream 仅建立上游流式连接；上游在流开始前返回的错误（含状态码）在此暴露，
	// 便于调用方在提交客户端响应状态前透传。
	OpenCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error)
	// ReadCreateMessageStream 消费已打开的上游流，负责关闭 stream。
	ReadCreateMessageStream(ctx context.Context, stream io.ReadCloser, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error)
	ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error)
}

// OpenAIProxyPort OpenAI 协议上游代理端口
//
// 由 infrastructure/transport.openAIProxy 实现。
// 定义在 application 层，确保 application 层不直接依赖 infrastructure 层的接口定义。
type OpenAIProxyPort interface {
	ForwardChatCompletion(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error)
	// OpenChatCompletionStream 仅建立上游流式连接；上游在流开始前返回的错误（含状态码）在此暴露，
	// 便于调用方在提交客户端响应状态前透传。
	OpenChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error)
	// ReadChatCompletionStream 消费已打开的上游流，负责关闭 stream。
	ReadChatCompletionStream(ctx context.Context, stream io.ReadCloser, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error)
	ForwardCreateResponse(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error)
	OpenCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (io.ReadCloser, error)
	// ReadCreateResponseStream 消费已打开的上游流，负责关闭 stream。
	ReadCreateResponseStream(ctx context.Context, stream io.ReadCloser, onEvent func(event string, data []byte) error) error
}

// BlockedChecker 敏感词检查端口
type BlockedChecker interface {
	Check(text string) []uint
	MatchedWords(ids []uint) []string
	IncrementHits(ctx context.Context, ids []uint) error
}
