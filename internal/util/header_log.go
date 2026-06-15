package util

import (
	"net/http"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/samber/lo"
)

var sensitiveHeadersForLog = []string{
	constant.HTTPHeaderAuthorization,
	constant.HTTPHeaderAPIKey,
	constant.HTTPHeaderProxyAuthorization,
	constant.HTTPHeaderCookie,
	constant.HTTPHeaderSetCookie,
}

// MaskHTTPHeadersForLog 返回可安全写入日志的 HTTP 请求头副本。
func MaskHTTPHeadersForLog(headers http.Header) map[string]any {
	return lo.MapValues(headers, func(values []string, key string) any {
		if isSensitiveHTTPHeaderForLog(key) {
			return constant.MaskSecretPlaceholder
		}
		return headerValuesForLog(values)
	})
}

func isSensitiveHTTPHeaderForLog(key string) bool {
	return lo.ContainsBy(sensitiveHeadersForLog, func(h string) bool { return strings.EqualFold(key, h) })
}

func headerValuesForLog(values []string) any {
	if len(values) == 1 {
		return values[0]
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}
