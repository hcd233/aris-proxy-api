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

	HTTPHeaderContentLength      = "content-length"
	HTTPHeaderAcceptEncoding     = "accept-encoding"
	HTTPHeaderAPIKey             = "x-api-key"
	HTTPHeaderAnthropicVersion   = "anthropic-version"
	HTTPHeaderConnection         = "connection"
	HTTPHeaderTransferEncoding   = "transfer-encoding"
	HTTPHeaderHost               = "host"
	HTTPHeaderUpgrade            = "upgrade"
	HTTPHeaderTE                 = "te"
	HTTPHeaderTrailer            = "trailer"
	HTTPHeaderProxyAuthorization = "proxy-authorization"
	HTTPHeaderProxyAuthenticate  = "proxy-authenticate"
	HTTPHeaderTraceID            = "x-trace-id"

	HTTPHeaderContentType         = "Content-Type"
	HTTPHeaderAuthorization       = "Authorization"
	HTTPHeaderCacheControl        = "Cache-Control"
	HTTPHeaderXAccelBuffering     = "X-Accel-Buffering"
	HTTPHeaderUserAgent           = "User-Agent"
	HTTPHeaderLastModified        = "Last-Modified"
	HTTPHeaderETag                = "ETag"
	HTTPHeaderCookie              = "Cookie"
	HTTPHeaderSetCookie           = "Set-Cookie"
	HTTPHeaderXRateLimitLimit     = "X-RateLimit-Limit"
	HTTPHeaderXRateLimitRemaining = "X-RateLimit-Remaining"
	HTTPHeaderRetryAfter          = "Retry-After"

	HTTPAuthBearerPrefix           = "Bearer "
	HTTPContentTypeJSON            = "application/json"
	HTTPContentTypeEventStream     = "text/event-stream"
	HTTPContentDispositionParam    = "response-content-disposition"
	HTTPContentTypeParam           = "response-content-type"
	HTTPAttachmentFilenameTemplate = "attachment; filename=%q"
	HTTPCacheControlNoCache        = "no-cache"
	HTTPConnectionKeepAlive        = "keep-alive"
	HTTPTransferEncodingChunked    = "chunked"
	HTTPHeaderDisabled             = "no"

	MIMETypeOctetStream = "application/octet-stream"

	CORSAllowOrigins    = "http://localhost:3000"
	CORSPreflightMaxAge = 12 * time.Hour

	IdleTimeout          = 2 * time.Minute
	ShutdownTimeout      = 60 * time.Second
	FiberShutdownTimeout = 30 * time.Second
)
