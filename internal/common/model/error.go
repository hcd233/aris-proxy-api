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
