package usecase

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

func (u *openAIUseCase) forwardChatNative(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(req.Body, upstream.Model)

	if stream {
		return u.forwardChatNativeStream(ctx, req, m, ep, upstream, body)
	}
	return u.forwardChatNativeUnary(ctx, req, m, ep, upstream, body)
}

func (u *openAIUseCase) forwardChatViaAnthropic(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) *huma.StreamResponse {
	conv := &converter.AnthropicProtocolConverter{}
	anthropicReq, convErr := conv.FromOpenAIRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[OpenAIUseCase] Failed to convert chat request to anthropic", zap.Error(convErr))
		return proxyutil.SendOpenAIModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, true)
	body := proxyutil.MarshalAnthropicMessageBodyForModel(anthropicReq, upstream.Model)
	if stream {
		return u.forwardChatViaAnthropicStream(ctx, req, m, upstream, exposedModel, body)
	}
	return u.forwardChatViaAnthropicUnary(ctx, req, m, upstream, exposedModel, body)
}
