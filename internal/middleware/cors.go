package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// CORSMiddleware 跨域中间件
//
//	return gin.HandlerFunc
//	author centonhuang
//	update 2024-09-16 04:07:30
func CORSMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     constant.CORSAllowOrigins,
		AllowMethods:     constant.CORSAllowMethods,
		AllowHeaders:     constant.CORSAllowHeaders,
		ExposeHeaders:    constant.CORSExposeHeaders,
		AllowCredentials: true,
		MaxAge:           int(constant.CORSPreflightMaxAge.Seconds()),
	})
}
