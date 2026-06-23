# 上游请求重试/退避机制 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 对 LLM 代理上游请求的 5xx 和网络层错误实现指数退避重试，覆盖 OpenAI 和 Anthropic 两个 provider。

**Architecture:** 在 `internal/infrastructure/transport/` 包内新增重试模块，将 `doUpstreamRequest`（OpenAI）和 `sendRequest`（Anthropic）的单次发送逻辑拆为独立的单次发送函数，外层包裹公共的 `SendUpstreamWithRetry` 重试循环。所有 Forward 方法签名不变，自动受益。重试参数通过 viper 配置项控制。

**Tech Stack:** Go 1.25.1, spf13/viper, zap, math/rand/v2, standard library testing

---

## 文件结构

| 文件 | 职责 | 操作 |
|------|------|------|
| `internal/config/config.go` | 新增 4 个重试配置全局变量 + viper 默认值 | 修改 |
| `internal/infrastructure/transport/retry.go` | 重试模块：`IsRetryableError`、`CalculateBackoff`、`SendUpstreamWithRetry` | 新增 |
| `internal/infrastructure/transport/openai.go` | 拆分 `doUpstreamRequest` 为薄包装 + `sendUpstreamRequestOnce` | 修改 |
| `internal/infrastructure/transport/anthropic.go` | 拆分 `sendRequest` 为薄包装 + `sendRequestOnce` | 修改 |
| `test/unit/transport/retry_test.go` | 重试模块单元测试 | 新增 |

> **注意**：`IsRetryableError`、`CalculateBackoff`、`SendUpstreamWithRetry` 使用导出名（大写开头），因为测试文件在 `test/unit/transport/` 属于外部测试包，只能访问导出符号。这些函数在 `internal/` 包内，不会泄漏到模块外部。

---

### Task 1: 新增重试配置项

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: 添加全局变量声明**

在 `internal/config/config.go` 的 `var` 块末尾（`CronThinkExtractEnabled` 之后，第 198 行之前）添加：

```go
	// UpstreamRetryMaxAttempts int 上游请求重试最大次数（不含首次请求）
	//	@update 2026-06-23 10:00:00
	UpstreamRetryMaxAttempts int

	// UpstreamRetryInitialBackoff time.Duration 上游请求重试初始退避时间
	//	@update 2026-06-23 10:00:00
	UpstreamRetryInitialBackoff time.Duration

	// UpstreamRetryMaxBackoff time.Duration 上游请求重试最大退避时间
	//	@update 2026-06-23 10:00:00
	UpstreamRetryMaxBackoff time.Duration

	// UpstreamRetryJitterFactor float64 上游请求重试退避抖动因子 (0~1)
	//	@update 2026-06-23 10:00:00
	UpstreamRetryJitterFactor float64
```

- [ ] **Step 2: 添加 viper 默认值**

在 `initEnvironment()` 函数中，`config.AutomaticEnv()` 之前（第 253 行之前）添加：

```go
	config.SetDefault("upstream.retry.max_attempts", 2)
	config.SetDefault("upstream.retry.initial_backoff", 500*time.Millisecond)
	config.SetDefault("upstream.retry.max_backoff", 2*time.Second)
	config.SetDefault("upstream.retry.jitter_factor", 0.3)
```

- [ ] **Step 3: 添加配置读取赋值**

在 `initEnvironment()` 函数末尾（`TrustedProxies` 赋值逻辑之后，第 334 行之前）添加：

```go
	UpstreamRetryMaxAttempts = config.GetInt("upstream.retry.max_attempts")
	UpstreamRetryInitialBackoff = config.GetDuration("upstream.retry.initial_backoff")
	UpstreamRetryMaxBackoff = config.GetDuration("upstream.retry.max_backoff")
	UpstreamRetryJitterFactor = config.GetFloat64("upstream.retry.jitter_factor")
```

- [ ] **Step 4: 验证编译**

