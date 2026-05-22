package middleware

import (
	"github.com/gofiber/contrib/v3/fgprof"
	"github.com/gofiber/fiber/v3"
)

// FgprofMiddleware fgprof中间件
//
//	@return fiber.Handler
//	@author centonhuang
//	@update 2025-09-25 21:17:02
func FgprofMiddleware() fiber.Handler {
	return fgprof.New()
}
