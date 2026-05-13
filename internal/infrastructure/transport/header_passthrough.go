package transport

import (
	"context"
	"net/http"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// responsePassthroughExcludedHeaders 不从上游透传到客户端的响应头
var responsePassthroughExcludedHeaders = map[string]struct{}{
	constant.HTTPTitleHeaderContentType:       {},
	constant.HTTPLowerHeaderContentLength:     {},
	constant.HTTPLowerHeaderTransferEncoding:  {},
	constant.HTTPLowerHeaderConnection:        {},
	constant.HTTPLowerHeaderUpgrade:           {},
	constant.HTTPLowerHeaderTrailer:           {},
	constant.HTTPLowerHeaderProxyAuthenticate: {},
	constant.HTTPTitleHeaderTraceID:           {},
}

// isPassthroughResponseHeader 判断响应头是否应透传
func isPassthroughResponseHeader(name string) bool {
	_, excluded := responsePassthroughExcludedHeaders[name]
	return !excluded
}

// applyPassthroughRequestHeaders 将客户端请求头按原始大小写写入上游请求。
func applyPassthroughRequestHeaders(ctx context.Context, header http.Header) {
	if headers := util.GetPassthroughHeaders(ctx); headers != nil {
		for k, v := range headers {
			setRequestHeader(header, k, v)
		}
	}
}

// setRequestHeader 写入请求头时不触发 http.Header.Set 的 CanonicalMIMEHeaderKey 转换。
func setRequestHeader(header http.Header, name, value string) {
	for existing := range header {
		if strings.EqualFold(existing, name) {
			delete(header, existing)
		}
	}
	header[name] = []string{value}
}

// capturePassthroughResponseHeaders 从上游响应中提取需要透传的响应头
func capturePassthroughResponseHeaders(header http.Header) map[string]string {
	headers := make(map[string]string, 4)
	for k := range header {
		if isPassthroughResponseHeader(k) {
			headers[k] = header.Get(k)
		}
	}
	return headers
}

// storePassthroughResponseHeaders 将响应头存入 context 的 map 中
func storePassthroughResponseHeaders(ctx context.Context, header http.Header) {
	if m := util.GetPassthroughResponseHeaders(ctx); m != nil {
		for k := range header {
			if isPassthroughResponseHeader(k) {
				m[k] = header.Get(k)
			}
		}
	}
}
