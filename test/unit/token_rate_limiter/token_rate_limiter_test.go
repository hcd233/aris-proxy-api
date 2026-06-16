package token_rate_limiter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ratelimit"
	"github.com/hcd233/aris-proxy-api/internal/dto/anthropic"
	"github.com/hcd233/aris-proxy-api/internal/dto/openai"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
)

type testRsp struct {
	OK bool `json:"ok"`
}

func newRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close(); mr.Close() })
	return rdb
}

func TestOpenAICompletionUsage_InputOutputTokens(t *testing.T) {
	t.Parallel()
	u := &openai.OpenAICompletionUsage{PromptTokens: 10, CompletionTokens: 5}
	if got := u.InputOutputTokens(); got != 15 {
		t.Fatalf("InputOutputTokens = %d, want 15", got)
	}
}

func TestResponseUsage_InputOutputTokens(t *testing.T) {
	t.Parallel()
	u := &openai.ResponseUsage{InputTokens: 20, OutputTokens: 8}
	if got := u.InputOutputTokens(); got != 28 {
		t.Fatalf("InputOutputTokens = %d, want 28", got)
	}
}

func TestAnthropicUsage_InputOutputTokens(t *testing.T) {
	t.Parallel()
	u := &anthropic.AnthropicUsage{InputTokens: 7, OutputTokens: 3}
	if got := u.InputOutputTokens(); got != 10 {
		t.Fatalf("InputOutputTokens = %d, want 10", got)
	}
}

func TestTokenBucketTokenRateLimiterMiddleware_AllowsRequest(t *testing.T) {
	t.Parallel()
	rdb := newRedis(t)

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, uint(1))
		next(ctx)
	})

	huma.Register(api, huma.Operation{
		OperationID: "testAllow",
		Method:      http.MethodPost,
		Path:        "/allow",
		Middlewares: huma.Middlewares{
			middleware.TokenBucketTokenRateLimiterMiddleware(rdb, "allowSvc", constant.CtxKeyAPIKeyID, time.Minute, 100),
		},
	}, func(_ context.Context, _ *struct{}) (*testRsp, error) {
		return &testRsp{OK: true}, nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/allow", http.NoBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d or %d", resp.StatusCode, http.StatusOK, http.StatusNoContent)
	}
	if resp.Header.Get(constant.HTTPHeaderXRateLimitLimit) != "100" {
		t.Fatalf("X-RateLimit-Limit = %q, want 100", resp.Header.Get(constant.HTTPHeaderXRateLimitLimit))
	}
	if resp.Header.Get(constant.HTTPHeaderXRateLimitRemaining) != "100" {
		t.Fatalf("X-RateLimit-Remaining = %q, want 100", resp.Header.Get(constant.HTTPHeaderXRateLimitRemaining))
	}
}

func TestTokenBucketTokenRateLimiterMiddleware_RejectsDepletedBucket(t *testing.T) {
	t.Parallel()
	rdb := newRedis(t)

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, uint(1))
		next(ctx)
	})

	huma.Register(api, huma.Operation{
		OperationID: "testReject",
		Method:      http.MethodPost,
		Path:        "/reject",
		Middlewares: huma.Middlewares{
			middleware.TokenBucketTokenRateLimiterMiddleware(rdb, "rejectSvc", constant.CtxKeyAPIKeyID, time.Minute, 0),
		},
	}, func(_ context.Context, _ *struct{}) (*testRsp, error) {
		t.Fatal("handler should not be called when rate limited")
		return nil, nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/reject", http.NoBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusTooManyRequests)
	}
}

func TestTokenUsageReporter_DeductsTokens(t *testing.T) {
	t.Parallel()
	rdb := newRedis(t)

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, uint(1))
		next(ctx)
	})

	huma.Register(api, huma.Operation{
		OperationID: "testDeduct",
		Method:      http.MethodPost,
		Path:        "/deduct",
		Middlewares: huma.Middlewares{
			middleware.TokenBucketTokenRateLimiterMiddleware(rdb, "deductSvc", constant.CtxKeyAPIKeyID, time.Minute, 100),
		},
	}, func(ctx context.Context, _ *struct{}) (*testRsp, error) {
		reporter, ok := ctx.Value(constant.CtxKeyTokenUsageReporter).(ratelimit.TokenUsageReporter)
		if ok && reporter != nil {
			reporter.Report(ctx, 30)
		}
		return &testRsp{OK: true}, nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/deduct", http.NoBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d or %d", resp.StatusCode, http.StatusOK, http.StatusNoContent)
	}

	remaining, err := rdb.HGet(context.Background(), "tb:deductSvc:apiKeyID:1", "tokens").Result()
	if err != nil {
		t.Fatalf("hget failed: %v", err)
	}
	if remaining != "70" {
		t.Fatalf("remaining = %s, want 70", remaining)
	}
}
