package openai_chat_completion

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

// e2eHTTPTimeout 是单条 e2e 请求从发出到整段流读完的总超时。
// kimi thinking 模式下首个实质 token 通常 5~15s，保守给 90s。
const e2eHTTPTimeout = 90 * time.Second

// e2eStreamReadDeadline 是流式用例从收到响应头后允许继续读 body 的最长时间；
// 我们只想读到首个有实质内容的 delta 就退出，所以这里比总超时短一些。
const e2eStreamReadDeadline = 60 * time.Second

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

// newE2EClient 返回一个带总超时的 http.Client；禁止在 e2e 中使用
// http.DefaultClient（默认无超时，流式响应可能永远不返回）。
func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
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

	resp, err := newE2EClient().Do(req)
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
// 断言策略：HTTP 200 + text/event-stream + X-Trace-Id 响应头之外，还要等到上游至少
// 吐出一条**含实质内容**的 delta（content 或 reasoning_content 非空），才算证明
// 上游接受了我们的 reasoning_content 占位并真的开始推理；只看到空壳 role chunk
// 不能证明链路健康（极端情况下 Moonshot 可能先发 role 再 500）。
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
	traceID := resp.Header.Get("X-Trace-Id")
	if traceID == "" {
		t.Errorf("missing X-Trace-Id header in stream response")
	}

	deadline := time.Now().Add(e2eStreamReadDeadline)
	reader := bufio.NewReader(resp.Body)
	var (
		dataLines    int
		gotSubstance bool
		firstDelta   string
	)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("stream read deadline exceeded after %s without substantive delta (traceID=%s, data_lines=%d)", e2eStreamReadDeadline, traceID, dataLines)
		}
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, "data: ") {
			dataLines++
			payload := strings.TrimPrefix(trimmed, "data: ")
			if payload == "[DONE]" {
				break
			}
			if hasSubstantiveDelta(payload) {
				gotSubstance = true
				if firstDelta == "" {
					firstDelta = payload
				}
				break
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			t.Fatalf("failed to read SSE stream (traceID=%s): %v", traceID, readErr)
		}
	}

	if !gotSubstance {
		t.Fatalf("stream ended without any substantive delta (traceID=%s, data_lines=%d); reasoning_content compatibility regression?", traceID, dataLines)
	}
	t.Logf("stream ok (traceID=%s, data_lines_before_substance=%d): %s", traceID, dataLines, firstDelta)
}

// TestChatCompletion_GPT55_AliasRegression_NonStream 是针对 endpointFields 缺少 "alias"
// 导致所有模型被误判为 "model_not_found" 的线上回归用例。
//
// 场景：2026-04-25 的 a013442 在 CreateEndpoint 中添加了 alias 非空校验，但
// endpoint_repository.go 的 endpointFields 未同步包含 "alias"，导致 GORM 查询后
// m.Alias 为空字符串，toAggregate 调用 CreateEndpoint 时触发验证失败，最终上层
// 包装为 [OpenAIUseCase] Model not found 并返回 404。
// 原始 traceId: c4ebac29-1e05-42cc-b7f0-10f54456e4ca
//
// 断言策略：HTTP 200 + 响应 JSON 关键字段存在。若上游返回 404，其 body 格式为
// {"error":{...}}，与代理层直接返回的 {"message":...} 不同；此处优先断言 200，
// 因为 gpt-5.5 在生产环境有实际配置且被频繁调用。
func TestChatCompletion_GPT55_AliasRegression_NonStream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)

	resp := postChatCompletions(t, baseURL, apiKey, loadFixture(t, "gpt55_alias_regression_non_stream"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		traceID := resp.Header.Get("X-Trace-Id")
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, traceID, string(body))
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

// hasSubstantiveDelta 检查一条 SSE chunk payload 是否携带了实质内容
// （content 非空 或 reasoning_content 非空），用于判定流式链路是否真的在产 token。
// 只有 role 字段的空壳 chunk 返回 false。
func hasSubstantiveDelta(payload string) bool {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := sonic.UnmarshalString(payload, &chunk); err != nil {
		return false
	}
	for _, c := range chunk.Choices {
		if c.Delta.Content != "" || c.Delta.ReasoningContent != "" {
			return true
		}
	}
	return false
}