Run: `go build ./internal/config/`
Expected: 编译成功，无错误

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add upstream retry config items"
```

---

### Task 2: 创建 IsRetryableError 函数（TDD）

**Files:**
- Create: `internal/infrastructure/transport/retry.go`
- Test: `test/unit/transport/retry_test.go`

- [ ] **Step 1: 创建 retry.go 骨架和 IsRetryableError**

创建 `internal/infrastructure/transport/retry.go`：

```go
package transport

import (
	"errors"
	"net/http"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// IsRetryableError 判断上游错误是否可重试
//
// 可重试：UpstreamConnectionError（网络层错误）、UpstreamError 且 StatusCode >= 500（5xx 瞬时错误）
// 不可重试：UpstreamError 且 StatusCode < 500（4xx 永久错误）、其他错误（请求构建失败等）
//
//	@param err error 错误
//	@return bool 是否可重试
//	@author centonhuang
//	@update 2026-06-23 10:00:00
func IsRetryableError(err error) bool {
	var connErr *model.UpstreamConnectionError
	if errors.As(err, &connErr) {
		return true
	}
	var upstreamErr *model.UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.StatusCode >= 500
	}
	return false
}
```

- [ ] **Step 2: 创建测试文件和 IsRetryableError 测试**

创建 `test/unit/transport/retry_test.go`：

```go
// Package transport 验证上游请求重试/退避机制的核心逻辑。
//
// 覆盖范围：
//   - IsRetryableError 对 5xx/网络错误/4xx/其他错误的判定
//   - CalculateBackoff 的指数增长、max cap、jitter 范围
//   - SendUpstreamWithRetry 的重试成功、耗尽、不重试、context 取消
package transport

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
)

