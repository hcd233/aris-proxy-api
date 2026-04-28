// Package constant 常量
package constant

import "github.com/hcd233/aris-proxy-api/internal/common/enum"

const (

	// CtxKeyUserID undefined
	//	@update 2025-09-30 15:57:05
	CtxKeyUserID enum.CtxKey = "userID"

	// CtxKeyUserName undefined
	//	@update 2025-09-30 15:57:07
	CtxKeyUserName enum.CtxKey = "userName"

	// CtxKeyPermission undefined
	//	@update 2025-09-30 15:57:08
	CtxKeyPermission enum.CtxKey = "permission"
	// CtxKeyTraceID undefined
	//	@update 2025-09-30 15:57:13
	CtxKeyTraceID enum.CtxKey = "traceID"

	// CtxKeyLimiter undefined
	//	@update 2025-09-30 15:57:14
	CtxKeyLimiter enum.CtxKey = "limiter"

	// CtxKeyClient 请求客户端User-Agent
	//	@update 2026-03-29 10:00:00
	CtxKeyClient enum.CtxKey = "client"

	// CtxKeyAPIKeyID API Key ID（用于日志追踪）
	//	@update 2026-04-08 10:00:00
	CtxKeyAPIKeyID enum.CtxKey = "apiKeyID"

	// CtxKeyPassthroughHeaders 透传到上游的请求头
	//	@update 2026-04-28 10:00:00
	CtxKeyPassthroughHeaders enum.CtxKey = "passthroughHeaders"
)
