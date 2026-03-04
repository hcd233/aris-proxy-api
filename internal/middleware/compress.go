package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
)

// CompressMiddleware 压缩中间件
//
//	@return fiber.Handler
//	@author centonhuang
//	@update 2025-08-18 20:22:36
func CompressMiddleware() fiber.Handler {
	return compress.New(compress.Config{
		Level: compress.LevelDefault,
	})
}
