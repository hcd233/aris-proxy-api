# Header Passthrough Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Passthrough incoming request headers (except proxy-managed ones) to upstream LLM providers.

**Architecture:** Capture headers in APIKey middleware via `huma.Context.EachHeader()`, store in context as `map[string]string`, read in transport layer and apply to upstream `*http.Request` before proxy-specific headers.

**Tech Stack:** Go, Huma v2, Fiber, `http.Request.Header`

---

### Task 1: Add context key and helpers

**Files:**
- Modify: `internal/common/constant/ctx.go`
- Modify: `internal/util/context.go`

- [ ] **Step 1: Add context key**

Edit `internal/common/constant/ctx.go`, add after `CtxKeyAPIKeyID`:

```go
// CtxKeyPassthroughHeaders 透传到上游的请求头
//	@update 2026-04-28 10:00:00
CtxKeyPassthroughHeaders enum.CtxKey = "passthroughHeaders"
```

- [ ] **Step 2: Add GetPassthroughHeaders helper**

Edit `internal/util/context.go`, add:

```go
// GetPassthroughHeaders 从上下文中获取透传的请求头
//
//	@param ctx context.Context
//	@return map[string]string
//	@author centonhuang
//	@update 2026-04-28 10:00:00
func GetPassthroughHeaders(ctx context.Context) map[string]string {
	if v := ctx.Value(constant.CtxKeyPassthroughHeaders); v != nil {
		if m, ok := v.(map[string]string); ok {
			return m
		}
	}
	return nil
}
```

- [ ] **Step 3: Update CopyContextValues to include passthrough headers**

Edit `internal/util/context.go`, add inside the function body (after `CtxKeyClient` line):

```go
dst = context.WithValue(dst, constant.CtxKeyPassthroughHeaders, src.Value(constant.CtxKeyPassthroughHeaders))
```

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: no error

---

### Task 2: Extract and store headers in middleware

**Files:**
- Modify: `internal/middleware/apikey.go`

- [ ] **Step 1: Add excluded headers set and import `http`**

Add after existing imports:

```go
"net/http"
```

Add as package-level var after import block:

```go
// passthroughExcludedHeaders 不透传到上游的请求头（小写比较）
var passthroughExcludedHeaders = map[string]struct{}{
	"content-type":        {},
	"content-length":      {},
	"authorization":       {},
	"x-api-key":           {},
	"anthropic-version":   {},
	"host":                {},
	"connection":          {},
	"transfer-encoding":   {},
	"upgrade":             {},
	"proxy-authorization": {},
	"proxy-authenticate":  {},
	"te":                  {},
	"trailer":             {},
	"x-trace-id":          {},
}
```

- [ ] **Step 2: Add header extraction after auth**

Edit `internal/middleware/apikey.go`, add before `next(ctx)` (after the existing `huma.WithValue` lines):

```go
		// 透传请求头到上游（排除代理自身管理的头）
		passthroughHeaders := make(map[string]string, 8)
		ctx.EachHeader(func(name, value string) {
			if _, excluded := passthroughExcludedHeaders[http.CanonicalHeaderKey(name)]; !excluded {
				passthroughHeaders[http.CanonicalHeaderKey(name)] = value
			}
		})
		ctx = huma.WithValue(ctx, constant.CtxKeyPassthroughHeaders, passthroughHeaders)
```

Wait, `http.CanonicalHeaderKey` normalizes to `Content-Type` format (title-case with dashes). But my excluded map uses lowercase keys. Let me fix this — either normalize both to lowercase, or normalize both to canonical form.

I'll use lowercase for the comparison to be safe:

```go
		ctx.EachHeader(func(name, value string) {
			lower := strings.ToLower(name)
			if _, excluded := passthroughExcludedHeaders[lower]; !excluded {
				passthroughHeaders[http.CanonicalHeaderKey(name)] = value
			}
		})
```

And I need to import `strings` (already imported) and `net/http`.

- [ ] **Step 3: Build check**

Run: `go build ./...`
Expected: no error

---

### Task 3: Forward in OpenAI transport

**Files:**
- Modify: `internal/infrastructure/transport/openai.go`

- [ ] **Step 1: Add GetPassthroughHeaders import**

The file already imports `commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"`. I need `util` for `GetPassthroughHeaders`. Check existing imports:

```go
"github.com/hcd233/aris-proxy-api/internal/util"
```

This is already imported. Good.

- [ ] **Step 2: Apply passthrough headers in `sendRequest`**

Edit `internal/infrastructure/transport/openai.go`, in `sendRequest`, add after creating `req` (after `http.NewRequestWithContext`) and BEFORE the Content-Type/Authorization lines:

```go
	// 透传客户端请求头
	if headers := util.GetPassthroughHeaders(ctx); headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
```

The existing `req.Header.Set(constant.HTTPHeaderContentType, ...)` and `req.Header.Set(constant.HTTPHeaderAuthorization, ...)` lines follow right after, so they will override passthrough headers if there were any conflicts.

- [ ] **Step 3: Apply passthrough headers in `sendResponseRequest`**

