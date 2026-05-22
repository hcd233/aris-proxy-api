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

func (u *anthropicUseCase) forwardMessageNative(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, exposedModel string, stream bool) *huma.StreamResponse {
	body := proxyutil.MarshalAnthropicMessageBodyForModel(req.Body, upstream.Model)
	if stream {
		return u.forwardMessageNativeStream(ctx, req, m, upstream, exposedModel, body)
	}
	return u.forwardMessageNativeUnary(ctx, req, m, upstream, exposedModel, body)
}

func (u *anthropicUseCase) forwardMessageViaChat(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, exposedModel string) *huma.StreamResponse {
	conv := &converter.OpenAIProtocolConverter{}
	chatReq, convErr := conv.FromAnthropicRequest(req.Body)
	if convErr != nil {
		logger.WithCtx(ctx).Error("[AnthropicUseCase] Failed to convert anthropic request to chat", zap.Error(convErr))
		return proxyutil.SendAnthropicModelNotFoundError(exposedModel)
	}
	stream := req.Body.Stream != nil && *req.Body.Stream
	upstream := toTransportEndpoint(m, ep, false)
	body := proxyutil.MarshalOpenAIChatCompletionBodyForModel(chatReq, upstream.Model)
	if stream {
		return u.forwardMessageViaChatStream(ctx, req, m, upstream, exposedModel, body)
	}
	return u.forwardMessageViaChatUnary(ctx, req, m, upstream, exposedModel, body)
}
