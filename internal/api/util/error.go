package apiutil

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
)

// bizHTTPError 统一业务错误响应模型。
//
// 实现 huma.StatusError：handler 返回该 error 时，huma 直接序列化本结构，
// 输出顶层 {"error": {code, message}} + 由业务错误码推导的 HTTP 状态码。
// 与中间件层 apiutil.WriteErrorResponse / WriteErrorHTTPResponse 的输出结构一致。
type bizHTTPError struct {
	ErrorBody *model.Error `json:"error" doc:"业务错误体"`
	status    int
}

// Error 实现 error 接口。
func (e *bizHTTPError) Error() string {
	if e.ErrorBody == nil {
		return ""
	}
	return e.ErrorBody.Error()
}

// GetStatus 实现 huma.StatusError 接口，返回 HTTP 状态码。
func (e *bizHTTPError) GetStatus() int {
	return e.status
}

// Unwrap 支持 errors.As / errors.Is 解包到业务错误。
func (e *bizHTTPError) Unwrap() error {
	if e.ErrorBody == nil {
		return nil
	}
	return e.ErrorBody
}

// NewHumaBizError 将内部错误转换为统一的业务错误响应。
//
// 与 ierr.ToBizErrorLocalized 语义一致：从 err 提取业务错误并本地化，
// 若 err 非 InternalError 则使用 fallback。HTTP 状态码由业务错误码推导。
//
//	@param ctx context.Context
//	@param err error 内部错误
//	@param fallback *model.Error 非 InternalError 时的兜底业务错误
//	@return error
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func NewHumaBizError(ctx context.Context, err error, fallback *model.Error) error {
	bizErr := ierr.ToBizError(err, fallback)
	return newBizHTTPError(ctx, bizErr)
}

// NewHumaBizErrorFromModel 直接基于业务错误模型构造统一的业务错误响应。
//
// 用于 handler 内直接抛出的校验错误等场景（err 本身不是 InternalError 时）。
//
//	@param ctx context.Context
//	@param bizErr *model.Error 业务错误模型
//	@return error
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func NewHumaBizErrorFromModel(ctx context.Context, bizErr *model.Error) error {
	if bizErr == nil {
		bizErr = ierr.ErrInternal.BizError()
	}
	return newBizHTTPError(ctx, bizErr)
}

func newBizHTTPError(ctx context.Context, bizErr *model.Error) error {
	localized := bizErr.Localize(i18n.FromCtx(ctx))
	return &bizHTTPError{
		ErrorBody: localized,
		status:    localized.StatusCode(),
	}
}

// FrameworkError 将 huma 框架错误（校验失败 422、路由 404 等）转换为统一的
// {"error": {code, message}} 结构。HTTP 状态码保留 huma 传入的原始状态码，
// 业务码由状态码反向推导（未识别状态码兜底为内部错误）。
// errs 中的字段校验细节（huma.ErrorDetail 等）会拼接到 message，避免信息丢失。
//
//	@param status int huma 传入的 HTTP 状态码
//	@param message string huma 传入的错误消息
//	@param errs ...error 字段级错误细节
//	@return huma.StatusError
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func FrameworkError(status int, message string, errs ...error) huma.StatusError {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	detail := message
	if len(errs) > 0 {
		details := make([]string, 0, len(errs))
		for _, e := range errs {
			if e != nil {
				details = append(details, e.Error())
			}
		}
		if len(details) > 0 {
			detail = message + constant.BizErrorDetailSep + strings.Join(details, constant.BizErrorDetailJoinSep)
		}
	}
	return &bizHTTPError{
		ErrorBody: &model.Error{
			Code:    statusToBizCode(status),
			Message: detail,
		},
		status: status,
	}
}

// statusToBizCode 将 HTTP 状态码反向推导为业务错误码（框架错误场景）。
func statusToBizCode(status int) int {
	switch status {
	case http.StatusUnauthorized:
		return constant.BizErrorCodeUnauthorized
	case http.StatusForbidden:
		return constant.BizErrorCodeNoPermission
	case http.StatusNotFound:
		return constant.BizErrorCodeDataNotExists
	case http.StatusConflict:
		return constant.BizErrorCodeDataExists
	case http.StatusTooManyRequests:
		return constant.BizErrorCodeTooManyRequests
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return constant.BizErrorCodeBadRequest
	case http.StatusLocked:
		return constant.BizErrorCodeResourceLocked
	default:
		return constant.BizErrorCodeInternal
	}
}
