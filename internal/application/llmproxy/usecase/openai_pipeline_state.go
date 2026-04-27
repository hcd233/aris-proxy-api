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

type openAIChatPipelineState struct {
	Req          *dto.OpenAIChatCompletionRequest
	Log          *zap.Logger
	Endpoint     *aggregate.Endpoint
	Upstream     transport.UpstreamEndpoint
	Route        pipelineRoute
	Stream       bool
	HTTPResponse *huma.StreamResponse
	Body         []byte
}

type openAIResponsePipelineState struct {
	Req          *dto.OpenAICreateResponseRequest
	Log          *zap.Logger
	Endpoint     *aggregate.Endpoint
	Upstream     transport.UpstreamEndpoint
	Route        pipelineRoute
	Stream       bool
	HTTPResponse *huma.StreamResponse
	Body         []byte
}

func (u *openAIUseCase) buildOpenAIChatPipeline() *llmpipeline.Pipeline[openAIChatPipelineState] {
	return llmpipeline.NewPipeline(
		llmpipeline.NewStep("resolve_endpoint", u.resolveOpenAIChatEndpoint),
		llmpipeline.NewStep("select_route", u.selectOpenAIChatRoute),
		llmpipeline.NewStep("prepare_openai_chat_route", u.prepareOpenAIChatRoute),
		llmpipeline.NewStep("forward_openai_chat_route", u.forwardOpenAIChatRoute),
	)
}

func (u *openAIUseCase) resolveOpenAIChatEndpoint(ctx context.Context, state *openAIChatPipelineState) error {
	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(state.Req.Body.Model), enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		state.Log.Error("[OpenAIUseCase] Model not found", zap.String("model", state.Req.Body.Model), zap.Error(err))
		state.HTTPResponse = util.SendOpenAIModelNotFoundError(state.Req.Body.Model)
		return nil
	}
	state.Endpoint = ep
	state.Upstream = toTransportEndpoint(ep)
	return nil
}

func (u *openAIUseCase) selectOpenAIChatRoute(_ context.Context, state *openAIChatPipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	state.Route = selectPipelineRoute(enum.ProviderOpenAI, state.Endpoint.Provider(), state.Stream)
	return nil
}

func (u *openAIUseCase) prepareOpenAIChatRoute(_ context.Context, state *openAIChatPipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	if state.Route.TargetProvider == enum.ProviderAnthropic {
		rsp, body := u.prepareChatViaAnthropic(state.Log, state.Req, state.Upstream)
		state.HTTPResponse = rsp
		state.Body = body
		return nil
	}
	state.Body = prepareChatNativeBody(state.Req, state.Upstream)
	return nil
}

func (u *openAIUseCase) forwardOpenAIChatRoute(ctx context.Context, state *openAIChatPipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	if state.Route.TargetProvider == enum.ProviderAnthropic {
		conv := converter.AnthropicProtocolConverter{}
		if state.Stream {
			state.HTTPResponse = u.forwardChatViaAnthropicStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
			return nil
		}
		state.HTTPResponse = u.forwardChatViaAnthropicUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
		return nil
	}
	if state.Stream {
		state.HTTPResponse = u.forwardChatNativeStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
		return nil
	}
	state.HTTPResponse = u.forwardChatNativeUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
	return nil
}

func (u *openAIUseCase) buildOpenAIResponsePipeline() *llmpipeline.Pipeline[openAIResponsePipelineState] {
	return llmpipeline.NewPipeline(
		llmpipeline.NewStep("resolve_endpoint", u.resolveOpenAIResponseEndpoint),
		llmpipeline.NewStep("select_route", u.selectOpenAIResponseRoute),
		llmpipeline.NewStep("prepare_openai_response_route", u.prepareOpenAIResponseRoute),
		llmpipeline.NewStep("forward_openai_response_route", u.forwardOpenAIResponseRoute),
	)
}

func (u *openAIUseCase) resolveOpenAIResponseEndpoint(ctx context.Context, state *openAIResponsePipelineState) error {
	ep, err := u.resolver.Resolve(ctx, vo.EndpointAlias(state.Req.Body.Model), enum.ProviderOpenAI, enum.ProviderAnthropic)
	if err != nil {
		state.Log.Error("[OpenAIUseCase] Response API model not found", zap.String("model", state.Req.Body.Model), zap.Error(err))
		state.HTTPResponse = util.SendOpenAIModelNotFoundError(state.Req.Body.Model)
		return nil
	}
	state.Endpoint = ep
	state.Upstream = toTransportEndpoint(ep)
	return nil
}

func (u *openAIUseCase) selectOpenAIResponseRoute(_ context.Context, state *openAIResponsePipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	state.Route = selectPipelineRoute(enum.ProviderOpenAI, state.Endpoint.Provider(), state.Stream)
	return nil
}

func (u *openAIUseCase) prepareOpenAIResponseRoute(_ context.Context, state *openAIResponsePipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	if state.Route.TargetProvider == enum.ProviderAnthropic {
		rsp, body := u.prepareResponseViaAnthropic(state.Log, state.Req, state.Upstream)
		state.HTTPResponse = rsp
		state.Body = body
		return nil
	}
	state.Body = prepareResponseNativeBody(state.Req, state.Upstream)
	return nil
}

func (u *openAIUseCase) forwardOpenAIResponseRoute(ctx context.Context, state *openAIResponsePipelineState) error {
	if state.HTTPResponse != nil {
		return nil
	}
	if state.Route.TargetProvider == enum.ProviderAnthropic {
		conv := converter.AnthropicProtocolConverter{}
		if state.Stream {
			state.HTTPResponse = u.forwardResponseViaAnthropicStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
			return nil
		}
		state.HTTPResponse = u.forwardResponseViaAnthropicUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body, &conv)
		return nil
	}
	if state.Stream {
		state.HTTPResponse = u.forwardResponseNativeStream(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
		return nil
	}
	state.HTTPResponse = u.forwardResponseNativeUnary(ctx, state.Log, state.Req, state.Endpoint, state.Upstream, state.Body)
	return nil
}