func TestIsRetryableError(t *testing.T) {
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
		{"网络错误可重试", &model.UpstreamConnectionError{Cause: errors.New("connection refused")}, true},
		{"ierr 不可重试", ierr.New(ierr.ErrProxyRequest, "create request"), false},
		{"nil 不可重试", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 3: 运行测试验证通过**

Run: `go test -v -count=1 -run TestIsRetryableError ./test/unit/transport/`
Expected: PASS（所有子用例通过）

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/transport/retry.go test/unit/transport/retry_test.go
git commit -m "feat: add IsRetryableError for upstream retry judgment"
```

---

### Task 3: 添加 CalculateBackoff 函数（TDD）

**Files:**
- Modify: `internal/infrastructure/transport/retry.go`
- Modify: `test/unit/transport/retry_test.go`

- [ ] **Step 1: 在 retry.go 添加 CalculateBackoff**

在 `retry.go` 的 import 块中添加 `"math/rand/v2"` 和 `"time"`，然后在 `IsRetryableError` 之后添加：

```go
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
func CalculateBackoff(attempt int, initial, max time.Duration, jitterFactor float64) time.Duration {
	base := initial * time.Duration(1<<uint(attempt))
	if base > max || base <= 0 {
		base = max
	}
	jitter := float64(base) * jitterFactor * (2*rand.Float64() - 1)
	return base + time.Duration(jitter)
}
```

> 注意：`1<<uint(attempt)` 防止 attempt 为负数时的未定义行为；`base <= 0` 检查防止溢出后回绕为负数。

- [ ] **Step 2: 在 retry_test.go 添加 CalculateBackoff 测试**

在 `TestIsRetryableError` 之后添加：

```go
func TestCalculateBackoff(t *testing.T) {
	initial := 100 * time.Millisecond
	maxBackoff := 1 * time.Second
	jitterFactor := 0.3

	t.Run("指数增长", func(t *testing.T) {
		a0 := CalculateBackoff(0, initial, maxBackoff, 0)
		a1 := CalculateBackoff(1, initial, maxBackoff, 0)
		a2 := CalculateBackoff(2, initial, maxBackoff, 0)
		if a0 >= a1 || a1 >= a2 {
			t.Errorf("backoff should grow exponentially: a0=%v a1=%v a2=%v", a0, a1, a2)
		}
	})

	t.Run("max cap", func(t *testing.T) {
		got := CalculateBackoff(20, initial, maxBackoff, 0)
		if got != maxBackoff {
			t.Errorf("backoff should be capped at max: got=%v want=%v", got, maxBackoff)
		}
	})

	t.Run("零 jitter 等于 base", func(t *testing.T) {
		got := CalculateBackoff(1, initial, maxBackoff, 0)
		want := initial * 2
		if got != want {
			t.Errorf("zero jitter: got=%v want=%v", got, want)
		}
	})

	t.Run("jitter 范围", func(t *testing.T) {
		base := initial * 2
		lo := time.Duration(float64(base) * (1 - jitterFactor))
		hi := time.Duration(float64(base) * (1 + jitterFactor))
		for i := 0; i < 100; i++ {
			got := CalculateBackoff(1, initial, maxBackoff, jitterFactor)
			if got < lo || got > hi {
				t.Errorf("backoff out of jitter range: got=%v lo=%v hi=%v", got, lo, hi)
			}
		}
	})
}
```

- [ ] **Step 3: 运行测试验证通过**

Run: `go test -v -count=1 -run TestCalculateBackoff ./test/unit/transport/`
Expected: PASS（所有子用例通过）

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/transport/retry.go test/unit/transport/retry_test.go
git commit -m "feat: add CalculateBackoff with exponential backoff and jitter"
```

---

### Task 4: 添加 SendUpstreamWithRetry 函数（TDD）

**Files:**
- Modify: `internal/infrastructure/transport/retry.go`
- Modify: `test/unit/transport/retry_test.go`

- [ ] **Step 1: 在 retry.go 添加 SendUpstreamWithRetry**

在 `retry.go` 的 import 块中添加 `"context"`、`"github.com/hcd233/aris-proxy-api/internal/config"`、`"github.com/hcd233/aris-proxy-api/internal/logger"`、`"go.uber.org/zap"`，然后在 `CalculateBackoff` 之后添加：

```go
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
			log.Warn("["+module+"] Retry cancelled by context",
				zap.Int("attempt", attempt+1),
				zap.Error(ctx.Err()),
			)
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, lastErr
}
```

- [ ] **Step 2: 在 retry_test.go 添加 SendUpstreamWithRetry 测试**

在测试文件的 import 块中已有 `config` 导入。在 `TestCalculateBackoff` 之后添加：

```go
func TestSendUpstreamWithRetry(t *testing.T) {
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
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return makeOKResp(), nil
		}
		_, err := SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("expected 1 call, got %d", got)
		}
	})

	t.Run("522 后重试成功", func(t *testing.T) {
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			n := calls.Add(1)
			if n == 1 {
				return nil, &model.UpstreamError{StatusCode: 522, Body: "timeout"}
			}
			return makeOKResp(), nil
		}
		_, err := SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := calls.Load(); got != 2 {
			t.Errorf("expected 2 calls, got %d", got)
		}
	})

	t.Run("522 重试耗尽", func(t *testing.T) {
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return nil, &model.UpstreamError{StatusCode: 522, Body: "timeout"}
		}
		_, err := SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
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
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return nil, &model.UpstreamError{StatusCode: 404, Body: "not found"}
		}
		_, err := SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("expected 1 call (no retry), got %d", got)
		}
	})

	t.Run("网络错误后重试成功", func(t *testing.T) {
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			n := calls.Add(1)
			if n == 1 {
				return nil, &model.UpstreamConnectionError{Cause: errors.New("connection refused")}
			}
			return makeOKResp(), nil
		}
		_, err := SendUpstreamWithRetry(context.Background(), "TestProxy", sendFn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := calls.Load(); got != 2 {
			t.Errorf("expected 2 calls, got %d", got)
		}
	})

	t.Run("context 取消", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		var calls atomic.Int32
		sendFn := func() (*http.Response, error) {
			calls.Add(1)
			return nil, &model.UpstreamError{StatusCode: 522, Body: "timeout"}
		}
		// 先取消 context，再发送请求；首次 sendFn 返回 522 后进入退避 select，
		// ctx.Done() 已就绪，必然先于 timer.C 触发
		cancel()
		_, err := SendUpstreamWithRetry(ctx, "TestProxy", sendFn)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("expected 1 call before context cancel detected, got %d", got)
		}
	})
}
```

> 注意："context 取消"用例中，先 `cancel()` 再调用 `SendUpstreamWithRetry`。首次 sendFn 返回 522 后进入退避 `select`，此时 `ctx.Done()` 已就绪，必然先于 `timer.C` 触发，避免竞态。

- [ ] **Step 3: 运行测试验证通过**

Run: `go test -v -count=1 -run TestSendUpstreamWithRetry ./test/unit/transport/`
Expected: PASS（所有子用例通过）

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/transport/retry.go test/unit/transport/retry_test.go
git commit -m "feat: add SendUpstreamWithRetry with exponential backoff retry"
```

---

### Task 5: 重构 openai.go 接入重试

**Files:**
- Modify: `internal/infrastructure/transport/openai.go:119-168`

- [ ] **Step 1: 将 doUpstreamRequest 重命名为 sendUpstreamRequestOnce 并移除错误日志**

在 `internal/infrastructure/transport/openai.go` 中，将第 119-168 行的 `doUpstreamRequest` 方法替换为以下两个方法：

```go
// doUpstreamRequest 构建并发送上游 HTTP 请求，对可重试错误自动重试
func (p *openAIProxy) doUpstreamRequest(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, pathSuffix string) (*http.Response, error) {
	sendFn := func() (*http.Response, error) {
		return p.sendUpstreamRequestOnce(ctx, ep, body, pathSuffix)
	}
	return SendUpstreamWithRetry(ctx, "OpenAIProxy", sendFn)
}

// sendUpstreamRequestOnce 执行单次上游 HTTP 请求发送（不含重试逻辑）
func (p *openAIProxy) sendUpstreamRequestOnce(ctx context.Context, ep vo.UpstreamEndpoint, body []byte, pathSuffix string) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + pathSuffix

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[OpenAIProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create request")
	}

	// 透传客户端请求头
	applyPassthroughRequestHeaders(ctx, req.Header)

	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ep.APIKey)

	log.Info("[OpenAIProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.String("upstreamAPIKey", commonutil.MaskSecret(ep.APIKey)),
		zap.Any("upstreamHeaders", util.MaskHTTPHeadersForLog(req.Header)),
		zap.Any("upstreamRequestSummary", parseUpstreamRequestSummary(body)),
	)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // read best effort on error path
		_ = resp.Body.Close()                 //nolint:errcheck // close best effort on error path
		return nil, &model.UpstreamError{
			StatusCode: resp.StatusCode,
			Headers:    capturePassthroughResponseHeaders(resp.Header),
			Body:       string(errorBody),
		}
	}

	storePassthroughResponseHeaders(ctx, resp.Header)

	return resp, nil
}
```

> 关键变更：`sendUpstreamRequestOnce` 内部移除了 `log.Error("[OpenAIProxy] Send http request error", ...)` 和 `log.Error("[OpenAIProxy] Upstream returned non-200 status", ...)` 两处 Error 日志。错误日志改由 `SendUpstreamWithRetry` 在重试耗尽时通过 Warn/返回错误统一处理。Info 日志保留（每次尝试都记录）。请求构建失败的 Error 日志保留（不可重试错误，直接返回）。

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/infrastructure/transport/`
Expected: 编译成功，无错误

