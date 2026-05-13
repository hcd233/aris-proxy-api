// Package header_passthrough 验证自定义请求头被透传到上游。
//
// 测试方法：
//   - 发送含 X-Custom-Header / X-Request-Id 的请求
//   - 断言 HTTP 200（说明上游正常处理，proxy 没有因为未知头而出错）
//   - 断言响应含 X-Trace-Id（确认请求被正确路由）
package header_passthrough

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 60 * time.Second
const e2eStreamReadDeadline = 45 * time.Second

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := "./fixtures/requests/" + name + ".json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return data
}

func mustE2EEnv(t *testing.T) (string, string) {
	t.Helper()
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey
}

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

func postWithHeaders(t *testing.T, baseURL, apiKey string, body []byte, extraHeaders map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	return resp
}

func TestHeaderPassthrough_NonStream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)

	resp := postWithHeaders(t, baseURL, apiKey, loadFixture(t, "chat_completion"), map[string]string{
		"X-Custom-Header": "passthrough-test-value",
		"X-Request-Id":    "test-request-123",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get(constant.HTTPTitleHeaderTraceID), string(body))
	}

	traceID := resp.Header.Get(constant.HTTPTitleHeaderTraceID)
	if traceID == "" {
		t.Error("expected X-Trace-Id header in response")
	}
	t.Logf("Non-stream test passed, traceID=%s", traceID)
}

func TestHeaderPassthrough_Stream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)

	resp := postWithHeaders(t, baseURL, apiKey, loadFixture(t, "chat_completion_stream"), map[string]string{
		"X-Custom-Header": "passthrough-stream-test",
		"X-Request-Id":    "test-stream-456",
	})
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get(constant.HTTPTitleHeaderTraceID), string(body))
	}

	traceID := resp.Header.Get(constant.HTTPTitleHeaderTraceID)
	if traceID == "" {
		t.Error("expected X-Trace-Id header in response")
	}
	t.Logf("Stream test traceID=%s", traceID)

	// 消费 stream 直到结束
	deadline := time.After(e2eStreamReadDeadline)
	reader := bufio.NewReader(resp.Body)
	hasData := false
	for {
		select {
		case <-deadline:
			t.Fatal("stream read deadline exceeded")
		default:
		}
		raw, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			t.Fatalf("stream read error: %v", readErr)
		}
		if strings.HasPrefix(raw, "data: ") {
			hasData = true
		}
	}
	if !hasData {
		t.Log("stream returned no data events")
	}
}
