package util

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// CopyContextValues 复制上下文值
//
//	@param src context.Context
//	@return dst context.Context
//	@author centonhuang
//	@update 2026-01-31 03:33:01
func CopyContextValues(src context.Context) (dst context.Context) {
	dst = context.Background()
	dst = context.WithValue(dst, constant.CtxKeyTraceID, src.Value(constant.CtxKeyTraceID))
	dst = context.WithValue(dst, constant.CtxKeyUserID, src.Value(constant.CtxKeyUserID))
	return dst
}
