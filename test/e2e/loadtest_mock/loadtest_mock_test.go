// Package loadtest_mock 验证三接口 mock 模型的随机字符流输出与落库形态。
// 默认离线 skip，需显式提供 BASE_URL + API_KEY 才打到目标环境。
//
// 落库形态的核验边界：本包只断言响应体形态。审计 / session / message 的落库与
// 清理归零由 docs/loadtest/BENCHMARK.md §5.2 覆盖（清理脚本 dry-run 预览 +
// Step 7 复核）；e2e 侧查库需另配 JWT 并处理审计异步写入的等待，未在本包实现。
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

	// 流式：完整读完全流（不提前 break），断言 role 首片 / content 片 / finish_reason
	// 收尾片都出现——只扫到首个 content 就中断会漏掉流的后半段形态问题
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var sawRole, sawContent, sawFinish bool
	for scanner.Scan() {
		line := scanner.Text()
		sawRole = sawRole || strings.Contains(line, `"role":"assistant"`)
		sawContent = sawContent || strings.Contains(line, `"content"`)
		sawFinish = sawFinish || strings.Contains(line, `"finish_reason":"stop"`)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !sawRole || !sawContent || !sawFinish {
		t.Fatalf("stream shape incomplete: role=%v content=%v finish=%v", sawRole, sawContent, sawFinish)
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
				Type string `json:"type"`
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
	if out.Output[0].Content[0].Type != "output_text" {
		t.Fatalf("content type want output_text, got %q; body=%s", out.Output[0].Content[0].Type, raw)
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
	if out.Content[0].Type != "text" {
		t.Fatalf("content type want text, got %q; body=%s", out.Content[0].Type, raw)
	}
	if out.Usage.OutputTokens <= 0 {
		t.Fatal("usage.output_tokens must be > 0")
	}
}

// TestMockMessageStreamContentBlockStartEmpty 验证 Anthropic 流式 content_block_start
// 只声明空块、正文全部走 delta：start 携带全文会让官方 SDK 按「初值 + delta 追加」
// 累加出两倍正文（网关自身聚合因覆盖语义掩盖该缺陷）。
func TestMockMessageStreamContentBlockStartEmpty(t *testing.T) {
	t.Parallel()
	baseURL, apiKey := mustEnv(t)
	body := []byte(`{"model":"mock-messages","stream":true,"max_tokens":64,"messages":[{"role":"user","content":"` + loadtestMarker + ` e2e probe"}]}`)

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

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	currentEvent := ""
	startSeen, deltaChars := false, 0
	for scanner.Scan() {
		line := scanner.Text()
		if after, ok := strings.CutPrefix(line, "event:"); ok {
			currentEvent = strings.TrimSpace(after)
			continue
		}
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		switch currentEvent {
		case "content_block_start":
			var payload struct {
				ContentBlock struct {
					Text *string `json:"text"`
				} `json:"content_block"`
			}
			if err := sonic.Unmarshal([]byte(strings.TrimSpace(data)), &payload); err != nil {
				t.Fatalf("unmarshal content_block_start: %v", err)
			}
			if payload.ContentBlock.Text == nil || *payload.ContentBlock.Text != "" {
				t.Fatalf("content_block_start text must be empty, got %v", payload.ContentBlock.Text)
			}
			startSeen = true
		case "content_block_delta":
			var payload struct {
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := sonic.Unmarshal([]byte(strings.TrimSpace(data)), &payload); err != nil {
				t.Fatalf("unmarshal content_block_delta: %v", err)
			}
			deltaChars += len(payload.Delta.Text)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !startSeen {
		t.Fatal("content_block_start event not observed")
	}
	if deltaChars == 0 {
		t.Fatal("no content_block_delta text observed")
	}
}