Same change in `sendResponseRequest` at the same location (after creating `req`, before the Content-Type/Authorization lines):

```go
	// 透传客户端请求头
	if headers := util.GetPassthroughHeaders(ctx); headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
```

- [ ] **Step 4: Build check**

Run: `go build ./...`
Expected: no error

---

### Task 4: Forward in Anthropic transport

**Files:**
- Modify: `internal/infrastructure/transport/anthropic.go`

- [ ] **Step 1: Apply passthrough headers in `sendRequest`**

Edit `internal/infrastructure/transport/anthropic.go`, in `sendRequest`, add after creating `req` and BEFORE the Content-Type/x-api-key/anthropic-version lines:

```go
	// 透传客户端请求头
	if headers := util.GetPassthroughHeaders(ctx); headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
```

- [ ] **Step 2: Build check**

Run: `go build ./...`
Expected: no error

---

### Task 5: Unit test for header filtering logic

**Files:**
- Create: `test/unit/header_passthrough/header_passthrough_test.go`

- [ ] **Step 1: Create unit test file**

```go
package header_passthrough

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func TestGetPassthroughHeaders_NoHeaders(t *testing.T) {
	ctx := context.Background()
	headers := util.GetPassthroughHeaders(ctx)
	if headers != nil {
		t.Errorf("expected nil, got %v", headers)
	}
}

func TestGetPassthroughHeaders_WithHeaders(t *testing.T) {
	expected := map[string]string{
		"User-Agent":    "test-agent",
		"X-Custom":      "custom-value",
	}
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, expected)
	headers := util.GetPassthroughHeaders(ctx)
	if headers == nil {
		t.Fatal("expected non-nil headers")
	}
	if headers["User-Agent"] != "test-agent" {
		t.Errorf("expected test-agent, got %s", headers["User-Agent"])
	}
	if headers["X-Custom"] != "custom-value" {
		t.Errorf("expected custom-value, got %s", headers["X-Custom"])
	}
}

func TestGetPassthroughHeaders_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), constant.CtxKeyPassthroughHeaders, "not-a-map")
	headers := util.GetPassthroughHeaders(ctx)
	if headers != nil {
		t.Errorf("expected nil for wrong type, got %v", headers)
	}
}
```

- [ ] **Step 2: Run unit test**

Run: `go test -v -count=1 ./test/unit/header_passthrough/`
Expected: all tests PASS

---

### Task 6: E2E test for header passthrough

**Files:**
- Create: `test/e2e/header_passthrough/header_passthrough_test.go`
- Create: `test/e2e/header_passthrough/fixtures/requests/chat_completion.json`

- [ ] **Step 1: Create fixture request**

```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "user",
      "content": "Say hello in one word"
    }
  ],
  "max_tokens": 10
}
```

- [ ] **Step 2: Create E2E test**

```go
package header_passthrough

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestHeaderPassthrough(t *testing.T) {
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}

	body, err := os.ReadFile("fixtures/requests/chat_completion.json")
	if err != nil {
		t.Fatal("read fixture:", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal("create request:", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Custom-Header", "passthrough-test-value")
	req.Header.Set("X-Request-Id", "test-request-123")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("send request:", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	traceID := resp.Header.Get(constant.HTTPHeaderTraceID)
	if traceID == "" {
		t.Error("expected X-Trace-Id header in response")
	}

	t.Logf("TraceID: %s", traceID)
}

func TestHeaderPassthroughStream(t *testing.T) {
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}

	body, err := os.ReadFile("fixtures/requests/chat_completion.json")
	if err != nil {
		t.Fatal("read fixture:", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal("create request:", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Custom-Header", "passthrough-stream-test")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("send request:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	ct := resp.Header.Get("Content-Type")
	t.Logf("Content-Type: %s", ct)

	traceID := resp.Header.Get(constant.HTTPHeaderTraceID)
	if traceID == "" {
		t.Error("expected X-Trace-Id header in response")
	}
	t.Logf("TraceID: %s", traceID)
}
```

Wait, for the stream test, I need to set `stream: true` in the request body. The fixture currently doesn't have it. Let me create a separate fixture or use a different approach. Actually, let me modify the fixture to have `stream: true` or create a separate one. Let me create a separate fixture.

Actually, looking at existing E2E tests, they use different fixtures for stream vs non-stream. Let me use two fixtures.

Let me simplify - just create a `chat_completion.json` without stream (non-stream), and a `chat_completion_stream.json` with stream.

`fixtures/requests/chat_completion_stream.json`:
```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "user",
      "content": "Say hello in one word"
    }
  ],
  "max_tokens": 10,
  "stream": true
}
```

- [ ] **Step 3: Run E2E test** (requires BASE_URL and API_KEY)

Run: `BASE_URL=https://xxx API_KEY=xxx go test -v -count=1 ./test/e2e/header_passthrough/`
Expected: PASS

---

### Task 7: Final verification

- [ ] **Step 1: Run lint**

Run: `make lint-conv` (or equivalent)
Expected: no errors

- [ ] **Step 2: Run full test suite**

Run: `go test -count=1 ./...`
Expected: PASS
