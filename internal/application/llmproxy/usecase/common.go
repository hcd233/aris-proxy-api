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
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/metrics"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// auditFailure 记录非流式失败调用的审计（上下游协议一致的简化入口）。
func auditFailure(ctx context.Context, m *aggregate.Model, submitter TaskSubmitter, tokenMetrics *metrics.TokenUsageCounter, _, endpoint string, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	auditFailureWithProviders(ctx, m, submitter, tokenMetrics, "", endpoint, apiProtocol, apiProtocol, totalMs, err)
}

// auditFailureWithProviders 记录非流式失败调用的审计（上下游协议可不同）。
//
// 处于 runWithFallback 的尝试期内时只暂存不提交：是否终局由 runWithFallback 判定。
func auditFailureWithProviders(ctx context.Context, m *aggregate.Model, submitter TaskSubmitter, tokenMetrics *metrics.TokenUsageCounter, _, endpoint string, upstreamProtocol, apiProtocol enum.ProtocolType, totalMs int64, err error) {
	submit := func() {
		recordModelCall(ctx, submitter, tokenMetrics, callOutcome{
			model:               m,
			endpoint:            endpoint,
			upstreamProtocol:    upstreamProtocol,
			apiProtocol:         apiProtocol,
			firstTokenLatencyMs: totalMs,
			err:                 err,
		})
	}
	if d, ok := ctx.Value(constant.CtxKeyFailureAuditDeferral).(*failureAuditDeferral); ok && d.active {
		d.pending = submit
		return
	}
	submit()
}

// failureAuditDeferral 单次 fallback 尝试的失败审计暂存槽。
//
// active 仅在 attempt 执行期间为 true：流建立后（runWithFallback 已返回）
// 发生的失败审计直接提交，不会被吞掉。
type failureAuditDeferral struct {
	active  bool
	pending func()
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

// guardOpenFallbackBody 熔断打开的降级错误体（按协议格式；type 用官方枚举，不暴露内部实现语义）。
func guardOpenFallbackBody(protocol enum.ProtocolKind) []byte {
	if protocol == enum.ProtocolKindAnthropic {
		return []byte(`{"type":"error","error":{"type":"overloaded_error","message":"上游服务暂时不可用，请稍后重试或更换模型"}}`)
	}
	return []byte(`{"error":{"message":"上游服务暂时不可用，请稍后重试或更换模型","type":"server_error","code":"circuit_open"}}`)
}

// guardFullFallbackBody 信号量满载的降级错误体（按协议格式；type 用官方枚举，不暴露内部实现语义）。
func guardFullFallbackBody(protocol enum.ProtocolKind) []byte {
	if protocol == enum.ProtocolKindAnthropic {
		return []byte(`{"type":"error","error":{"type":"overloaded_error","message":"上游负载过高，请稍后重试"}}`)
	}
	return []byte(`{"error":{"message":"上游负载过高，请稍后重试","type":"server_error","code":"bulkhead_full"}}`)
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

// CanSwitchEndpoint 判断转发失败是否可切换到下一个候选端点。
//
// 可切换：连接错误 / 5xx / 429（common/model 的 Retryable()，与 transport 层
// IsRetryableError 共用一份判定），以及 Guard 熔断打开、信号量满载
// （端点熔断正是换端点的时机）。errors.As 穿透 ProxyError.Cause 后判定。
func CanSwitchEndpoint(err error) bool {
	if err == nil {
		return false
	}
	var connErr *model.UpstreamConnectionError
	if errors.As(err, &connErr) {
		return connErr.Retryable()
	}
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.Retryable()
	}
	var circuitErr *model.CircuitOpenError
	if errors.As(err, &circuitErr) {
		return true
	}
	var bulkheadErr *model.BulkheadFullError
	if errors.As(err, &bulkheadErr) {
		return true
	}
	var proxyErr *port.ProxyError
	if errors.As(err, &proxyErr) && proxyErr.Cause != nil {
		return CanSwitchEndpoint(proxyErr.Cause)
	}
	return false
}

// runWithFallback 按候选顺序转发，失败且 CanSwitchEndpoint 时切换到下一个端点。
//
// dispatch 内部仍走 transport 层同端点重试（SendUpstreamWithRetry）与 Guard 熔断租约；
// 仅在流建立前的失败可切换——流一旦交付给 handler 即为终局。
// 候选耗尽、不可切换或请求已取消（客户端断开/服务 drain）时返回最后一次错误。
//
// 审计口径（spec §5）：只有终局结果进审计；被切换掉的中间失败仅记日志，
// 由 failureAuditDeferral 暂存后丢弃。
func runWithFallback(ctx context.Context, module, modelName string, cands []service.Candidate, attempt func(context.Context, service.Candidate) (port.Result, error)) (port.Result, error) {
	log := logger.WithCtx(ctx)
	var lastErr error
	for i := range cands {
		deferral := &failureAuditDeferral{active: true}
		result, fwdErr := attempt(context.WithValue(ctx, constant.CtxKeyFailureAuditDeferral, deferral), cands[i])
		deferral.active = false
		if fwdErr == nil {
			return result, nil
		}
		lastErr = fwdErr
		if i == len(cands)-1 || ctx.Err() != nil || !CanSwitchEndpoint(fwdErr) {
			if deferral.pending != nil {
				deferral.pending()
			}
			return nil, fwdErr
		}
		log.Warn("["+module+"] Upstream failed, switching endpoint",
			zap.String("model", modelName),
			zap.String("from", cands[i].Endpoint.Name()),
			zap.String("to", cands[i+1].Endpoint.Name()),
			zap.Error(fwdErr),
		)
	}
	return nil, lastErr
}
