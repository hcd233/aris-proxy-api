package model

import (
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// Error 错误
//
//	@author centonhuang
//	@update 2025-11-10 19:10:53
type Error struct {
	Code    int    `json:"code" doc:"Code"`
	Message string `json:"message" doc:"Message"`
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

func (e *Error) Error() string {
	return fmt.Sprintf(constant.ErrorModelTemplate, e.Code, e.Message)
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
