package constant

import "time"

const (
	// HeartbeatInterval SSE心跳间隔
	//
	//	@author centonhuang
	//	@update 2025-11-08 04:43:54
	HeartbeatInterval = 1 * time.Second

	// PresignObjectExpire 预签名对象过期时间
	//	@update 2025-11-12 19:20:26
	PresignObjectExpire = 5 * time.Minute

	// IdleTimeout Fiber应用空闲超时时间
	//	@update 2025-11-19 16:00:00
	IdleTimeout = 2 * time.Minute
)
