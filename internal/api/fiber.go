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
		// fiber 默认 4MB（BodyLimit<=0 回落默认），超限直接 413。
		BodyLimit:   constant.MaxHTTPBodyBytes,
		JSONEncoder: sonic.Marshal,
		JSONDecoder: sonic.Unmarshal,
		TrustProxy:  true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: config.TrustedProxies,
		},
		ProxyHeader: fiber.HeaderXForwardedFor,
	})
}
