package constant

import "time"

const (
	HTTPMaxIdleConns          = 100
	HTTPMaxIdleConnsPerHost   = 20
	HTTPClientTimeout         = 5 * time.Minute
	HTTPDialTimeout           = 10 * time.Second
	HTTPKeepAlive             = 30 * time.Second
	HTTPTLSHandshakeTimeout   = 10 * time.Second
	HTTPResponseHeaderTimeout = 30 * time.Second
	HTTPIdleConnTimeout       = 90 * time.Second

	// HTTPLowerHeader 小写格式的HTTP头部（用于请求/响应）
	HTTPLowerHeaderContentLength      = "content-length"
	HTTPLowerHeaderAcceptEncoding     = "accept-encoding"
	HTTPLowerHeaderAPIKey             = "x-api-key"
	HTTPLowerHeaderAnthropicVersion   = "anthropic-version"
	HTTPLowerHeaderConnection         = "connection"
	HTTPLowerHeaderTransferEncoding   = "transfer-encoding"
	HTTPLowerHeaderHost               = "host"
	HTTPLowerHeaderUpgrade            = "upgrade"
	HTTPLowerHeaderTE                 = "te"
	HTTPLowerHeaderTrailer            = "trailer"
	HTTPLowerHeaderProxyAuthorization = "proxy-authorization"
	HTTPLowerHeaderProxyAuthenticate  = "proxy-authenticate"
	HTTPLowerHeaderTraceID            = "x-trace-id"
	HTTPLowerHeaderAuthorization      = "authorization"

	// HTTPTitleHeader Title-Case格式的HTTP头部（用于响应/标准头部）
	HTTPTitleHeaderContentType         = "Content-Type"
	HTTPTitleHeaderCacheControl        = "Cache-Control"
	HTTPTitleHeaderXAccelBuffering     = "X-Accel-Buffering"
	HTTPTitleHeaderUserAgent           = "User-Agent"
	HTTPTitleHeaderLastModified        = "Last-Modified"
	HTTPTitleHeaderETag                = "ETag"
	HTTPTitleHeaderCookie              = "Cookie"
	HTTPTitleHeaderSetCookie           = "Set-Cookie"
	HTTPTitleHeaderXRateLimitLimit     = "X-RateLimit-Limit"
	HTTPTitleHeaderXRateLimitRemaining = "X-RateLimit-Remaining"
	HTTPTitleHeaderRetryAfter          = "Retry-After"
	HTTPTitleHeaderAuthorization       = "Authorization"
	HTTPTitleHeaderAPIKey              = "X-Api-Key"

	HTTPAuthBearerPrefix              = "Bearer "
	HTTPContentTypeJSON               = "application/json"
	HTTPContentTypeEventStream        = "text/event-stream"
	HTTPLowerHeaderContentDisposition = "response-content-disposition"
	HTTPLowerHeaderContentType        = "response-content-type"
	HTTPAttachmentFilenameTemplate    = "attachment; filename=%q"
	HTTPCacheControlNoCache           = "no-cache"
	HTTPConnectionKeepAlive           = "keep-alive"
	HTTPTransferEncodingChunked       = "chunked"
	HTTPHeaderDisabled                = "no"

	MIMETypeOctetStream = "application/octet-stream"

	CORSAllowOrigins    = "http://localhost:3000"
	CORSPreflightMaxAge = 12 * time.Hour

	IdleTimeout          = 2 * time.Minute
	ShutdownTimeout      = 60 * time.Second
	FiberShutdownTimeout = 30 * time.Second
)
