package util

import (
	"net/url"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// IsSafeInstallHost 校验 Host 头是否仅包含 URL host 允许的字符。
//
// trace 安装脚本模板将 origin 直接嵌入 bash 脚本（host='{{.Host}}'），
// Host 头由客户端可控，若包含单引号 / 反引号 / $() 等字符可突破 shell 字符串注入任意命令。
// 白名单仅放行字母数字与 . : [ ] -（覆盖域名、端口、IPv6 字面量）。
//
//	@param host string 请求 Host 头值（含端口）
//	@return bool
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func IsSafeInstallHost(host string) bool {
	if host == "" {
		return false
	}
	for i := range len(host) {
		c := host[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == ':', c == '[', c == ']', c == '-':
		default:
			return false
		}
	}
	return true
}

// SafeInstallOrigin 校验安装脚本请求头并组装 origin（scheme://host）。
//
// 模板将 origin 嵌入 bash 单引号串（host='{{.Host}}'），X-Forwarded-Proto 与 Host
// 均为客户端可控输入，三点防御缺一不可：
//
//  1. scheme 空值回退 http，非空必须严格为 http/https——否则 `https://evil/';cmd;'`
//     形态的值会让 url.Parse 把载荷挪进 Path，而 parsed.Host 仍"合法"；
//  2. 校验原始 Host 头值（而非 parsed.Host）——url.Parse 会把 `/`、`'`、`;` 之后的
//     载荷截到 Path，只校验 parsed.Host 会漏掉这些字符；
//  3. 只返回 parsed.Scheme://parsed.Host 已验证分量，绝不回填原始整串。
//
// IPv6 字面量（含方括号）在此层按语义拒绝：安装脚本面向域名/IPv4 访问形态，
// 收紧可最小化嵌入 shell 引号串的字符集（含 `a[b` 这类畸形方括号 host）；
// IsSafeInstallHost 的字符白名单保持放行方括号不变，供其他调用方复用。
//
//	@param scheme string X-Forwarded-Proto 头值（可空，空值回退 http）
//	@param host string Host 头值（含端口）
//	@return string 校验通过的 origin（parsed.Scheme://parsed.Host）
//	@return bool 是否通过校验
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func SafeInstallOrigin(scheme, host string) (string, bool) {
	if scheme == "" {
		scheme = constant.HTTPSchemeHTTP
	}
	isValidScheme := scheme == constant.HTTPSchemeHTTP || scheme == constant.HTTPSchemeHTTPS
	isIPv6Literal := strings.ContainsAny(host, "[]")
	if !isValidScheme || !IsSafeInstallHost(host) || isIPv6Literal {
		return "", false
	}
	parsed, err := url.Parse(scheme + "://" + host)
	if err != nil {
		return "", false
	}
	isValidParsedScheme := parsed.Scheme == constant.HTTPSchemeHTTP || parsed.Scheme == constant.HTTPSchemeHTTPS
	if !isValidParsedScheme || parsed.Host == "" {
		return "", false
	}
	return parsed.Scheme + "://" + parsed.Host, true
}
