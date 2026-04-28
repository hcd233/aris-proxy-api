package transport

import (
	"context"
	"net/http"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// responsePassthroughExcludedHeaders 不从上游透传到客户端的响应头
var responsePassthroughExcludedHeaders = map[string]struct{}{
	constant.HTTPHeaderContentType:       {},
	constant.HTTPHeaderContentLength:     {},
	constant.HTTPHeaderTransferEncoding:  {},
	constant.HTTPHeaderConnection:        {},
	constant.HTTPHeaderUpgrade:           {},
	constant.HTTPHeaderTrailer:           {},
	constant.HTTPHeaderProxyAuthenticate: {},
	constant.HTTPHeaderTraceID:           {},
}

// isPassthroughResponseHeader 判断响应头是否应透传
func isPassthroughResponseHeader(name string) bool {
	_, excluded := responsePassthroughExcludedHeaders[name]
	return !excluded
}

// capturePassthroughResponseHeaders 从上游响应中提取需要透传的响应头
func capturePassthroughResponseHeaders(header http.Header) map[string]string {
	headers := make(map[string]string, 4)
	for k := range header {
		canonical := http.CanonicalHeaderKey(k)
		if isPassthroughResponseHeader(canonical) {
			headers[canonical] = header.Get(k)
		}
	}
	return headers
}

// storePassthroughResponseHeaders 将响应头存入 context 的 map 中
func storePassthroughResponseHeaders(ctx context.Context, header http.Header) {
	if m := util.GetPassthroughResponseHeaders(ctx); m != nil {
		for k := range header {
			canonical := http.CanonicalHeaderKey(k)
			if isPassthroughResponseHeader(canonical) {
				m[canonical] = header.Get(k)
			}
		}
	}
}
