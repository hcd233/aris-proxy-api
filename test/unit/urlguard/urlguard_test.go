// Package urlguard 验证 endpoint baseURL 的 SSRF 校验（util.ValidateEndpointBaseURL）。
package urlguard

import (
	"net"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/util"
)

// fakeLookupIP 按主机名返回预设解析结果，避免真实 DNS（CI 无网 / 解析抖动）。
func fakeLookupIP(host string) ([]net.IP, error) {
	switch host {
	case "api.example.com":
		return []net.IP{net.ParseIP("203.0.113.10")}, nil
	case "dual.example.com":
		// 任一结果落在内网区间即整体拒绝
		return []net.IP{net.ParseIP("203.0.113.10"), net.ParseIP("10.0.0.1")}, nil
	case "loopback.example.com":
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	case "metadata-ip.example.com":
		return []net.IP{net.ParseIP("169.254.169.254")}, nil
	default:
		return nil, &net.DNSError{Err: "no such host", Name: host}
	}
}

func init() {
	util.LookupIPFn = fakeLookupIP
}

// TestValidateEndpointBaseURL_Reject 表驱动覆盖全部拒绝分支：
// IP 字面量内网/元数据区间、localhost 与云元数据主机名、DNS 解析结果内网/失败、
// userinfo、畸形 URL、无 scheme、非 http(s) scheme。
//
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func TestValidateEndpointBaseURL_Reject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
	}{
		// IP 字面量：回环
		{"ipv4 loopback", "http://127.0.0.1"},
		{"ipv4 loopback with port and path", "http://127.0.0.1:8080/v1"},
		{"ipv6 loopback", "http://[::1]"},
		{"ipv6 loopback with port", "http://[::1]:8080"},
		// IP 字面量：链路本地 / 云元数据
		{"link local metadata", "http://169.254.169.254/latest/meta-data/"},
		{"ipv6 link local", "http://[fe80::1]"},
		// IP 字面量：ULA / RFC1918
		{"rfc1918 10", "http://10.0.0.5"},
		{"rfc1918 172.16", "http://172.16.0.1"},
		{"rfc1918 172.31 boundary", "http://172.31.255.255"},
		{"rfc1918 192.168", "http://192.168.1.1"},
		{"ula fd00", "http://[fd00::1]"},
		{"ula fc00", "http://[fc00::1]"},
		{"ipv4 mapped loopback", "http://[::ffff:127.0.0.1]"},
		// IP 字面量：未指定地址
		{"ipv4 unspecified", "http://0.0.0.0"},
		{"ipv6 unspecified", "http://[::]"},
		// 主机名黑名单
		{"localhost", "http://localhost"},
		{"localhost with port", "http://localhost:8080"},
		{"localhost uppercase", "http://LOCALHOST"},
		{"localhost trailing dot", "http://localhost."},
		{"localhost subdomain", "http://foo.localhost"},
		{"gcp metadata", "http://metadata.google.internal/computeMetadata/v1/"},
		{"gcp metadata short", "http://metadata.goog"},
		// DNS 分支：解析结果内网 / 解析失败（fakeLookupIP 中除白名单外均失败）
		{"dns resolves loopback", "http://loopback.example.com"},
		{"dns resolves link local", "http://metadata-ip.example.com"},
		{"dns mixed public and private", "http://dual.example.com"},
		{"dns nxdomain", "http://nxdomain.example.com"},
		// userinfo
		{"userinfo", "http://user:pass@api.example.com"},
		{"percent encoded userinfo to loopback", "http://evil.com%2f@127.0.0.1"},
		// 畸形 / 非 http(s)
		{"empty", ""},
		{"missing scheme", "example.com/v1"},
		{"empty host", "http://"},
		{"leading colon", "://example.com"},
		{"ftp scheme", "ftp://api.example.com"},
		{"backslash userinfo parse error", "http://example.com\\@127.0.0.1"},
		// 大写 scheme 不构成绕过：归一化后 IP 校验仍然拒绝
		{"uppercase scheme to link local", "HtTp://169.254.169.254"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := util.ValidateEndpointBaseURL(tc.raw); err == nil {
				t.Fatalf("ValidateEndpointBaseURL(%q) = nil, want error", tc.raw)
			}
		})
	}
}

// TestValidateEndpointBaseURL_Allow 表驱动覆盖放行分支：
// 域名（含大写 scheme 归一化、带端口、带路径）与解析结果为公网 IP 的主机名。
//
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func TestValidateEndpointBaseURL_Allow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		raw  string
	}{
		{"public hostname", "https://api.example.com"},
		{"public hostname with port and path", "https://api.example.com:8443/v1"},
		{"uppercase scheme normalized", "HTTPS://api.example.com"},
		{"mixed case scheme normalized", "HtTp://api.example.com"},
		{"public ip literal", "https://203.0.113.10"},
		{"public ipv6 literal", "https://[2001:db8::1]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := util.ValidateEndpointBaseURL(tc.raw); err != nil {
				t.Fatalf("ValidateEndpointBaseURL(%q) = %v, want nil", tc.raw, err)
			}
		})
	}
}
