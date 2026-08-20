package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/common/ratelimit"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// auditFailure 记录非流式失败调用的审计（上下游协议一致的简化入口）。
func auditFailure(ctx context.Context, m *aggregate.Model, submitter TaskSubmitter, tokenMetrics *metrics.TokenUsageCounter, _, endpoint string, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	auditFailureWithProviders(ctx, m, submitter, tokenMetrics, "", endpoint, apiProtocol, apiProtocol, totalMs, err)
}

// auditFailureWithProviders 记录非流式失败调用的审计（上下游协议可不同）。
func auditFailureWithProviders(ctx context.Context, m *aggregate.Model, submitter TaskSubmitter, tokenMetrics *metrics.TokenUsageCounter, _, endpoint string, upstreamProtocol, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	recordModelCall(ctx, submitter, tokenMetrics, callOutcome{
		model:               m,
		endpoint:            endpoint,
		upstreamProtocol:    upstreamProtocol,
		apiProtocol:         apiProtocol,
		firstTokenLatencyMs: totalMs,
		err:                 err,
	})
}

// ProxyErrorFromUpstream 透传"打开上游流"阶段的错误：将 *model.UpstreamError 转换为
// *port.ProxyError，保证上游状态码/响应头/错误体原样下发，而不是包进 200 的 SSE 流。
// 熔断打开（*model.CircuitOpenError）与信号量满载（*model.BulkheadFullError）映射为
// 503 + Retry-After 的降级响应。
// 仅适用于流式请求在流开始前即失败的场景；流开始后的中断仍走 WriteUpstreamSSEError。
//
//	@param err error 上游/容错错误
//	@param protocol enum.ProtocolKind 入口协议（决定错误体格式）
//	@param fallbackBody []byte 未知错误的兜底错误体
//	@return *port.ProxyError
//	@author centonhuang
//	@update 2026-08-20 10:00:00
func ProxyErrorFromUpstream(err error, protocol enum.ProtocolKind, fallbackBody []byte) *port.ProxyError {
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		headers := upstreamErr.Headers
		if headers == nil {
			headers = map[string]string{}
		}
		headers[constant.HTTPHeaderContentType] = constant.HTTPContentTypeJSON
		return &port.ProxyError{
			StatusCode: upstreamErr.StatusCode,
			Headers:    headers,
			Body:       []byte(upstreamErr.Body),
			Cause:      err,
			Protocol:   protocol,
		}
	}

	var circuitErr *model.CircuitOpenError
	if errors.As(err, &circuitErr) {
		retryAfter := int(math.Ceil(circuitErr.RetryAfter.Seconds()))
		return guardRejectedProxyError(circuitErr, protocol, guardOpenFallbackBody(protocol), retryAfter)
	}
	var bulkheadErr *model.BulkheadFullError
	if errors.As(err, &bulkheadErr) {
		return guardRejectedProxyError(bulkheadErr, protocol, guardFullFallbackBody(protocol), constant.BulkheadRetryAfterSeconds)
	}

	logger.Logger().Error("[ProxyService] Proxy error", zap.Error(err))
	return &port.ProxyError{
		StatusCode: http.StatusBadGateway,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       fallbackBody,
		Cause:      err,
		Protocol:   protocol,
	}
}

// guardRejectedProxyError 把熔断/满载错误映射为 503 + Retry-After 的降级响应。
func guardRejectedProxyError(cause error, protocol enum.ProtocolKind, body []byte, retryAfter int) *port.ProxyError {
	if retryAfter < 1 {
		retryAfter = 1
	}
	return &port.ProxyError{
		StatusCode: http.StatusServiceUnavailable,
		Headers: map[string]string{
			constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON,
			constant.HTTPHeaderRetryAfter:  strconv.Itoa(retryAfter),
		},
		Body:     body,
		Cause:    cause,
		Protocol: protocol,
	}
}

// guardOpenFallbackBody 熔断打开的降级错误体（按协议格式）。
func guardOpenFallbackBody(protocol enum.ProtocolKind) []byte {
	if protocol == enum.ProtocolKindAnthropic {
		return []byte(`{"type":"error","error":{"type":"overloaded_error","message":"上游服务暂时不可用，请稍后重试或更换模型"}}`)
	}
	return []byte(`{"error":{"message":"上游服务暂时不可用，请稍后重试或更换模型","type":"circuit_open","code":503}}`)
}

// guardFullFallbackBody 信号量满载的降级错误体（按协议格式）。
func guardFullFallbackBody(protocol enum.ProtocolKind) []byte {
	if protocol == enum.ProtocolKindAnthropic {
		return []byte(`{"type":"error","error":{"type":"overloaded_error","message":"上游负载过高，请稍后重试"}}`)
	}
	return []byte(`{"error":{"message":"上游负载过高，请稍后重试","type":"bulkhead_full","code":503}}`)
}

// extractUpstreamStatusAndError 从 err 提取上游状态码与错误信息，用于审计任务。
// 与 transport 层的同名工具等价，但在 application 层内定义以避免反向依赖 HTTP 边界。
func extractUpstreamStatusAndError(err error) (statusCode int, errorMessage string) {
	if err == nil {
		return http.StatusOK, ""
	}
	var ue *model.UpstreamError
	if errors.As(err, &ue) {
		msg := ue.Error()
		if ue.Body != "" {
			msg += fmt.Sprintf(constant.ColonMessageTemplate, ue.Body)
		}
		return ue.StatusCode, msg
	}
	var connErr *model.UpstreamConnectionError
	if errors.As(err, &connErr) {
		return enum.CallStatusConnectionError, connErr.Error()
	}
	return enum.CallStatusUnknownError, err.Error()
}

// reportTokenUsage 从 context 取出 TokenUsageReporter 并上报实际 token 用量。
func reportTokenUsage(ctx context.Context, tokens int64) {
	if tokens <= 0 {
		return
	}
	reporter, ok := ctx.Value(constant.CtxKeyTokenUsageReporter).(ratelimit.TokenUsageReporter)
	if !ok || reporter == nil {
		return
	}
	reporter.Report(ctx, tokens)
}
