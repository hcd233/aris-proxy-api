package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// LogSamplingRule 日志采样规则
//
//	@author centonhuang
//	@update 2026-03-30 10:00:00
type LogSamplingRule struct {
	Path     string        // 需要采样的路径
	Interval time.Duration // 采样间隔，在此时间内最多打印一次日志
}

// LogMiddlewareConfig 日志中间件配置
//
//	@author centonhuang
//	@update 2026-03-30 10:00:00
type LogMiddlewareConfig struct {
	SamplingRules []LogSamplingRule // 路径采样规则列表
}

// logSampler 日志采样器，记录每个路径的上次打印时间
type logSampler struct {
	mu       sync.Mutex
	lastLogs map[string]time.Time
}

func (s *logSampler) shouldLog(path string, interval time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if last, ok := s.lastLogs[path]; ok && now.Sub(last) < interval {
		return false
	}
	s.lastLogs[path] = now
	return true
}

// LogMiddleware 日志中间件
//
//	@param cfg LogMiddlewareConfig
//	@return fiber.Handler
//	@author centonhuang
//	@update 2026-03-30 10:00:00
func LogMiddleware(cfg LogMiddlewareConfig) fiber.Handler {
	samplingIndex := make(map[string]time.Duration, len(cfg.SamplingRules))
	for _, rule := range cfg.SamplingRules {
		samplingIndex[rule.Path] = rule.Interval
	}

	sampler := &logSampler{lastLogs: make(map[string]time.Time, len(cfg.SamplingRules))}

	return func(c *fiber.Ctx) error {
		start := time.Now().UTC()
		path := c.Path()
		query := string(c.Request().URI().QueryString())

		err := c.Next()

		// 对匹配采样规则的路径，按间隔控制日志频率（错误始终打印）
		if err == nil {
			if interval, ok := samplingIndex[path]; ok {
				if !sampler.shouldLog(path, interval) {
					return err
				}
			}
		}

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
			safeFields := make(map[string]interface{})
			if reqBody := c.Body(); reqBody != nil {
				raw := make(map[string]interface{})
				if jsonErr := sonic.Unmarshal(reqBody, &raw); jsonErr == nil {
					// 只记录非敏感的元信息字段，不记录消息内容、工具参数等
					for _, key := range []string{"model", "stream", "max_tokens", "max_completion_tokens", "temperature", "top_p"} {
						if v, ok := raw[key]; ok {
							safeFields[key] = v
						}
					}
				}
			}
			if len(safeFields) > 0 {
				fields = append(fields, zap.Dict("request", lo.MapToSlice(safeFields, func(key string, value interface{}) zap.Field {
					return zap.Any(key, value)
				})...))
			}
		}

		// FIXME: get response body will break sse
		// reference: https://github.com/gofiber/fiber/issues/429
		// reference: https://github.com/samber/slog-fiber/issues/68
		if strings.Contains(string(c.Response().Header.ContentType()), "application/json") { // response header content-type is not text/event-stream
			safeFields := make(map[string]interface{})
			if respBody := c.Response().Body(); respBody != nil {
				raw := make(map[string]interface{})
				if jsonErr := sonic.Unmarshal(respBody, &raw); jsonErr == nil {
					// 只记录非敏感的元信息字段
					for _, key := range []string{"id", "model", "object", "error", "usage"} {
						if v, ok := raw[key]; ok {
							safeFields[key] = v
						}
					}
				}
			}
			if len(safeFields) > 0 {
				fields = append(fields, zap.Dict("response", lo.MapToSlice(safeFields, func(key string, value interface{}) zap.Field {
					return zap.Any(key, value)
				})...))
			}
		}

		if err != nil {
			fields = append([]zap.Field{zap.Error(err)}, fields...)
			logger.Error("[LogMiddleware] Error", fields...)
		} else {
			logger.Info("[LogMiddleware] Info", fields...)
		}

		return err
	}
}
