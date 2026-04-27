package openai_chat_completion

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
)

// loadFixture 读取 fixtures/requests/<name>.json 原始字节。
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := "./fixtures/requests/" + name + ".json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return data
}

// mustE2EEnv 返回 (baseURL, apiKey) 或 t.Skip；
// E2E 默认离线 skip，只有显式提供环境变量时才打到生产。
func mustE2EEnv(t *testing.T) (string, string) {
	t.Helper()
	baseURL := os.Getenv("BASE_URL")
	apiKey := os.Getenv("API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("BASE_URL and API_KEY are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey
}

// postChatCompletions 统一封装一次 POST /api/openai/v1/chat/completions 调用。
// 调用方负责 close body 并对 resp 做断言。
func postChatCompletions(t *testing.T, baseURL, apiKey string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	return resp
}

// TestChatCompletion_ToolCall_NonStream 覆盖 kimi 工具调用的非流式路径；
// 也验证 Moonshot 思考模式下缺失 reasoning_content 的补位逻辑对非流式也生效。
func TestChatCompletion_ToolCall_NonStream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)

	resp := postChatCompletions(t, baseURL, apiKey, loadFixture(t, "tool_call_non_stream"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d, body: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !sonic.ValidString(string(respBody)) {
		t.Fatalf("response is not valid JSON: %s", string(respBody))
	}

	var obj map[string]any
	if err := sonic.Unmarshal(respBody, &obj); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if obj["id"] == nil || strings.TrimSpace(obj["id"].(string)) == "" {
		t.Errorf("missing or empty id")
	}
	if obj["model"] == nil || strings.TrimSpace(obj["model"].(string)) == "" {
		t.Errorf("missing or empty model")
	}
	if obj["choices"] == nil {
		t.Errorf("missing choices")
	}
	if obj["usage"] == nil {
		t.Errorf("missing usage")
	}
}

// TestChatCompletion_KimiThinking_MissingReasoningContent_Stream 是针对 Moonshot
// "thinking is enabled but reasoning_content is missing in assistant tool call message"
// 400 错误的线上回归用例。
//
// 场景：多轮对话历史中，带 tool_calls 的 assistant message 没有 reasoning_content 字段
// （opencode 等客户端会丢弃该字段）。代理层必须在转发前给这类消息补一个非空占位符
// （见 util.EnsureAssistantMessageReasoningContent），否则 Moonshot 会把缺失或空串
// 都判为 missing 并返回 400。
//
// 断言策略：只要上游没回 400，我们的代理就完成了兼容层工作；流式响应能读到至少一条
// `data:` SSE 行即视为转发链路正常。模型输出本身的语义不在本用例覆盖范围内。
func TestChatCompletion_KimiThinking_MissingReasoningContent_Stream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)

	resp := postChatCompletions(t, baseURL, apiKey, loadFixture(t, "kimi_thinking_missing_reasoning_stream"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (regression: Moonshot should not reject; body: %s)", resp.StatusCode, string(body))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected Content-Type = %q, want text/event-stream", ct)
	}
	if traceID := resp.Header.Get("X-Trace-Id"); traceID == "" {
		t.Errorf("missing X-Trace-Id header in stream response")
	}

	// 只要读到至少一条 SSE data 行（或 [DONE]），就证明上游没有返回 400 且代理把流
	// 接住转发给了客户端。为避免走满整段生成而拖慢 CI，读到第一条 data 行即返回。
	reader := bufio.NewReader(resp.Body)
	sawData := false
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, "data: ") {
			sawData = true
			break
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("failed to read SSE stream: %v", err)
		}
	}
	if !sawData {
		t.Fatalf("did not receive any SSE data line; reasoning_content compatibility regression?")
	}
}
