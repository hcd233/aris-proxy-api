// Package ierr 内部错误定义，用于统一项目内部的 Go error 管理
//
//	@author centonhuang
//	@update 2026-03-29 10:00:00
package ierr

import (
	"errors"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// InternalError 内部错误，携带哨兵错误 + 可选上下文信息 + 可选原始错误
//
//	@author centonhuang
//	@update 2026-03-29 10:00:00
type InternalError struct {
	sentinel *sentinel
	message  string
	cause    error
}

// sentinel 哨兵错误，定义错误类别和对应的业务错误映射
type sentinel struct {
	name     string
	bizError *model.Error
}

// Error 实现 error 接口
func (e *InternalError) Error() string {
	base := e.sentinel.name
	if e.message != "" {
		base += ": " + e.message
	}
	if e.cause != nil {
		base += ": " + e.cause.Error()
	}
	return base
}

// Unwrap 支持 errors.Is / errors.As 解包
func (e *InternalError) Unwrap() error {
	return e.cause
}

// Is 支持与哨兵错误比较
func (e *InternalError) Is(target error) bool {
	var ie *InternalError
	if errors.As(target, &ie) {
		return e.sentinel == ie.sentinel
	}
	return false
}

// BizError 获取对应的业务错误，用于 Service 层自动映射
func (e *InternalError) BizError() *model.Error {
	return e.sentinel.bizError
}

// newSentinel 创建哨兵错误（包内使用）
func newSentinel(name string, bizError *model.Error) *sentinel {
	return &sentinel{
		name:     name,
		bizError: bizError,
	}
}

// newFromSentinel 从哨兵创建 InternalError 实例（无上下文、无 cause）
func newFromSentinel(s *sentinel) *InternalError {
	return &InternalError{sentinel: s}
}

// Wrap 包装一个原始错误，附加上下文描述
//
//	@param sentinel *InternalError 哨兵错误实例
//	@param cause error 原始错误
//	@param message string 上下文描述
//	@return error
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func Wrap(sentinel *InternalError, cause error, message string) error {
	return &InternalError{
		sentinel: sentinel.sentinel,
		message:  message,
		cause:    cause,
	}
}

// Wrapf 包装一个原始错误，附加格式化的上下文描述
//
//	@param sentinel *InternalError 哨兵错误实例
//	@param cause error 原始错误
//	@param format string 格式化字符串
//	@param args ...any 格式化参数
//	@return error
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func Wrapf(sentinel *InternalError, cause error, format string, args ...any) error {
	return &InternalError{
		sentinel: sentinel.sentinel,
		message:  fmt.Sprintf(format, args...),
		cause:    cause,
	}
}

// New 创建一个不包装原始错误的内部错误，仅携带上下文描述
//
//	@param sentinel *InternalError 哨兵错误实例
//	@param message string 上下文描述
//	@return error
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func New(sentinel *InternalError, message string) error {
	return &InternalError{
		sentinel: sentinel.sentinel,
		message:  message,
	}
}

// Newf 创建一个不包装原始错误的内部错误，携带格式化的上下文描述
//
//	@param sentinel *InternalError 哨兵错误实例
//	@param format string 格式化字符串
//	@param args ...any 格式化参数
//	@return error
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func Newf(sentinel *InternalError, format string, args ...any) error {
	return &InternalError{
		sentinel: sentinel.sentinel,
		message:  fmt.Sprintf(format, args...),
	}
}

// ToBizError 从 error 中提取业务错误，若非 InternalError 则返回默认的 fallback
//
//	@param err error
//	@param fallback *model.Error 降级业务错误
//	@return *model.Error
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func ToBizError(err error, fallback *model.Error) *model.Error {
	var ie *InternalError
	if errors.As(err, &ie) {
		return ie.BizError()
	}
	return fallback
}
