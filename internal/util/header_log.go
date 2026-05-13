package util

import (
	"net/http"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

var sensitiveHeadersForLog = []string{
	constant.HTTPLowerHeaderAuthorization,
	constant.HTTPTitleHeaderAuthorization,
	constant.HTTPLowerHeaderAPIKey,
	constant.HTTPTitleHeaderAPIKey,
	constant.HTTPLowerHeaderProxyAuthorization,
	constant.HTTPTitleHeaderCookie,
	constant.HTTPTitleHeaderSetCookie,
}

// MaskHTTPHeadersForLog 返回可安全写入日志的 HTTP 请求头副本。
func MaskHTTPHeadersForLog(headers http.Header) map[string]any {
	masked := make(map[string]any, len(headers))
	for key, values := range headers {
		if isSensitiveHTTPHeaderForLog(key) {
			masked[key] = constant.MaskSecretPlaceholder
			continue
		}
		masked[key] = headerValuesForLog(values)
	}
	return masked
}

func isSensitiveHTTPHeaderForLog(key string) bool {
	for _, header := range sensitiveHeadersForLog {
		if strings.EqualFold(key, header) {
			return true
		}
	}
	return false
}

func headerValuesForLog(values []string) any {
	if len(values) == 1 {
		return values[0]
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}
