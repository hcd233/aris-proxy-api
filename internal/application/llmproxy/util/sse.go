package proxyutil

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// WriteAnthropicMessageStop 向客户端写入 Anthropic 协议的 message_stop 结束帧。
//
// 两条转发路径（forwardNative / forwardViaOpenAI）都通过此函数发送结束帧，
// 保证 event 类型和 data payload 一致（参见提交 184dcf9 的回归修复）。
// 返回 flush 错误而不 panic，调用方可按需处理（通常忽略即可）。
//
//	@param sink port.EventSink
//	@return error
//	@author centonhuang
//	@update 2026-07-25 10:00:00
func WriteAnthropicMessageStop(sink port.EventSink) error {
	return sink.WriteEvent(enum.AnthropicSSEEventTypeMessageStop, []byte(constant.AnthropicMessageStopData))
}

// WriteUpstreamSSEError 在 SSE 流中写入上游错误。
// 当上游在流式请求开始后（HTTP 200 已发送）返回错误时，本函数将上游错误体
// 以 SSE data 帧的形式写入客户端，避免客户端收到空的截断流。
//
//	@param ctx context.Context
//	@param sink port.EventSink
//	@param err error
//	@author centonhuang
//	@update 2026-07-25 10:00:00
func WriteUpstreamSSEError(ctx context.Context, sink port.EventSink, err error) {
	log := logger.WithCtx(ctx)
	var upstreamErr *model.UpstreamError
	if !errors.As(err, &upstreamErr) {
		log.Error("[WriteUpstreamSSEError] Non-upstream error in SSE stream", zap.Error(err))
		if writeErr := sink.WriteEvent("", []byte(constant.SSEOpenAIInternalErrorData)); writeErr != nil {
			log.Debug("[WriteUpstreamSSEError] Failed to write internal error frame", zap.Error(writeErr))
		}
		return
	}
	if upstreamErr.Body != "" {
		if writeErr := sink.WriteEvent("", []byte(upstreamErr.Body)); writeErr != nil {
			log.Debug("[WriteUpstreamSSEError] Failed to write upstream error body", zap.Error(writeErr))
		}
		return
	}
	data := fmt.Sprintf(constant.SSEOpenAIUpstreamErrorData, upstreamErr.StatusCode)
	if writeErr := sink.WriteEvent("", []byte(data)); writeErr != nil {
		log.Debug("[WriteUpstreamSSEError] Failed to write upstream status frame", zap.Error(writeErr))
	}
}

// SendOpenAIModelNotFoundError 构造 OpenAI 模型不存在错误。
//
// 返回 *port.ProxyError 由 adapter 映射为 HTTP JSON 响应；
// application 不构造 Huma response，也不设置 HTTP status/header。
//
//	@param modelName string
//	@return *port.ProxyError
//	@author centonhuang
//	@update 2026-07-25 10:00:00
func SendOpenAIModelNotFoundError(modelName string) *port.ProxyError {
	body := lo.Must1(sonic.Marshal(&dto.OpenAIError{
		Message: fmt.Sprintf(constant.OpenAIModelNotFoundMessageTemplate, modelName),
		Type:    constant.OpenAIInvalidRequestErrorType,
		Code:    constant.OpenAIModelNotFoundCode,
	}))
	return &port.ProxyError{
		StatusCode: http.StatusNotFound,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       body,
		Protocol:   enum.ProtocolKindOpenAI,
	}
}

// SendOpenAIContentBlockedError 构造 OpenAI 内容被拦截错误 (403)。
//
// 返回 *port.ProxyError 由 adapter 映射为 HTTP JSON 响应。
//
//	@return *port.ProxyError
//	@author centonhuang
//	@update 2026-07-25 10:00:00
func SendOpenAIContentBlockedError() *port.ProxyError {
	body := lo.Must1(sonic.Marshal(&dto.OpenAIError{
		Message: constant.TriggerContentBlockedErrorMessage,
		Type:    constant.TriggerContentBlockedErrorType,
		Code:    constant.TriggerContentBlockedErrorCode,
	}))
	return &port.ProxyError{
		StatusCode: http.StatusForbidden,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       body,
		Protocol:   enum.ProtocolKindOpenAI,
	}
}

// SendAnthropicContentBlockedError 构造 Anthropic 内容被拦截错误 (403)。
//
// 返回 *port.ProxyError 由 adapter 映射为 HTTP JSON 响应。
//
//	@return *port.ProxyError
//	@author centonhuang
//	@update 2026-07-25 10:00:00
func SendAnthropicContentBlockedError() *port.ProxyError {
	body := lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
		Type: constant.AnthropicInternalErrorBodyType,
		Error: &dto.AnthropicError{
			Type:    constant.TriggerContentBlockedErrorType,
			Message: constant.TriggerContentBlockedErrorMessage,
		},
	}))
	return &port.ProxyError{
		StatusCode: http.StatusForbidden,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       body,
		Protocol:   enum.ProtocolKindAnthropic,
	}
}
