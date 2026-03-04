package middleware

import (
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// LogMiddleware 日志中间件
//
//	param logger *zap.Logger
//	return fiber.Handler
//	author centonhuang
//	update 2025-01-05 21:21:46
func LogMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now().UTC()
		path := c.Path()
		query := string(c.Request().URI().QueryString())

		err := c.Next()

		logger := logger.WithFCtx(c)

		latency := time.Since(start)

		fields := []zap.Field{
			zap.Int("status", c.Response().StatusCode()),
			zap.String("method", c.Method()),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.IP()),
			zap.String("user-agent", c.Get("User-Agent")),
			zap.String("latency", latency.String()),
		}

		if strings.Contains(string(c.Request().Header.ContentType()), "application/json") {
			request := make(map[string]interface{})
			if reqBody := c.Body(); reqBody != nil {
				if err := sonic.Unmarshal(reqBody, &request); err != nil {
					logger.Warn("[LogMiddleware] unmarshal request error", zap.ByteString("request", reqBody), zap.Error(err))
				}
			}
			fields = append(fields, zap.Dict("request", lo.MapToSlice(request, func(key string, value interface{}) zap.Field {
				return zap.Any(key, value)
			})...))
		}

		// FIXME: get response body will break sse
		// reference: https://github.com/gofiber/fiber/issues/429
		// reference: https://github.com/samber/slog-fiber/issues/68
		if strings.Contains(string(c.Response().Header.ContentType()), "application/json") { // response header content-type is not text/event-stream
			response := make(map[string]interface{})
			if respBody := c.Response().Body(); respBody != nil {
				if err := sonic.Unmarshal(respBody, &response); err != nil {
					logger.Warn("[LogMiddleware] unmarshal response error", zap.ByteString("response", respBody), zap.Error(err))
				}
			}
			fields = append(fields, zap.Dict("response", lo.MapToSlice(response, func(key string, value interface{}) zap.Field {
				return zap.Any(key, value)
			})...))
		}

		if err != nil {
			fields = append([]zap.Field{zap.Error(err)}, fields...)
			logger.Error("[LogMiddleware] error", fields...)
		} else {
			logger.Info("[LogMiddleware] info", fields...)
		}

		return err
	}
}
