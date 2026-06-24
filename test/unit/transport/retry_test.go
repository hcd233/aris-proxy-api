// Package transport_test 验证上游请求重试/退避机制的核心逻辑。
//
// 覆盖范围：
//   - IsRetryableError 对 5xx/网络错误/4xx/其他错误的判定
//   - CalculateBackoff 的指数增长、max cap、jitter 范围
//   - SendUpstreamWithRetry 的重试成功、耗尽、不重试、context 取消
package transport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

func TestIsRetryableError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"522 可重试", &model.UpstreamError{StatusCode: 522, Body: "timeout"}, true},
		{"502 可重试", &model.UpstreamError{StatusCode: 502}, true},
		{"503 可重试", &model.UpstreamError{StatusCode: 503}, true},
		{"504 可重试", &model.UpstreamError{StatusCode: 504}, true},
		{"500 可重试", &model.UpstreamError{StatusCode: 500}, true},
		{"404 不可重试", &model.UpstreamError{StatusCode: 404}, false},
		{"401 不可重试", &model.UpstreamError{StatusCode: 401}, false},
		{"429 不可重试", &model.UpstreamError{StatusCode: 429}, false},
		{"400 不可重试", &model.UpstreamError{StatusCode: 400}, false},
		{"网络错误可重试", &model.UpstreamConnectionError{Cause: ierr.New(ierr.ErrProxyRequest, "connection refused")}, true},
		{"ierr 不可重试", ierr.New(ierr.ErrProxyRequest, "create request"), false},
		{"nil 不可重试", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := transport.IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	t.Parallel()
	initial := 100 * time.Millisecond
	maxBackoff := 1 * time.Second
	jitterFactor := 0.3

	t.Run("指数增长", func(t *testing.T) {
		t.Parallel()
		a0 := transport.CalculateBackoff(0, initial, maxBackoff, 0)
		a1 := transport.CalculateBackoff(1, initial, maxBackoff, 0)
		a2 := transport.CalculateBackoff(2, initial, maxBackoff, 0)
		if a0 >= a1 || a1 >= a2 {
			t.Errorf("backoff should grow exponentially: a0=%v a1=%v a2=%v", a0, a1, a2)
		}
	})

	t.Run("max cap", func(t *testing.T) {
		t.Parallel()
		got := transport.CalculateBackoff(20, initial, maxBackoff, 0)
		if got != maxBackoff {
			t.Errorf("backoff should be capped at max: got=%v want=%v", got, maxBackoff)
		}
	})

	t.Run("零 jitter 等于 base", func(t *testing.T) {
		t.Parallel()
		got := transport.CalculateBackoff(1, initial, maxBackoff, 0)
		want := initial * 2
		if got != want {
			t.Errorf("zero jitter: got=%v want=%v", got, want)
		}
	})

	t.Run("jitter 范围", func(t *testing.T) {
		t.Parallel()
		base := initial * 2
		lo := time.Duration(float64(base) * (1 - jitterFactor))
		hi := time.Duration(float64(base) * (1 + jitterFactor))
		for i := 0; i < 100; i++ {
			got := transport.CalculateBackoff(1, initial, maxBackoff, jitterFactor)
			if got < lo || got > hi {
				t.Errorf("backoff out of jitter range: got=%v lo=%v hi=%v", got, lo, hi)
			}
		}
	})
}

func TestSendUpstreamWithRetry(t *testing.T) { //nolint:paralleltest // modifies global config
	// 设置极小退避参数避免测试缓慢
	originalMax := config.UpstreamRetryMaxAttempts
	originalInitial := config.UpstreamRetryInitialBackoff
	originalMaxBackoff := config.UpstreamRetryMaxBackoff
	originalJitter := config.UpstreamRetryJitterFactor
	t.Cleanup(func() {
		config.UpstreamRetryMaxAttempts = originalMax
		config.UpstreamRetryInitialBackoff = originalInitial
		config.UpstreamRetryMaxBackoff = originalMaxBackoff
		config.UpstreamRetryJitterFactor = originalJitter
	})
	config.UpstreamRetryMaxAttempts = 2
	config.UpstreamRetryInitialBackoff = 1 * time.Millisecond
	config.UpstreamRetryMaxBackoff = 5 * time.Millisecond
	config.UpstreamRetryJitterFactor = 0

	makeOKResp := func() *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
		}
	}

	t.Run("首次成功", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return makeOKResp(), nil
		}
		resp, err := transport.SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp.Body.Close()
		if got := calls.Load(); got != 1 {
			t.Errorf("expected 1 call, got %d", got)
		}
	})

	t.Run("522 后重试成功", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			n := calls.Add(1)
			if n == 1 {
				return nil, &model.UpstreamError{StatusCode: 522, Body: "timeout"}
			}
			return makeOKResp(), nil
		}
		resp, err := transport.SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp.Body.Close()
		if got := calls.Load(); got != 2 {
			t.Errorf("expected 2 calls, got %d", got)
		}
	})

	t.Run("522 重试耗尽", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return nil, &model.UpstreamError{StatusCode: 522, Body: "timeout"}
		}
		_, err := transport.SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn) //nolint:bodyclose // no response body on error
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var ue *model.UpstreamError
		if !errors.As(err, &ue) || ue.StatusCode != 522 {
			t.Errorf("expected UpstreamError 522, got %v", err)
		}
		if got := calls.Load(); got != 3 {
			t.Errorf("expected 3 calls (1+2 retries), got %d", got)
		}
	})

	t.Run("4xx 不重试", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return nil, &model.UpstreamError{StatusCode: 404, Body: "not found"}
		}
		_, err := transport.SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn) //nolint:bodyclose // no response body on error
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("expected 1 call (no retry), got %d", got)
		}
	})

	t.Run("网络错误后重试成功", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			n := calls.Add(1)
			if n == 1 {
				return nil, &model.UpstreamConnectionError{Cause: ierr.New(ierr.ErrProxyRequest, "connection refused")}
			}
			return makeOKResp(), nil
		}
		resp, err := transport.SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = resp.Body.Close()
		if got := calls.Load(); got != 2 {
			t.Errorf("expected 2 calls, got %d", got)
		}
	})

	t.Run("context 取消", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return nil, &model.UpstreamError{StatusCode: 522, Body: "timeout"}
		}
		// 先取消 context，再发送请求；首次 sendFn 返回 522 后进入退避 select，
		// ctx.Done() 已就绪，必然先于 timer.C 触发
		cancel()
		_, err := transport.SendUpstreamWithRetry(ctx, "TestProxy", sendFn) //nolint:bodyclose // no response body on error
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("expected 1 call before context cancel detected, got %d", got)
		}
	})
}
