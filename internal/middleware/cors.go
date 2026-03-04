package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORSMiddleware 跨域中间件
//
//	return gin.HandlerFunc
//	author centonhuang
//	update 2024-09-16 04:07:30
func CORSMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With,X-Trace-Id",
		ExposeHeaders:    "Content-Length",
		AllowCredentials: true,
		MaxAge:           int(12 * time.Hour.Seconds()),
	})
}
