package constant

import (
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

var (

	// ErrInternalError 内部错误
	//
	//	update 2025-01-04 17:35:44
	ErrInternalError = model.NewError(10000, "InternalError")

	// ErrUnauthorized 未授权错误
	//
	//	update 2025-01-04 17:36:00
	ErrUnauthorized = model.NewError(10001, "Unauthorized")

	// ErrNoPermission 没有权限错误
	//
	//	update 2025-01-04 17:36:00
	ErrNoPermission = model.NewError(10002, "NoPermission")

	// ErrDataNotExists 数据不存在错误
	//
	//	update 2025-01-04 17:36:00
	ErrDataNotExists = model.NewError(10003, "DataNotExists")

	// ErrDataExists 数据已存在错误
	//
	//	update 2025-01-04 17:36:00
	ErrDataExists = model.NewError(10004, "DataExists")

	// ErrTooManyRequests 请求过于频繁错误
	//
	//	update 2025-01-04 17:36:00
	ErrTooManyRequests = model.NewError(10005, "TooManyRequests")

	// ErrBadRequest 请求错误
	//
	//	update 2025-01-04 17:36:00
	ErrBadRequest = model.NewError(10006, "BadRequest")

	// ErrInsufficientQuota 配额不足错误
	//
	//	update 2025-01-05 18:41:32
	ErrInsufficientQuota = model.NewError(10007, "InsufficientQuota")

	// ErrNoImplement 未实现错误
	//
	//	update 2025-01-05 18:41:32
	ErrNoImplement = model.NewError(10008, "NoImplement")

	// ErrResourceLocked 资源锁定错误
	//
	//	update 2025-11-13 17:48:00
	ErrResourceLocked = model.NewError(10009, "ResourceLocked")
)
