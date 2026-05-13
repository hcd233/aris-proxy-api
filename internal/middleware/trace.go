package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TraceMiddleware 追踪中间件
//
//	return fiber.Handler
//	author centonhuang
//	update 2025-01-05 15:30:00
func TraceMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		traceID := uuid.New().String()

		c.Locals(constant.CtxKeyTraceID, traceID)

		c.Set(constant.HTTPTitleHeaderTraceID, traceID)

		return c.Next()
	}
}
