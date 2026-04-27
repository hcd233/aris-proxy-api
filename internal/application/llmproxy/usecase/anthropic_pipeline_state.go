package usecase

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	llmpipeline "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/pipeline"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type anthropicMessagePipelineState struct {
	Req          *dto.AnthropicCreateMessageRequest
	Log          *zap.Logger
	Endpoint     *aggregate.Endpoint
	Upstream     transport.UpstreamEndpoint
	Route        pipelineRoute
	Stream       bool
	ExposedModel string
	HTTPResponse *huma.StreamResponse
	Body         []byte
}

func (u *anthropicUseCase) buildAnthropicMessagePipeline() *llmpipeline.Pipeline[anthropicMessagePipelineState] {
	return llmpipeline.NewPipeline(
		llmpipeline.NewStep("resolve_endpoint", u.resolveAnthropicMessageEndpoint),
		llmpipeline.NewStep("select_route", u.selectAnthropicMessageRoute),
		llmpipeline.NewStep("prepare_anthropic_message_route", u.prepareAnthropicMessageRoute),
		llmpipeline.NewStep("forward_anthropic_message_route", u.forwardAnthropicMessageRoute),
	)
}

func (u *anthropicUseCase) resolveAnthropicMessageEndpoint(ctx context.Context, state *anthropicMessagePipelineState) error {
	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(state.Req.Body.Model), enum.ProviderAnthropic, enum.ProviderOpenAI)
	if err != nil {
		state.Log.Error("[AnthropicUseCase] Model not found", zap.String("model", state.Req.Body.Model), zap.Error(err))
		state.HTTPResponse = util.SendAnthropicModelNotFoundError(state.Req.Body.Model)
		return nil
	}
	state.Endpoint = ep
	state.Upstream = toTransportEndpoint(ep)
	return nil
}

func (u *anthropicUseCase) selectAnthropicMessageRoute(_ context.Context, state *anthropicMessagePipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	state.Route = selectPipelineRoute(enum.ProviderAnthropic, state.Endpoint.Provider(), state.Stream)
	return nil
}

func (u *anthropicUseCase) prepareAnthropicMessageRoute(_ context.Context, state *anthropicMessagePipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	if state.Route.TargetProvider == enum.ProviderOpenAI {
		rsp, body := u.prepareMessageViaOpenAI(state.Log, state.Req, state.Upstream)
		state.HTTPResponse = rsp
		state.Body = body
		return nil
	}
	state.Body = prepareMessageNativeBody(state.Req, state.Upstream)
	return nil
}

func (u *anthropicUseCase) forwardAnthropicMessageRoute(ctx context.Context, state *anthropicMessagePipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	if state.Route.TargetProvider == enum.ProviderOpenAI {
		conv := converter.OpenAIProtocolConverter{}
		if state.Stream {
			state.HTTPResponse = u.forwardMessageViaOpenAIStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body, &conv)
			return nil
		}
		state.HTTPResponse = u.forwardMessageViaOpenAIUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body, &conv)
		return nil
	}
	if state.Stream {
		state.HTTPResponse = u.forwardMessageNativeStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body)
		return nil
	}
	state.HTTPResponse = u.forwardMessageNativeUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.ExposedModel, state.Body)
	return nil
}
