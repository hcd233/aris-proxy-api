package transport

import (
	"context"
	"net/http"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
)

// responsePassthroughExcludedHeaders 不从上游透传到客户端的响应头
// 上游响应头已被 Go 标准库标准化为 Title-Case，使用 HTTPHeader 常量匹配。
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

// applyPassthroughRequestHeaders 将客户端请求头写入上游请求。
func applyPassthroughRequestHeaders(ctx context.Context, header http.Header) {
	if headers := util.GetPassthroughHeaders(ctx); headers != nil {
		for k, v := range headers {
			header.Set(k, v)
		}
	}
}

// capturePassthroughResponseHeaders 从上游响应中提取需要透传的响应头
func capturePassthroughResponseHeaders(header http.Header) map[string]string {
	picked := lo.PickBy(header, func(k string, _ []string) bool { return isPassthroughResponseHeader(k) })
	headers := make(map[string]string, len(picked))
	for k, v := range picked {
		headers[k] = v[0]
	}
	return headers
}

// storePassthroughResponseHeaders 将响应头存入 context 的 map 中
func storePassthroughResponseHeaders(ctx context.Context, header http.Header) {
	if m := util.GetPassthroughResponseHeaders(ctx); m != nil {
		picked := lo.PickBy(header, func(k string, _ []string) bool { return isPassthroughResponseHeader(k) })
		for k, v := range picked {
			m[k] = v[0]
		}
	}
}
