package usecase

import (
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
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
	case enum.ProxyAPIOpenAIDecision:
		// Decision 仅支持原生转发：predicate/choice/score 在 Chat/Anthropic 协议里
		// 没有等价语义，跨协议转换等于自造一套 prompt 协议（见设计文档 §2.2）。
		if ep.SupportOpenAIDecision() {
			return enum.CompatRouteNative
		}
	}
	return enum.CompatRouteUnsupported
}
