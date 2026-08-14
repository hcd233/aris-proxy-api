package constant

import "time"

const (
	HTTPMaxIdleConns          = 100
	HTTPMaxIdleConnsPerHost   = 20
	HTTPClientTimeout         = 5 * time.Minute
	HTTPDialTimeout           = 10 * time.Second
	HTTPKeepAlive             = 30 * time.Second
	HTTPTLSHandshakeTimeout   = 10 * time.Second
	HTTPResponseHeaderTimeout = 60 * time.Second
	HTTPIdleConnTimeout       = 90 * time.Second

	// HTTPHeader HTTP 头部常量（全部使用标准 Title-Case 格式）
	HTTPHeaderAcceptEncoding      = "Accept-Encoding"
	HTTPHeaderAcceptLanguage      = "Accept-Language"
	HTTPHeaderAuthorization       = "Authorization"
	HTTPHeaderAPIKey              = "X-Api-Key"
	HTTPHeaderAnthropicVersion    = "Anthropic-Version"
	HTTPHeaderCacheControl        = "Cache-Control"
	HTTPHeaderConnection          = "Connection"
	HTTPHeaderContentLength       = "Content-Length"
	HTTPHeaderContentDisposition  = "Content-Disposition"
	HTTPHeaderContentType         = "Content-Type"
	HTTPHeaderCookie              = "Cookie"
	HTTPHeaderETag                = "ETag"
	HTTPHeaderHost                = "Host"
	HTTPHeaderLastModified        = "Last-Modified"
	HTTPHeaderProxyAuthenticate   = "Proxy-Authenticate"
	HTTPHeaderProxyAuthorization  = "Proxy-Authorization"
	HTTPHeaderRemoteHost          = "Remote-Host"
	HTTPHeaderRetryAfter          = "Retry-After"
	HTTPHeaderSetCookie           = "Set-Cookie"
	HTTPHeaderTE                  = "TE"
	HTTPHeaderTraceID             = "X-Trace-Id"
	HTTPHeaderTrailer             = "Trailer"
	HTTPHeaderTransferEncoding    = "Transfer-Encoding"
	HTTPHeaderUpgrade             = "Upgrade"
	HTTPHeaderUserAgent           = "User-Agent"
	HTTPHeaderXAccelBuffering     = "X-Accel-Buffering"
	HTTPHeaderXForwardedFor       = "X-Forwarded-For"
	HTTPHeaderXForwardedPort      = "X-Forwarded-Port"
	HTTPHeaderXForwardedProto     = "X-Forwarded-Proto"
	HTTPHeaderXRateLimitLimit     = "X-RateLimit-Limit"
	HTTPHeaderXRateLimitRemaining = "X-RateLimit-Remaining"
	HTTPHeaderXRealIP             = "X-Real-IP"

	HTTPAuthBearerPrefix           = "Bearer "
	HTTPContentTypeJSON            = "application/json"
	HTTPContentTypeProblemJSON     = "application/problem+json"
	HTTPContentTypeEventStream     = "text/event-stream"
	HTTPContentTypeTextPlain       = "text/plain; charset=utf-8"
	HTTPAttachmentFilenameTemplate = "attachment; filename=%q"
	HTTPCacheControlNoCache        = "no-cache"
	HTTPCacheControlNoStore        = "no-store"
	HTTPConnectionKeepAlive        = "keep-alive"
	HTTPTransferEncodingChunked    = "chunked"
	HTTPHeaderDisabled             = "no"

	HTTPSchemeHTTP  = "http"
	HTTPSchemeHTTPS = "https"

	// MaxLLMProxyBodyBytes LLM 代理路由请求体大小上限（huma Operation.MaxBodyBytes）。
	// huma 语义：0 为默认 1MB，-1 为不限制。LLM 请求体可能包含长上下文、多模态
	// base64 内容（单图即可达数 MB），远超默认 1MB 限制，故代理路由显式放开 huma 层限制。
	// 注意：请求体仍受 fiber 层 MaxHTTPBodyBytes 兜底（见 api/fiber.go BodyLimit），
	// 防止无限 body + 全量内存缓冲导致的内存 DoS。
	MaxLLMProxyBodyBytes int64 = -1

	// MaxHTTPBodyBytes fiber 层全局请求体上限（BodyLimit）。
	// fiber 默认 4MB（BodyLimit<=0 回落默认），超过即 413；
	// 管理路由仍受 huma 默认 1MB 限制，此处仅兜底 LLM 代理路由放开后的大 body。
	MaxHTTPBodyBytes int = 16 * 1024 * 1024

	MIMETypeOctetStream = "application/octet-stream"

	CORSAllowOrigins    = "http://localhost:3000"
	CORSPreflightMaxAge = 12 * time.Hour

	IdleTimeout                    = 2 * time.Minute
	ShutdownTimeout                = 10 * time.Minute
	CronStopTimeout                = 3 * time.Minute
	PoolStopTimeout                = 3 * time.Minute
	InflightDrainSoftTimeout       = 5 * time.Minute
	InflightDrainHardTimeout       = 30 * time.Second
	FiberShutdownTimeout           = 30 * time.Second
	InflightStateRunning     int32 = 0
	InflightStateDraining    int32 = 1
	ServerShuttingDownMsg          = "server is shutting down"
)
