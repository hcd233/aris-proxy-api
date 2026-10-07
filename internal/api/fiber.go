package api

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
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
		TrustProxy:  true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: config.TrustedProxies,
		},
		ProxyHeader: fiber.HeaderXForwardedFor,
	})
}
