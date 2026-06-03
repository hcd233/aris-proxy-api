// Package port defines application-layer ports for llmproxy use cases.
package port

import (
	"context"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// OpenAIUseCase OpenAI 协议用例端口
type OpenAIUseCase interface {
	ListModels(ctx context.Context) (*dto.OpenAIListModelsRsp, error)
	CreateChatCompletion(ctx context.Context, req *dto.OpenAIChatCompletionRequest) (*huma.StreamResponse, error)
	CreateResponse(ctx context.Context, req *dto.OpenAICreateResponseRequest) (*huma.StreamResponse, error)
}

// AnthropicUseCase Anthropic 协议用例端口
type AnthropicUseCase interface {
	ListModels(ctx context.Context) (*dto.AnthropicListModelsRsp, error)
	CreateMessage(ctx context.Context, req *dto.AnthropicCreateMessageRequest) (*huma.StreamResponse, error)
	CountTokens(ctx context.Context, req *dto.AnthropicCountTokensRequest) (*dto.AnthropicTokensCount, error)
}
