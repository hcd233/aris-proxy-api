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
//	@update 2026-03-29 10:00:00
func CopyContextValues(src context.Context) (dst context.Context) {
	dst = context.Background()
	dst = context.WithValue(dst, constant.CtxKeyTraceID, src.Value(constant.CtxKeyTraceID))
	dst = context.WithValue(dst, constant.CtxKeyUserID, src.Value(constant.CtxKeyUserID))
	return dst
}

// CtxValueString 安全获取上下文中的字符串值
//
//	@param ctx context.Context
//	@param key string
//	@return string
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func CtxValueString(ctx context.Context, key string) string {
	if v := ctx.Value(key); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
