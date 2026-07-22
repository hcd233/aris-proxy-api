package transport

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// IsRetryableError 判断上游错误是否可重试
//
// 可重试：
//   - UpstreamConnectionError（网络层错误）
//   - UpstreamError 且 StatusCode >= 500（5xx 瞬时错误）
//   - UpstreamError 且 StatusCode == 429（Too Many Requests，限流）
//
// 不可重试：UpstreamError 且 StatusCode 为其他 4xx（客户端错误）、其他错误（请求构建失败等）
//
//	@param err error 错误
//	@return bool 是否可重试
//	@author centonhuang
//	@update 2026-07-22 11:00:00
func IsRetryableError(err error) bool {
	var connErr *model.UpstreamConnectionError
	if errors.As(err, &connErr) {
		return true
	}
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.StatusCode >= constant.UpstreamRetryableStatusThreshold ||
			upstreamErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// CalculateBackoff 计算指数退避 + jitter 的等待时间
//
// 公式：base = min(initial * 2^attempt, max)，backoff = base * (1 + jitterFactor * (2*rand - 1))
//
//	@param attempt int 当前重试次数（从 0 开始）
//	@param initial time.Duration 初始退避时间
//	@param max time.Duration 最大退避时间
//	@param jitterFactor float64 抖动因子 (0~1)
//	@return time.Duration 退避等待时间
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func CalculateBackoff(attempt int, initial, maxBackoff time.Duration, jitterFactor float64) time.Duration {
	base := initial * time.Duration(1<<uint(attempt))
	if base > maxBackoff || base <= 0 {
		base = maxBackoff
	}
	jitter := float64(base) * jitterFactor * (2*rand.Float64() - 1) //nolint:gosec // jitter doesn't need crypto-grade randomness
	return base + time.Duration(jitter)
}

// SendUpstreamWithRetry 包装单次上游请求发送函数，对可重试错误进行指数退避重试
//
// sendFn 每次调用都应重新构建并发送请求（body 为原始字节数组，可重复读取）。
// 重试参数来自 config 全局变量。退避等待期间监听 ctx.Done()。
//
//	@param ctx context.Context 请求上下文
//	@param module string 模块名（用于日志前缀，如 "OpenAIProxy"）
//	@param sendFn func() (*http.Response, error) 单次发送闭包
//	@return *http.Response 成功响应
//	@return error 错误（不可重试错误、重试耗尽后的最后错误、或 ctx.Err()）
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func SendUpstreamWithRetry(ctx context.Context, module string, sendFn func() (*http.Response, error)) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	var lastErr error
	maxAttempts := config.UpstreamRetryMaxAttempts

	for attempt := 0; attempt <= maxAttempts; attempt++ {
		resp, err := sendFn()
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !IsRetryableError(err) {
			return nil, err
		}

		if attempt >= maxAttempts {
			break
		}

		backoff := CalculateBackoff(
			attempt,
			config.UpstreamRetryInitialBackoff,
			config.UpstreamRetryMaxBackoff,
			config.UpstreamRetryJitterFactor,
		)

		log.Warn("["+module+"] Upstream request failed, retrying",
			zap.Int("attempt", attempt+1),
			zap.Int("maxAttempts", maxAttempts+1),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Warn("["+module+"] Retry canceled by context",
				zap.Int("attempt", attempt+1),
				zap.Error(ctx.Err()),
			)
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, lastErr
}
