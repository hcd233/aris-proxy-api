// Package upstream_resilience E2E 验证容错默认配置下正常链路不受影响、熔断指标已注册。
// 触发熔断/恢复路径由 test/unit/transport/guard_integration_test.go 覆盖（本用例不制造上游故障）。
package upstream_resilience

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// e2eHTTPTimeout 单条请求总超时（与 openai_chat_completion 用例保持一致）。
const e2eHTTPTimeout = 90 * time.Second

// mustE2EEnv 返回 (baseURL, apiKey) 或 t.Skip；E2E 默认离线 skip，显式提供环境变量时才打到生产。
func mustE2EEnv(t *testing.T) (baseURL, apiKey string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey
}

// TestChatCompletionSucceedsWithDefaultGuard 回归：默认配置（熔断开启、bulkhead 开启）下正常请求成功。
func TestChatCompletionSucceedsWithDefaultGuard(t *testing.T) {
	t.Parallel()
	baseURL, apiKey := mustE2EEnv(t)

	body, err := os.ReadFile("./fixtures/requests/simple_chat.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: e2eHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, respBody)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(respBody), `"choices"`) {
		t.Fatalf("response missing choices: %s", respBody)
	}
}

// TestMetricsExposeCircuitState 验证容错指标已注册（GET /metrics 公开端点）。
func TestMetricsExposeCircuitState(t *testing.T) {
	t.Parallel()
	baseURL, _ := mustE2EEnv(t)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/metrics", http.NoBody)
	if err != nil {
		t.Fatalf("create metrics request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	for _, want := range []string{"upstream_circuit_state", "upstream_circuit_open_total", "upstream_bulkhead_rejected_total"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("metrics missing %q", want)
		}
	}
}
