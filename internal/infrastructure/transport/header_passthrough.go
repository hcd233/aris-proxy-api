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
	// 上游 OpenCode（Console Go 等，如 opencode.ai/zen/go/v1）要求 x-opencode-session 会话头
	// 用于路由与 prompt 缓存，缺失时返回 400 MissingSessionID。客户端已带 x-opencode-session 则
	// 原样保留；否则回退取 X-Session-Id / x-session-id（同一 canonical 头，ZCode / openrouter 等
	// 客户端形态自带）的值补上，避免无谓的上游 400。
	ensureOpencodeSessionHeader(header)
}

// ensureOpencodeSessionHeader 仅在缺失时用客户端会话头回填 x-opencode-session；
// 客户端未带任何会话头时不虚构值（保持原转发行为）。
func ensureOpencodeSessionHeader(header http.Header) {
	if header.Get(constant.HTTPHeaderOpencodeSession) != "" {
		return
	}
	if sessionID := header.Get(constant.HTTPHeaderSessionID); sessionID != "" {
		header.Set(constant.HTTPHeaderOpencodeSession, sessionID)
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
