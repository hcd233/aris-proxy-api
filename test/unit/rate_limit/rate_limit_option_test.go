package rate_limit

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
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

type rateLimitTestRsp struct {
	OK bool `json:"ok"`
}

func newRateLimitRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close(); mr.Close() })
	return rdb
}

// TestWithPermissionFilter 验证 option 可正常构造且不 panic。
func TestWithPermissionFilter(t *testing.T) {
	t.Parallel()
	opt := middleware.WithPermissionFilter(enum.PermissionDemo)
	if opt == nil {
		t.Fatal("WithPermissionFilter returned nil")
	}
	_ = middleware.TokenBucketRateLimiterMiddleware(nil, "t", "", 0, 0, opt)
}

// TestTokenBucketRateLimiterMiddleware_PermissionFilterSkipsNonDemo 验证非 demo 权限
// 在启用 permissionFilter 时被零开销放行（不碰 Redis，不设置限流响应头）。
func TestTokenBucketRateLimiterMiddleware_PermissionFilterSkipsNonDemo(t *testing.T) {
	t.Parallel()
	rdb := newRateLimitRedis(t)

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyPermission, enum.PermissionUser)
		next(ctx)
	})

	handlerCalled := false
	huma.Register(api, huma.Operation{
		OperationID: "skipNonDemo",
		Method:      http.MethodPost,
		Path:        "/skip",
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(rdb, "skipSvc", "", time.Minute, 0, middleware.WithPermissionFilter(enum.PermissionDemo)),
		},
	}, func(_ context.Context, _ *struct{}) (*rateLimitTestRsp, error) {
		handlerCalled = true
		return &rateLimitTestRsp{OK: true}, nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/skip", http.NoBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !handlerCalled {
		t.Fatal("handler should be called for non-demo permission")
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d or %d", resp.StatusCode, http.StatusOK, http.StatusNoContent)
	}
	if got := resp.Header.Get(constant.HTTPHeaderXRateLimitLimit); got != "" {
		t.Fatalf("X-RateLimit-Limit = %q, want empty (skipped request must not touch Redis)", got)
	}
	if got := resp.Header.Get(constant.HTTPHeaderXRateLimitRemaining); got != "" {
		t.Fatalf("X-RateLimit-Remaining = %q, want empty (skipped request must not touch Redis)", got)
	}
}

// TestTokenBucketRateLimiterMiddleware_PermissionFilterAppliesToDemo 验证 demo 权限
// 在启用 permissionFilter 时正常执行限流逻辑（容量为 0 时拒绝）。
func TestTokenBucketRateLimiterMiddleware_PermissionFilterAppliesToDemo(t *testing.T) {
	t.Parallel()
	rdb := newRateLimitRedis(t)

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		ctx = huma.WithValue(ctx, constant.CtxKeyPermission, enum.PermissionDemo)
		next(ctx)
	})

	handlerCalled := false
	huma.Register(api, huma.Operation{
		OperationID: "rejectDemo",
		Method:      http.MethodPost,
		Path:        "/reject",
		Middlewares: huma.Middlewares{
			middleware.TokenBucketRateLimiterMiddleware(rdb, "rejectSvc", "", time.Minute, 0, middleware.WithPermissionFilter(enum.PermissionDemo)),
		},
	}, func(_ context.Context, _ *struct{}) (*rateLimitTestRsp, error) {
		handlerCalled = true
		return &rateLimitTestRsp{OK: true}, nil
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/reject", http.NoBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if handlerCalled {
		t.Fatal("handler should NOT be called for rate-limited demo permission")
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusTooManyRequests)
	}
}
