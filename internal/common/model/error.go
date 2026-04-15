package model

import "fmt"

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
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

// UpstreamError 上游返回非 200 状态码的错误
//
//	@author centonhuang
//	@update 2026-04-05 10:00:00
type UpstreamError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream returned status %d", e.StatusCode)
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
		return fmt.Sprintf("upstream connection error: %v", e.Cause)
	}
	return "upstream connection error"
}
