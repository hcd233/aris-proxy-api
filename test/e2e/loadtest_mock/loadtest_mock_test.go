// Package loadtest_mock 验证三接口 mock 模型的随机字符流输出与落库形态。
// 默认离线 skip，需显式提供 BASE_URL + API_KEY 才打到目标环境。
package loadtest_mock

import (
	"bufio"
	"bytes"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	// e2eHTTPTimeout 单条 e2e 请求从发出到整段流读完的总超时。
	// mock 流时长可达 205 秒，这里给足余量。
	e2eHTTPTimeout = 300 * time.Second
	// loadtestMarker 压测固定前缀，保证 checksum 不与真实消息相同。
	loadtestMarker = "__LOADTEST__"
)

// mustEnv 返回 (baseURL, apiKey) 或 t.Skip。
func mustEnv(t *testing.T) (baseURL, apiKey string) {
	t.Helper()
	baseURL, apiKey = os.Getenv("BASE_URL"), os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey
}

func newClient() *http.Client { return &http.Client{Timeout: e2eHTTPTimeout} }

// TestMockChatCompletionStream 验证 chat 接口流式返回随机字符流。
func TestMockChatCompletionStream(t *testing.T) {
	t.Parallel()
	baseURL, apiKey := mustEnv(t)
	body := []byte(`{"model":"mock-chat","stream":true,"max_tokens":256,"messages":[{"role":"user","content":"` + loadtestMarker + ` e2e probe"}]}`)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, baseURL+"/api/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)

	resp, err := newClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// 流式：至少读到一个带 content 的 chunk
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sawContent := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), `"content"`) {
			sawContent = true
			break
		}
	}
	if !sawContent {
		t.Fatal("no content chunk observed in SSE stream")
	}
}

// TestMockResponseNonStream 验证 responses 接口非流式返回随机文本。
func TestMockResponseNonStream(t *testing.T) {
	t.Parallel()
	baseURL, apiKey := mustEnv(t)
	body := []byte(`{"model":"mock-response","max_output_tokens":256,"input":"` + loadtestMarker + ` e2e probe"}`)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, baseURL+"/api/openai/v1/responses", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)

	resp, err := newClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Status string `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := sonic.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, raw)
	}
	if out.Status != "completed" {
		t.Fatalf("status want completed, got %s", out.Status)
	}
	if len(out.Output) == 0 || len(out.Output[0].Content) == 0 || out.Output[0].Content[0].Text == "" {
		t.Fatalf("empty mock output: %s", raw)
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatal("usage.output_tokens must be > 0")
	}
}

// TestMockMessageNonStream 验证 messages 接口非流式返回随机文本。
func TestMockMessageNonStream(t *testing.T) {
	t.Parallel()
	baseURL, apiKey := mustEnv(t)
	body := []byte(`{"model":"mock-messages","max_tokens":256,"messages":[{"role":"user","content":"` + loadtestMarker + ` e2e probe"}]}`)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, baseURL+"/api/anthropic/v1/messages", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")

	resp, err := newClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := sonic.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, raw)
	}
	if len(out.Content) == 0 || out.Content[0].Text == "" {
		t.Fatalf("empty mock content: %s", raw)
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatal("usage.output_tokens must be > 0")
	}
}
