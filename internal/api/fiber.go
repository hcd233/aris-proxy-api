package api

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-mass/internal/common/constant"
	"github.com/hcd233/aris-mass/internal/config"
)

// NewFiberApp 创建 Fiber 应用实例
//
//	@return *fiber.App
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func NewFiberApp() *fiber.App {
	return fiber.New(fiber.Config{
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  constant.IdleTimeout,
		// BodyLimit 兜底 LLM 代理路由（huma 层 MaxBodyBytes=-1 不限）后的大 body；
		// 默认 16MB，HTTP_BODY_LIMIT 可覆盖（生产 48MB，需低于 openresty client_max_body_size 50m）；
		// 超限在 fasthttp 读请求阶段直接 413，不经过路由组中间件（应用日志不可见）。
		BodyLimit:   config.HTTPBodyLimit,
		JSONEncoder: sonic.Marshal,
		JSONDecoder: sonic.Unmarshal,
		// TrustProxy + Proxies：只有来自可信代理（openresty / k3s ServiceLB）的请求才采信 XFF，
		// 否则 c.IP() 回退 RemoteIP。k3s 的 svclb 会对 LoadBalancer 流量做 MASQUERADE，
		// 应用看到的 RemoteIP 恒为 svclb pod IP（10.42.x.x）；若不信任该网段，所有客户端
		// 都会退化成同一个网关 IP，扫描封禁/按 IP 限流/审计会集体失真。
		TrustProxy: true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: config.TrustedProxies,
		},
		ProxyHeader: fiber.HeaderXForwardedFor,
		// EnableIPValidation：采信 XFF 时取第一个合法 IP，避免把整串 header（可被伪造）
		// 原样作为 IP 使用，污染 scanner:ban / tb 等以 IP 为维度的 key。
		EnableIPValidation: true,
	})
}
