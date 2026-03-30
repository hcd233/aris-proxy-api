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

	// ShutdownTimeout 优雅关闭的最大超时时间
	ShutdownTimeout = 60 * time.Second

	// HTTPClientTimeout HTTP客户端总超时时间（用于LLM请求）
	//	@author centonhuang
	//	@update 2026-03-31 10:00:00
	HTTPClientTimeout = 5 * time.Minute

	// HTTPDialTimeout TCP连接建立超时时间
	//	@author centonhuang
	//	@update 2026-03-31 10:00:00
	HTTPDialTimeout = 10 * time.Second

	// HTTPKeepAlive TCP连接保活间隔
	//	@author centonhuang
	//	@update 2026-03-31 10:00:00
	HTTPKeepAlive = 30 * time.Second

	// HTTPTLSHandshakeTimeout TLS握手超时时间
	//	@author centonhuang
	//	@update 2026-03-31 10:00:00
	HTTPTLSHandshakeTimeout = 10 * time.Second

	// HTTPResponseHeaderTimeout 等待响应头超时时间
	//	@author centonhuang
	//	@update 2026-03-31 10:00:00
	HTTPResponseHeaderTimeout = 30 * time.Second

	// HTTPIdleConnTimeout 空闲连接回收时间
	//	@author centonhuang
	//	@update 2026-03-31 10:00:00
	HTTPIdleConnTimeout = 90 * time.Second
)
