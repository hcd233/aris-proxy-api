package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
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

// sensitiveHeaders 需要掩码的敏感头列表
var sensitiveHeaders = []string{
	constant.HTTPHeaderAuthorization,
	constant.HTTPHeaderAPIKey,
	constant.HTTPHeaderProxyAuthorization,
	constant.HTTPHeaderCookie,
	constant.HTTPHeaderSetCookie,
}

// isSensitiveHeader 判断是否为需要掩码的敏感请求头
func isSensitiveHeader(key string) bool {
	return lo.ContainsBy(sensitiveHeaders, func(h string) bool { return strings.EqualFold(key, h) })
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

func shouldSuppressLog(samplingIndex map[string]time.Duration, sampler *logSampler, path string, _ error) bool {
	interval, ok := samplingIndex[path]
	if !ok {
		return false
	}
	return !sampler.shouldLog(path, interval)
}

func buildRequestHeadersFields(c fiber.Ctx) []zap.Field {
	reqHeaders := make(map[string]any)
	for k, v := range c.Request().Header.All() {
		key := string(k)
		value := string(v)
		if isSensitiveHeader(key) {
			value = constant.MaskSecretPlaceholder
		}
		reqHeaders[key] = value
	}
	return []zap.Field{zap.Dict("request-headers", lo.MapToSlice(reqHeaders, func(key string, value any) zap.Field {
		return zap.Any(key, value)
	})...)}
}

func buildRequestBodyFields(c fiber.Ctx) []zap.Field {
	if !strings.Contains(string(c.Request().Header.ContentType()), constant.HTTPContentTypeJSON) {
		return nil
	}
	body := c.Body()
	if len(body) == 0 {
		return nil
	}
	if len(body) > constant.LogMiddlewareBodyMaxBytes {
		return []zap.Field{zap.String("request", bodyPreview(body))}
	}
	request := make(map[string]any)
	if jsonErr := sonic.Unmarshal(body, &request); jsonErr != nil {
		return []zap.Field{zap.String("request", bodyPreview(body))}
	}
	return []zap.Field{zap.Dict("request", lo.MapToSlice(request, func(key string, value any) zap.Field {
		return zap.Any(key, value)
	})...)}
}

func buildResponseBodyFields(c fiber.Ctx) []zap.Field {
	contentType := string(c.Response().Header.ContentType())
	if !strings.Contains(contentType, constant.HTTPContentTypeJSON) &&
		!strings.Contains(contentType, constant.HTTPContentTypeProblemJSON) {
		return nil
	}
	body := c.Response().Body()
	if len(body) == 0 {
		return nil
	}
	if len(body) > constant.LogMiddlewareBodyMaxBytes {
		return []zap.Field{zap.String("response", bodyPreview(body))}
	}
	response := make(map[string]any)
	if jsonErr := sonic.Unmarshal(body, &response); jsonErr != nil {
		return []zap.Field{zap.String("response", bodyPreview(body))}
	}
	return []zap.Field{zap.Dict("response", lo.MapToSlice(response, func(key string, value any) zap.Field {
		return zap.Any(key, value)
	})...)}
}

// bodyPreview 把超长或不可解析的 body 截断成字符串字段，避免整段内容进入日志。
func bodyPreview(body []byte) string {
	return util.TruncateFieldValue(string(body), constant.LogMiddlewareBodyMaxBytes)
}

// ShouldLogBody 判断是否记录请求/响应体：只在失败现场记录。
//
// 成功响应体是日志体积与 CPU 的主要来源（如 session/message/list 的 310KB 响应
// 需完整 JSON 解析后再编码一次），而排障依赖的是失败请求的入参与出参，故成功路径不记录。
//
// 导出供回归测试断言策略（同 repository.FormatBigIntArray 的测试导出约定）。
func ShouldLogBody(err error, status int) bool {
	return err != nil || status >= fiber.StatusBadRequest
}

func buildResponseHeadersFields(c fiber.Ctx) []zap.Field {
	respHeaders := make(map[string]any)
	for k, v := range c.Response().Header.All() {
		key := string(k)
		value := string(v)
		if isSensitiveHeader(key) {
			value = constant.MaskSecretPlaceholder
		}
		respHeaders[key] = value
	}
	return []zap.Field{zap.Dict("response-headers", lo.MapToSlice(respHeaders, func(key string, value any) zap.Field {
		return zap.Any(key, value)
	})...)}
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

	return func(c fiber.Ctx) error {
		start := time.Now().UTC()
		// c.Path() 零拷贝引用 fasthttp 可复用缓冲区，此处作为采样器 map 的长期 key，
		// 并被后续异步上报引用，必须拷贝。
		path := strings.Clone(c.Path())
		query := string(c.Request().URI().QueryString())

		err := c.Next()

		if shouldSuppressLog(samplingIndex, sampler, path, err) {
			return err
		}

		log := logger.WithFCtx(c)
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

		fields = append(fields, buildRequestHeadersFields(c)...)
		fields = append(fields, buildResponseHeadersFields(c)...)
		if ShouldLogBody(err, c.Response().StatusCode()) {
			fields = append(fields, buildRequestBodyFields(c)...)
			fields = append(fields, buildResponseBodyFields(c)...)
		}

		if err != nil {
			fields = append([]zap.Field{zap.Error(err)}, fields...)
			log.Error("[LogMiddleware] Error", fields...)
		} else {
			log.Info("[LogMiddleware] Info", fields...)
		}

		return err
	}
}
