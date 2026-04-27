// Package openai_max_tokens_passthrough 是针对 `max_tokens` 字段被 proxy silent 改写
// 导致生产 503 的线上回归用例。
//
// 原始 bug (traceID 1d2403ab-7be1-4b0b-a189-ea2e4eb8d434, 2026-04-27 17:44, 上游
// api.chatanywhere.tech, model=gpt-5.5):
//
//	用户通过 opencode 客户端发 `max_tokens:32000`，proxy 在 openAIService.forwardNative
//	里把它 silent 改写为 `max_completion_tokens:32000`。chatanywhere 只认 `max_tokens`，
//	对 `max_completion_tokens` 直接忽略，相当于把用户设定的生成上限抹掉；叠加超长
//	system prompt + 32 tools，导致上游模型跑飞后返回 503 "模型无返回结果"。
//
// 修复（见 internal/service/openai.go: forwardNative）：删除 silent 改写，改为原样透传。
//
// 验证思路：直接构造 `max_tokens:1` 的请求。
//   - 修复前：proxy 把 max_tokens 改成 max_completion_tokens → chatanywhere 忽略 →
//     模型按默认上限生成，会产出**长回复**（usage.completion_tokens >> 1）。
//   - 修复后：max_tokens 原样透传 → 上游识别并截断 → 响应立即 `finish_reason=length`
//     且 `usage.completion_tokens <= 1`（极小）。
//
// 我们用 `usage.completion_tokens <= 5` 做上界断言（给上游一点 tokenizer 差异容差），
// 从而证明 max_tokens 确实被上游生效。
package openai_max_tokens_passthrough

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

// e2eHTTPTimeout 单条请求的总超时；max_tokens:1 的请求理应很快返回，60s 足够。
const e2eHTTPTimeout = 60 * time.Second

// e2eStreamReadDeadline 流式响应读取 body 的最长时间。
const e2eStreamReadDeadline = 45 * time.Second

// completionTokensUpperBound 我们希望 `max_tokens:1` 在透传到上游后，
// 上游立即截断。不同 tokenizer 对 BOS / role token 的计费略有差异，允许最多 5 个
// completion token 作为容差；一旦超过说明 max_tokens 没被上游识别（即仍然被 proxy
// 改写或漏传）。
const completionTokensUpperBound = 5

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

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

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

// TestMaxTokensPassthrough_NonStream 非流式路径下，`max_tokens:1` 必须被上游生效。
// 回归场景：gpt-5.5 / chatanywhere.tech（503 "模型无返回结果" 的那条线）。
func TestMaxTokensPassthrough_NonStream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)

	resp := postChatCompletions(t, baseURL, apiKey, loadFixture(t, "gpt55_max_tokens_1_non_stream"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get("X-Trace-Id"), string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if !sonic.ValidString(string(respBody)) {
		t.Fatalf("response is not valid JSON: %s", string(respBody))
	}

	var obj struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := sonic.Unmarshal(respBody, &obj); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if strings.TrimSpace(obj.ID) == "" {
		t.Errorf("missing or empty id")
	}
	if strings.TrimSpace(obj.Model) == "" {
		t.Errorf("missing or empty model")
	}
	if len(obj.Choices) == 0 {
		t.Fatalf("missing choices in response: %s", string(respBody))
	}
	if obj.Usage == nil {
		t.Fatalf("missing usage in response: %s", string(respBody))
	}

	// 关键断言：max_tokens 透传到上游 → 上游必然截断到极短。
	// 修复前 max_tokens 被改成 max_completion_tokens 被 chatanywhere 忽略，
	// completion_tokens 会远大于阈值。
	if obj.Usage.CompletionTokens > completionTokensUpperBound {
		t.Fatalf("max_tokens passthrough regression: usage.completion_tokens=%d exceeds upper bound %d "+
			"(max_tokens was likely rewritten/dropped before upstream saw it); body: %s",
			obj.Usage.CompletionTokens, completionTokensUpperBound, string(respBody))
	}
	t.Logf("non-stream ok: finish_reason=%q completion_tokens=%d total_tokens=%d",
		obj.Choices[0].FinishReason, obj.Usage.CompletionTokens, obj.Usage.TotalTokens)
}

// TestMaxTokensPassthrough_Stream 流式路径下，`max_tokens:1` 同样必须被上游生效。
// 用最后一个携带 usage 的 chunk（stream_options.include_usage=true）做断言。
func TestMaxTokensPassthrough_Stream(t *testing.T) {
	baseURL, apiKey := mustE2EEnv(t)

	resp := postChatCompletions(t, baseURL, apiKey, loadFixture(t, "gpt55_max_tokens_1_stream"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status = %d (traceID=%s); body: %s", resp.StatusCode, resp.Header.Get("X-Trace-Id"), string(body))
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
		lastUsageCompletionTokens = -1
		dataLines                 int
	)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("stream read deadline exceeded after %s (traceID=%s, data_lines=%d)", e2eStreamReadDeadline, traceID, dataLines)
		}
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, "data: ") {
			dataLines++
			payload := strings.TrimPrefix(trimmed, "data: ")
			if payload == "[DONE]" {
				break
			}
			if n, ok := extractUsageCompletionTokens(payload); ok {
				lastUsageCompletionTokens = n
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			t.Fatalf("failed to read SSE stream (traceID=%s): %v", traceID, readErr)
		}
	}

	if lastUsageCompletionTokens < 0 {
		t.Fatalf("no usage chunk observed in stream (traceID=%s, data_lines=%d); "+
			"stream_options.include_usage=true should guarantee one", traceID, dataLines)
	}
	// 关键断言（同非流式路径）。
	if lastUsageCompletionTokens > completionTokensUpperBound {
		t.Fatalf("max_tokens passthrough regression (stream): usage.completion_tokens=%d exceeds upper bound %d "+
			"(traceID=%s)", lastUsageCompletionTokens, completionTokensUpperBound, traceID)
	}
	t.Logf("stream ok (traceID=%s): data_lines=%d completion_tokens=%d", traceID, dataLines, lastUsageCompletionTokens)
}

// extractUsageCompletionTokens 从一条 SSE chunk payload 中提取 usage.completion_tokens；
// 若当前 chunk 没有 usage 或不是合法 JSON，返回 (_, false)。
func extractUsageCompletionTokens(payload string) (int, bool) {
	var chunk struct {
		Usage *struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := sonic.UnmarshalString(payload, &chunk); err != nil {
		return 0, false
	}
	if chunk.Usage == nil {
		return 0, false
	}
	return chunk.Usage.CompletionTokens, true
}
