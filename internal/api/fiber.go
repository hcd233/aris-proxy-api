package api

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
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
		Prefork:                  false,
		ReadTimeout:              config.ReadTimeout,
		WriteTimeout:             config.WriteTimeout,
		IdleTimeout:              constant.IdleTimeout,
		JSONEncoder:              sonic.Marshal,
		JSONDecoder:              sonic.Unmarshal,
		DisableHeaderNormalizing: true,
		EnableTrustedProxyCheck:  true,
		TrustedProxies:           config.TrustedProxies,
		ProxyHeader:              fiber.HeaderXForwardedFor,
	})
}
