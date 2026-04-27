package api

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
)

var fiberApp *fiber.App

// NewFiberApp 创建 Fiber 应用实例
//
//	@return *fiber.App
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func NewFiberApp() *fiber.App {
	return fiber.New(fiber.Config{
		Prefork:                 false,
		ReadTimeout:             config.ReadTimeout,
		WriteTimeout:            config.WriteTimeout,
		IdleTimeout:             constant.IdleTimeout,
		JSONEncoder:             sonic.Marshal,
		JSONDecoder:             sonic.Unmarshal,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          config.TrustedProxies,
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
}

// SetFiberApp 设置 Fiber 应用实例
//
//	@param app *fiber.App
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func SetFiberApp(app *fiber.App) {
	fiberApp = app
}

// GetFiberApp 获取 Fiber 应用实例
//
//	@return *fiber.App
//	@author centonhuang
//	@update 2025-11-02 02:35:59
func GetFiberApp() *fiber.App {
	return fiberApp
}
