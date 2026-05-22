package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// CORSMiddleware 跨域中间件
//
//	return gin.HandlerFunc
//	author centonhuang
//	update 2024-09-16 04:07:30
func CORSMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     strings.Split(constant.CORSAllowOrigins, ","),
		AllowMethods:     strings.Split(constant.CORSAllowMethods, ","),
		AllowHeaders:     strings.Split(constant.CORSAllowHeaders, ","),
		ExposeHeaders:    strings.Split(constant.CORSExposeHeaders, ","),
		AllowCredentials: true,
		MaxAge:           int(constant.CORSPreflightMaxAge.Seconds()),
	})
}
