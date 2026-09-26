package util

import (
	"net"
	"net/url"
	"slices"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// LookupIPFn 主机名 DNS 解析函数；包级可注入以便测试替换，生产默认 net.LookupIP。
var LookupIPFn = net.LookupIP

// ValidateEndpointBaseURL 校验 endpoint 上游 baseURL，防 SSRF（内网探测 / 云元数据读取）。
//
// 规则：scheme 仅 http/https（大小写不敏感，url.Parse 归一化）、host 非空、禁止 userinfo；
// IP 字面量与 DNS 解析结果均拒绝回环（127.0.0.0/8、::1）、链路本地（169.254.0.0/16、
// fe80::/10）、ULA（fc00::/7）、RFC1918（10/8、172.16/12、192.168/16）与未指定地址
// （0.0.0.0、::，多数协议栈等价本机）；hostname 拒绝 localhost、*.localhost、
// metadata.google.internal、metadata.goog；hostname 必须能解析，任一解析结果落在上述
// 区间或解析失败均按校验失败处理。
//
// DNS rebinding / TOCTOU（校验解析与实际请求之间解析记录被换）属残留风险，
// 生产侧由 K8s egress 策略兜底。
//
//	@param raw string 待校验的 baseURL
//	@return error 非法时返回 ierr.ErrValidation（文案不含解析结果等敏感细节）
//	@author centonhuang
//	@update 2026-09-26 10:00:00
func ValidateEndpointBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ierr.New(ierr.ErrValidation, "invalid endpoint base url")
	}
	isValidScheme := parsed.Scheme == constant.HTTPSchemeHTTP || parsed.Scheme == constant.HTTPSchemeHTTPS
	if !isValidScheme || parsed.Host == "" {
		return ierr.New(ierr.ErrValidation, "invalid endpoint base url")
	}
	if parsed.User != nil {
		return ierr.New(ierr.ErrValidation, "endpoint base url with userinfo is not allowed")
	}

	// 归一化主机名：小写 + 去尾点（`localhost.` 等 FQDN 写法不绕过黑名单）。
	hostname := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if ip := net.ParseIP(hostname); ip != nil {
		if isBlockedIP(ip) {
			return ierr.New(ierr.ErrValidation, "endpoint base url host is not allowed")
		}
		return nil
	}
	if isBlockedHostname(hostname) {
		return ierr.New(ierr.ErrValidation, "endpoint base url host is not allowed")
	}

	ips, err := LookupIPFn(hostname)
	if err != nil || len(ips) == 0 {
		return ierr.New(ierr.ErrValidation, "endpoint base url host cannot be resolved")
	}
	if slices.ContainsFunc(ips, isBlockedIP) {
		return ierr.New(ierr.ErrValidation, "endpoint base url host is not allowed")
	}
	return nil
}

// isBlockedIP 判断 IP 是否落在内网/元数据目标区间。
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() || ip.IsUnspecified()
}

// isBlockedHostname 判断主机名是否属于 localhost / 云元数据黑名单（调用前已归一化）。
func isBlockedHostname(hostname string) bool {
	isLocal := hostname == constant.HostnameLocalhost || strings.HasSuffix(hostname, constant.HostnameLocalhostSuffix)
	isMetadata := hostname == constant.HostnameMetadataGoogleInternal || hostname == constant.HostnameMetadataGoog
	return isLocal || isMetadata
}
