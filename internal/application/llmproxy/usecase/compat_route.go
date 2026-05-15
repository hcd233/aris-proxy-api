package usecase

import (
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

func SelectCompatRoute(requestAPI enum.ProxyAPI, ep *aggregate.Endpoint) enum.CompatRoute {
	if ep == nil {
		return enum.CompatRouteUnsupported
	}
	switch requestAPI {
	case enum.ProxyAPIOpenAIChat:
		if ep.SupportOpenAIChatCompletion() {
			return enum.CompatRouteNative
		}
		if ep.SupportAnthropicMessage() {
			return enum.CompatRouteViaAnthropicMessage
		}
	case enum.ProxyAPIOpenAIResponse:
		if ep.SupportOpenAIResponse() {
			return enum.CompatRouteNative
		}
		if ep.SupportOpenAIChatCompletion() {
			return enum.CompatRouteViaOpenAIChat
		}
		if ep.SupportAnthropicMessage() {
			return enum.CompatRouteViaAnthropicMessage
		}
	case enum.ProxyAPIAnthropicMessage:
		if ep.SupportAnthropicMessage() {
			return enum.CompatRouteNative
		}
		if ep.SupportOpenAIChatCompletion() {
			return enum.CompatRouteViaOpenAIChat
		}
	}
	return enum.CompatRouteUnsupported
}

func supportsCompatRoute(requestAPI enum.ProxyAPI) func(*aggregate.Endpoint) bool {
	return func(ep *aggregate.Endpoint) bool {
		return SelectCompatRoute(requestAPI, ep) != enum.CompatRouteUnsupported
	}
}
