package api

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
)

var fiberApp *fiber.App

// GetFiberApp 获取 Fiber 应用实例
//
//	@return *fiber.App
//	@author centonhuang
//	@update 2025-11-02 02:35:59
func GetFiberApp() *fiber.App {
	return fiberApp
}

func init() {
	fiberApp = fiber.New(fiber.Config{
		Prefork:           false,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       constant.IdleTimeout,
		JSONEncoder:       sonic.Marshal,
		JSONDecoder:       sonic.Unmarshal,
		StreamRequestBody: true,
	})
}
