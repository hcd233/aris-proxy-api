package util

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
