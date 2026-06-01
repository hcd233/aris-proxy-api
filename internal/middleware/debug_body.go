package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// DebugBodyMiddleware 请求体调试中间件
//
//	在 Huma DTO unmarshal 之前打印原始请求体，仅当 config.DebugRequestBodyEnabled 为 true 时生效。
//	必须注册在 TraceMiddleware 之后（确保 traceID 可用）且在任何 Huma handler 之前。
//	@update 2026-06-02 10:00:00
func DebugBodyMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !config.DebugRequestBodyEnabled {
			return c.Next()
		}

		if strings.Contains(string(c.Request().Header.ContentType()), constant.HTTPContentTypeJSON) {
			if body := c.Body(); len(body) > 0 {
				logger.WithFCtx(c).Info("[DebugBody] Raw request body before Huma unmarshal",
					zap.String("method", c.Method()),
					zap.String("path", c.Path()),
					zap.String("query", string(c.Request().URI().QueryString())),
					zap.ByteString("body", body),
				)
			}
		}

		return c.Next()
	}
}
