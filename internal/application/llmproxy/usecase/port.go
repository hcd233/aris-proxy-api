// Package usecase LLMProxy 域用例层 — 端口定义
package usecase

import (
	"context"

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
	ForwardCreateMessageStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onEvent func(dto.AnthropicSSEEvent) error) (*dto.AnthropicMessage, error)
	ForwardCountTokens(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.AnthropicTokensCount, error)
}

// OpenAIProxyPort OpenAI 协议上游代理端口
//
// 由 infrastructure/transport.openAIProxy 实现。
// 定义在 application 层，确保 application 层不直接依赖 infrastructure 层的接口定义。
type OpenAIProxyPort interface {
	ForwardChatCompletion(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) (*dto.OpenAIChatCompletion, error)
	ForwardChatCompletionStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onChunk func(*dto.OpenAIChatCompletionChunk) error) (*dto.OpenAIChatCompletion, error)
	ForwardCreateResponse(ctx context.Context, ep vo.UpstreamEndpoint, body []byte) ([]byte, error)
	ForwardCreateResponseStream(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, onEvent func(event string, data []byte) error) error
}
