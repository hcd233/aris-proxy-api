package util

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// CopyContextValues 复制上下文值
//
//	param src context.Context
//	return dst context.Context
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func CopyContextValues(src context.Context) (dst context.Context) {
	dst = context.Background()
	dst = context.WithValue(dst, constant.CtxKeyTraceID, src.Value(constant.CtxKeyTraceID))       //nolint:staticcheck // SA1029 项目统一使用 string 类型 context key，迁移至自定义类型需跨层联动
	dst = context.WithValue(dst, constant.CtxKeyUserID, src.Value(constant.CtxKeyUserID))         //nolint:staticcheck // SA1029 项目统一使用 string 类型 context key
	dst = context.WithValue(dst, constant.CtxKeyUserName, src.Value(constant.CtxKeyUserName))     //nolint:staticcheck // SA1029 项目统一使用 string 类型 context key
	dst = context.WithValue(dst, constant.CtxKeyPermission, src.Value(constant.CtxKeyPermission)) //nolint:staticcheck // SA1029 项目统一使用 string 类型 context key
	dst = context.WithValue(dst, constant.CtxKeyAPIKeyID, src.Value(constant.CtxKeyAPIKeyID))     //nolint:staticcheck // SA1029 项目统一使用 string 类型 context key
	dst = context.WithValue(dst, constant.CtxKeyClient, src.Value(constant.CtxKeyClient))         //nolint:staticcheck // SA1029 项目统一使用 string 类型 context key
	return dst
}

// CtxValueString 安全获取上下文中的字符串值
//
//	param ctx context.Context
//	param key string
//	return string
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

// CtxValueUint 安全获取上下文中的uint值
//
//	param ctx context.Context
//	param key string
//	return uint
//	@author centonhuang
//	@update 2026-03-31 10:00:00
func CtxValueUint(ctx context.Context, key string) uint {
	if v := ctx.Value(key); v != nil {
		switch n := v.(type) {
		case uint:
			return n
		case int:
			if n >= 0 {
				return uint(n)
			}
		}
	}
	return 0
}

// CtxValuePermission 安全获取上下文中的 Permission 值
//
// JWT 中间件存入的是 enum.Permission（named string），无法用 v.(string) 断言，需单独处理。
//
//	param ctx context.Context
//	return enum.Permission
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func CtxValuePermission(ctx context.Context) enum.Permission {
	if v := ctx.Value(constant.CtxKeyPermission); v != nil {
		if p, ok := v.(enum.Permission); ok {
			return p
		}
		if s, ok := v.(string); ok {
			return enum.Permission(s)
		}
	}
	return ""
}