- [ ] **Step 3: 运行已有测试确认无回归**

Run: `go test -count=1 ./test/unit/llmproxy_usecase/ ./test/unit/upstream_error/`
Expected: PASS（所有已有测试通过）

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/transport/openai.go
git commit -m "refactor: split openai doUpstreamRequest into retry wrapper and single send"
```

---

### Task 6: 重构 anthropic.go 接入重试

**Files:**
- Modify: `internal/infrastructure/transport/anthropic.go:134-185`

- [ ] **Step 1: 将 sendRequest 重命名为 sendRequestOnce 并添加薄包装**

在 `internal/infrastructure/transport/anthropic.go` 中，将第 134-185 行的 `sendRequest` 方法替换为以下两个方法：

```go
// sendRequest 构建并发送 Anthropic 协议的上游请求，对可重试错误自动重试
func (p *anthropicProxy) sendRequest(ctx context.Context, ep vo.UpstreamEndpoint, path string, body []byte) (*http.Response, error) {
	sendFn := func() (*http.Response, error) {
		return p.sendRequestOnce(ctx, ep, path, body)
	}
	return SendUpstreamWithRetry(ctx, "AnthropicProxy", sendFn)
}

// sendRequestOnce 执行单次 Anthropic 协议上游请求发送（不含重试逻辑）
func (p *anthropicProxy) sendRequestOnce(ctx context.Context, ep vo.UpstreamEndpoint, path string, body []byte) (*http.Response, error) {
	log := logger.WithCtx(ctx)

	upstreamURL := strings.TrimRight(ep.BaseURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		log.Error("[AnthropicProxy] New request error", zap.String("upstreamURL", upstreamURL), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrProxyRequest, err, "create request")
	}

	// 透传客户端请求头
	applyPassthroughRequestHeaders(ctx, req.Header)

	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+ep.APIKey)
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAPIKey, ep.APIKey)
	req.Header.Set(constant.HTTPHeaderAnthropicVersion, constant.AnthropicAPIVersion)

	log.Info("[AnthropicProxy] Send upstream request", zap.String("upstreamURL", upstreamURL),
		zap.String("upstreamModel", ep.Model),
		zap.String("upstreamAPIKey", commonutil.MaskSecret(ep.APIKey)),
		zap.Any("upstreamHeaders", util.MaskHTTPHeadersForLog(req.Header)),
		zap.Any("upstreamRequestSummary", parseUpstreamRequestSummary(body)),
	)

	resp, err := httpclient.GetHTTPClient().Do(req)
	if err != nil {
		return nil, &model.UpstreamConnectionError{Cause: err}
	}

	if resp.StatusCode != http.StatusOK {
		errorBody, _ := io.ReadAll(resp.Body) //nolint:errcheck // read best effort on error path
		_ = resp.Body.Close()                 //nolint:errcheck // close best effort on error path
		return nil, &model.UpstreamError{
			StatusCode: resp.StatusCode,
			Headers:    capturePassthroughResponseHeaders(resp.Header),
			Body:       string(errorBody),
		}
	}

	storePassthroughResponseHeaders(ctx, resp.Header)

	return resp, nil
}
```

> 关键变更与 openai.go 相同：`sendRequestOnce` 移除了网络错误和非 200 的 Error 日志，改由 `SendUpstreamWithRetry` 统一处理。请求构建失败的 Error 日志保留。

- [ ] **Step 2: 验证编译**

Run: `go build ./internal/infrastructure/transport/`
Expected: 编译成功，无错误

- [ ] **Step 3: 运行已有测试确认无回归**

Run: `go test -count=1 ./test/unit/llmproxy_usecase/ ./test/unit/upstream_error/`
Expected: PASS（所有已有测试通过）

- [ ] **Step 4: Commit**

```bash
git add internal/infrastructure/transport/anthropic.go
git commit -m "refactor: split anthropic sendRequest into retry wrapper and single send"
```

---

### Task 7: 全量验证

**Files:**
- 无修改

- [ ] **Step 1: 运行 transport 单元测试**

Run: `go test -v -count=1 ./test/unit/transport/`
Expected: PASS（IsRetryableError、CalculateBackoff、SendUpstreamWithRetry 全部通过）

- [ ] **Step 2: 运行全量测试**

Run: `make test`
Expected: PASS（所有测试通过，无回归）

- [ ] **Step 3: 运行规范扫描**

Run: `make lint`
Expected: PASS（lint-conv + lint-static 两阶段通过，无 ERROR）

- [ ] **Step 4: 最终提交（如有 lint 修复）**

```bash
git add -A
git commit -m "test: verify upstream retry/backoff full suite" --allow-empty
```

> 如果 lint 和 test 全部通过无需额外提交，此步骤可跳过。
