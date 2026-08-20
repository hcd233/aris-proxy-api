package model

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
)

// Error 错误
//
//	@author centonhuang
//	@update 2025-11-10 19:10:53
type Error struct {
	Code       int    `json:"code" doc:"Code"`
	Message    string `json:"message" doc:"Message"`
	MessageKey string `json:"-"`
}

// NewError 创建错误
//
//	@param code int
//	@param message string
//	@return *Error
//	@author centonhuang
//	@update 2025-11-10 19:14:00
func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithKey 创建带翻译键的错误
func NewErrorWithKey(code int, message, key string) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		MessageKey: key,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf(constant.ErrorModelTemplate, e.Code, e.Message)
}

// Localize 根据 locale 翻译错误消息
func (e *Error) Localize(locale enum.Locale) *Error {
	if e.MessageKey == "" {
		return e
	}
	return &Error{
		Code:       e.Code,
		Message:    i18n.Translate(locale, e.MessageKey, e.Message),
		MessageKey: e.MessageKey,
	}
}

// StatusCode 将业务错误码映射为 HTTP 状态码。
//
// 业务错误统一走顶层 {"error": {code, message}} 响应后，HTTP 状态码由业务码推导。
// 未显式映射的错误码兜底为 500。
func (e *Error) StatusCode() int {
	switch e.Code {
	case constant.BizErrorCodeUnauthorized: // ErrUnauthorized / ErrJWTDecode
		return http.StatusUnauthorized
	case constant.BizErrorCodeNoPermission: // ErrNoPermission
		return http.StatusForbidden
	case constant.BizErrorCodeDataNotExists: // ErrDataNotExists
		return http.StatusNotFound
	case constant.BizErrorCodeDataExists: // ErrDataExists
		return http.StatusConflict
	case constant.BizErrorCodeTooManyRequests, constant.BizErrorCodeInsufficientQuota, constant.BizErrorCodeQuotaExceeded: // ErrTooManyRequests / ErrInsufficientQuota / ErrQuotaExceeded
		return http.StatusTooManyRequests
	case constant.BizErrorCodeBadRequest: // ErrBadRequest / ErrValidation
		return http.StatusBadRequest
	case constant.BizErrorCodeResourceLocked: // ErrResourceLocked
		return http.StatusLocked
	case constant.BizErrorCodeContentBlocked: // ErrContentBlocked
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// UpstreamError 上游返回非 200 状态码的错误
//
//	@author centonhuang
//	@update 2026-04-29 10:00:00
type UpstreamError struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf(constant.UpstreamErrorTemplate, e.StatusCode)
}

// UpstreamConnectionError 上游连接错误（网络层错误，无法获取 HTTP 状态码）
//
//	@author centonhuang
//	@update 2026-04-15 19:00:00
type UpstreamConnectionError struct {
	Cause error
}

func (e *UpstreamConnectionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf(constant.UpstreamConnectionErrorTemplate, e.Cause)
	}
	return constant.UpstreamConnectionErrorMsg
}

// Unwrap 透传 Cause，使 errors.Is(err, context.Canceled) 能穿透判断
// （优雅退出 soft deadline 广播取消上游连接时，读循环据此识别断流原因）。
func (e *UpstreamConnectionError) Unwrap() error {
	return e.Cause
}

// CircuitOpenError 熔断器打开导致的快速失败错误。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type CircuitOpenError struct {
	Key        string
	RetryAfter time.Duration
}

func (e *CircuitOpenError) Error() string {
	return fmt.Sprintf(constant.CircuitOpenErrorTemplate, e.Key, e.RetryAfter)
}
