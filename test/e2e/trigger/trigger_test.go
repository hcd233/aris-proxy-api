package trigger_e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
)

// mustTriggerE2EEnv 返回 (baseURL, apiKey, adminToken) 或 t.Skip。
// E2E 默认离线 skip，只有显式提供环境变量时才打到目标环境。
// ADMIN_TOKEN 是管理后台 JWT（admin 权限），用于创建/修改/删除触发词。
func mustTriggerE2EEnv(t *testing.T) (baseURL, apiKey, adminToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	adminToken = os.Getenv("ADMIN_TOKEN")
	if baseURL == "" || apiKey == "" || adminToken == "" {
		t.Skip("BASE_URL, API_KEY and ADMIN_TOKEN are required for trigger e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey, adminToken
}

// createTriggerWord 创建触发词并返回其 ID（管理接口，admin JWT）。
func createTriggerWord(t *testing.T, baseURL, adminToken, word, action string) uint {
	t.Helper()
	body := map[string]string{"word": word}
	if action != "" {
		body["action"] = action
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal create body: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/v1/trigger", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create trigger word: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("create trigger word status = %d, body: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read create response: %v", err)
	}
	var rsp struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(respBody, &rsp); err != nil {
		t.Fatalf("create trigger word returned unexpected response: %s", string(respBody))
	}
	if rsp.Error != nil {
		t.Fatalf("create trigger word failed: %s", string(respBody))
	}

	// 通过 list 查询该词的 ID
	id, ok := findTriggerID(t, baseURL, adminToken, word)
	if !ok {
		t.Fatalf("trigger word %q not found after create", word)
	}
	return id
}

// findTriggerID 在分页列表中查找指定触发词的 ID。
func findTriggerID(t *testing.T, baseURL, adminToken, word string) (uint, bool) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/v1/trigger/list?page=1&pageSize=100", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create list request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to list trigger words: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read list response: %v", err)
	}

	var rsp struct {
		Data struct {
			Trigger []struct {
				ID     uint   `json:"id"`
				Word   string `json:"word"`
				Action string `json:"action"`
			} `json:"trigger"`
		} `json:"data"`
	}
	if err := sonic.Unmarshal(respBody, &rsp); err != nil {
		t.Fatalf("failed to unmarshal list response: %s", string(respBody))
	}
	for _, b := range rsp.Data.Trigger {
		if b.Word == word {
			return b.ID, true
		}
	}
	return 0, false
}

// updateTriggerAction 修改触发词 action（管理接口，admin JWT）。
func updateTriggerAction(t *testing.T, baseURL, adminToken string, id uint, action string) {
	t.Helper()
	payload, err := sonic.Marshal(map[string]string{"action": action})
	if err != nil {
		t.Fatalf("failed to marshal update body: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, fmt.Sprintf("%s/api/v1/trigger?id=%d", baseURL, id), strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to update trigger word: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("update trigger word status = %d, body: %s", resp.StatusCode, string(respBody))
	}
}

// deleteTriggerWord 删除触发词（管理接口，admin JWT）。
func deleteTriggerWord(t *testing.T, baseURL, adminToken string, id uint) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, fmt.Sprintf("%s/api/v1/trigger?ids=%d", baseURL, id), http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to delete trigger word: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("delete trigger word status = %d, body: %s", resp.StatusCode, string(respBody))
	}
}

// chatWithWord 发送一次非流式 chat completion，消息内容包含指定触发词。
func chatWithWord(t *testing.T, baseURL, apiKey, word string) *http.Response {
	t.Helper()
	body := map[string]any{
		"model":      "gpt-5.5",
		"messages":   []map[string]string{{"role": "user", "content": "please say: " + word}},
		"stream":     false,
		"max_tokens": 50,
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal chat body: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send chat request: %v", err)
	}
	return resp
}

// chatWithSystemWord 发送一次非流式 OpenAI chat completion，触发词放在 system 消息中。
func chatWithSystemWord(t *testing.T, baseURL, apiKey, word string) *http.Response {
	t.Helper()
	body := map[string]any{
		"model": "gpt-5.5",
		"messages": []map[string]string{
			{"role": "system", "content": "system rule says: " + word},
			{"role": "user", "content": "hello"},
		},
		"stream":     false,
		"max_tokens": 50,
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal chat body: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send chat request: %v", err)
	}
	return resp
}

// anthropicChatWithSystemWord 发送一次非流式 Anthropic messages 请求，触发词放在顶层 system 字段。
func anthropicChatWithSystemWord(t *testing.T, baseURL, apiKey, word string) *http.Response {
	t.Helper()
	body := map[string]any{
		"model":      "deepseek-v4-flash",
		"system":     "system rule says: " + word,
		"messages":   []map[string]string{{"role": "user", "content": "hello"}},
		"max_tokens": 50,
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal chat body: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/anthropic/v1/messages", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send chat request: %v", err)
	}
	return resp
}

// uniqueWord 生成本次测试唯一的触发词，避免污染环境中的存量数据。
func uniqueWord(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, os.Getpid())
}

// assertOpenAIContentFilterResp 断言 OpenAI chat 响应为内容拦截形态：200 + finish_reason=content_filter。
func assertOpenAIContentFilterResp(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("deny word: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read chat response: %v", err)
	}
	var obj struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := sonic.Unmarshal(body, &obj); err != nil {
		t.Fatalf("deny word: response is not valid JSON: %s", string(body))
	}
	if len(obj.Choices) == 0 {
		t.Fatalf("deny word: missing choices, body: %s", string(body))
	}
	if obj.Choices[0].FinishReason != "content_filter" {
		t.Fatalf("deny word: finish_reason = %q, want content_filter, body: %s", obj.Choices[0].FinishReason, string(body))
	}
}

// assertAnthropicRefusalResp 断言 Anthropic 响应为内容拦截形态：200 + stop_reason=refusal。
func assertAnthropicRefusalResp(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("deny word: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read anthropic response: %v", err)
	}
	var obj struct {
		StopReason  string `json:"stop_reason"`
		StopDetails *struct {
			Type string `json:"type"`
		} `json:"stop_details"`
	}
	if err := sonic.Unmarshal(body, &obj); err != nil {
		t.Fatalf("deny word: response is not valid JSON: %s", string(body))
	}
	if obj.StopReason != "refusal" {
		t.Fatalf("deny word: stop_reason = %q, want refusal, body: %s", obj.StopReason, string(body))
	}
	if obj.StopDetails == nil || obj.StopDetails.Type != "refusal" {
		t.Fatalf("deny word: stop_details = %+v, want type=refusal, body: %s", obj.StopDetails, string(body))
	}
}

// TestTrigger_SystemMessage_OpenAI_ContentFilter 验证触发词出现在 OpenAI system 消息中时拦截（200 content_filter）。
func TestTrigger_SystemMessage_OpenAI_ContentFilter(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2esysopenai")
	id := createTriggerWord(t, baseURL, adminToken, word, "deny")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := chatWithSystemWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	assertOpenAIContentFilterResp(t, resp)
}

// TestTrigger_SystemMessage_Anthropic_Refusal 验证触发词出现在 Anthropic 顶层 system 字段时拦截（200 refusal）。
func TestTrigger_SystemMessage_Anthropic_Refusal(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2esysanthropic")
	id := createTriggerWord(t, baseURL, adminToken, word, "deny")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := anthropicChatWithSystemWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	assertAnthropicRefusalResp(t, resp)
}

// TestTrigger_Deny_ContentFilter 验证 deny 型触发词命中时返回 200 内容拦截消息（content_filter）。
func TestTrigger_Deny_ContentFilter(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2edeny")
	id := createTriggerWord(t, baseURL, adminToken, word, "deny")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := chatWithWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	assertOpenAIContentFilterResp(t, resp)
}

// TestTrigger_Omit_ForwardsToUpstream 验证 omit 型触发词命中时请求照常转发（200 + 正常响应）。
func TestTrigger_Omit_ForwardsToUpstream(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2eomit")
	id := createTriggerWord(t, baseURL, adminToken, word, "omit")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := chatWithWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("omit word: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read chat response: %v", err)
	}
	var obj map[string]any
	if err := sonic.Unmarshal(body, &obj); err != nil {
		t.Fatalf("omit word: response is not valid JSON: %s", string(body))
	}
	if obj["choices"] == nil {
		t.Fatalf("omit word: missing choices, body: %s", string(body))
	}
}

// TestTrigger_UpdateAction_Switches 验证 PATCH 修改 action 后行为切换（deny → 200 content_filter、omit → 正常转发）。
func TestTrigger_UpdateAction_Switches(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2eswitch")
	id := createTriggerWord(t, baseURL, adminToken, word, "deny")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := chatWithWord(t, baseURL, apiKey, word)
	assertOpenAIContentFilterResp(t, resp)
	_ = resp.Body.Close()

	updateTriggerAction(t, baseURL, adminToken, id, "omit")

	resp = chatWithWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("after update to omit: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
}

// TestTrigger_Mixed_DenyWins 验证同时命中 deny 与 omit 型词时 deny 优先（200 content_filter，不转发）。
func TestTrigger_Mixed_DenyWins(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	denyWord := uniqueWord("e2emixdeny")
	omitWord := uniqueWord("e2emixomit")
	denyID := createTriggerWord(t, baseURL, adminToken, denyWord, "deny")
	omitID := createTriggerWord(t, baseURL, adminToken, omitWord, "omit")
	defer deleteTriggerWord(t, baseURL, adminToken, denyID)
	defer deleteTriggerWord(t, baseURL, adminToken, omitID)

	resp := chatWithWord(t, baseURL, apiKey, denyWord+" "+omitWord)
	defer func() { _ = resp.Body.Close() }()
	assertOpenAIContentFilterResp(t, resp)
}

// TestTrigger_RecreateAfterDelete 回归（fix/trigger-word-recreate-2026-08-12）：软删除后重新添加同词应成功。
// 修复前 trigger_words.word 单列唯一索引被软删除记录永久占用，重新添加同词违反唯一约束返回 500；
// 修复后唯一索引为 (word, deleted_at) 复合，软删记录不再占用索引。
func TestTrigger_RecreateAfterDelete(t *testing.T) {
	t.Parallel()
	baseURL, _, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2erecreate")
	id := createTriggerWord(t, baseURL, adminToken, word, "deny")
	deleteTriggerWord(t, baseURL, adminToken, id)

	// 软删除后再次添加同词：修复前返回 500，修复后应成功
	id2 := createTriggerWord(t, baseURL, adminToken, word, "deny")
	deleteTriggerWord(t, baseURL, adminToken, id2)
}

// TestTrigger_DuplicateCreate_Returns409 回归（fix/trigger-word-recreate-2026-08-12）：
// 未删除的同词重复添加应返回 409 + code 10004（Data Already Exists），而非 500 Internal Error。
func TestTrigger_DuplicateCreate_Returns409(t *testing.T) {
	t.Parallel()
	baseURL, _, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2edup")
	id := createTriggerWord(t, baseURL, adminToken, word, "deny")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	payload, err := sonic.Marshal(map[string]string{"word": word, "action": "deny"})
	if err != nil {
		t.Fatalf("failed to marshal duplicate create body: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/v1/trigger", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send duplicate create request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("duplicate create: unified contract expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read duplicate create response: %v", err)
	}
	var rsp struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(body, &rsp); err != nil || rsp.Error == nil || rsp.Error.Code != 10004 {
		t.Fatalf("duplicate create: expected code 10004, body: %s", string(body))
	}
}

// TestTrigger_BatchDelete 验证 DELETE /api/v1/trigger?ids=1,2,3 批量删除及 deletedCount 返回。
func TestTrigger_BatchDelete(t *testing.T) {
	t.Parallel()
	baseURL, _, adminToken := mustTriggerE2EEnv(t)

	w1 := uniqueWord("e2ebatch")
	id1 := createTriggerWord(t, baseURL, adminToken, w1, "deny")
	id2 := createTriggerWord(t, baseURL, adminToken, uniqueWord("e2ebatch"), "omit")

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete,
		fmt.Sprintf("%s/api/v1/trigger?ids=%d,%d", baseURL, id1, id2), http.NoBody)
	if err != nil {
		t.Fatalf("failed to create batch delete request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send batch delete request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch delete status = %d, body: %s", resp.StatusCode, string(respBody))
	}

	var rsp struct {
		Data struct {
			DeletedCount int `json:"deletedCount"`
		} `json:"data"`
	}
	if err := sonic.Unmarshal(respBody, &rsp); err != nil {
		t.Fatalf("batch delete returned unexpected response: %s", string(respBody))
	}
	if rsp.Data.DeletedCount != 2 {
		t.Fatalf("expected deletedCount=2, got %d", rsp.Data.DeletedCount)
	}

	// 删除后不应再出现在列表中
	if _, ok := findTriggerID(t, baseURL, adminToken, w1); ok {
		t.Fatal("trigger word still exists after batch delete")
	}
}

// chatCaptureWithHistory 发送一次带历史对话的非流式 chat completion，
// 最后一条 user 消息包含 capture 触发词（历史 + 触发消息共 3 条）。
func chatCaptureWithHistory(t *testing.T, baseURL, apiKey, word string) *http.Response {
	t.Helper()
	body := map[string]any{
		"model": "gpt-5.5",
		"messages": []map[string]string{
			{"role": "user", "content": "first question about proxies"},
			{"role": "assistant", "content": "first answer"},
			{"role": "user", "content": "please capture: " + word},
		},
		"stream":     false,
		"max_tokens": 50,
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal capture chat body: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create capture chat request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send capture chat request: %v", err)
	}
	return resp
}

// anthropicCaptureStream 发送一次带历史的流式 Anthropic messages 请求，最后一条 user 消息含 capture 触发词。
func anthropicCaptureStream(t *testing.T, baseURL, apiKey, word string) *http.Response {
	t.Helper()
	body := map[string]any{
		"model": "deepseek-v4-flash",
		"messages": []map[string]string{
			{"role": "user", "content": "history question"},
			{"role": "user", "content": "please capture: " + word},
		},
		"stream":     true,
		"max_tokens": 50,
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal anthropic capture body: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/anthropic/v1/messages", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create anthropic capture request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send anthropic capture request: %v", err)
	}
	return resp
}

// TestTrigger_Capture_ReturnsFixedReply_WithHistory 验证 capture 型触发词位于最后一条 user 消息
// 且存在历史时：不请求上游，返回 200 + "Context saved." 固定回复（choices 结构完整，客户端可解析）。
func TestTrigger_Capture_ReturnsFixedReply_WithHistory(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2ecapture")
	id := createTriggerWord(t, baseURL, adminToken, word, "capture")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := chatCaptureWithHistory(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("capture hit: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read capture response: %v", err)
	}
	if !strings.Contains(string(body), "Context saved.") {
		t.Fatalf("capture hit: expected fixed reply 'Context saved.', body: %s", string(body))
	}
	var obj map[string]any
	if err := sonic.Unmarshal(body, &obj); err != nil || obj["choices"] == nil {
		t.Fatalf("capture hit: response missing valid choices, body: %s", string(body))
	}
}

// TestTrigger_Capture_NoHistory_ReturnsEmptyReply 验证触发消息为第一条（无历史）时返回
// "No conversation history to save."。
func TestTrigger_Capture_NoHistory_ReturnsEmptyReply(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2ecapempty")
	id := createTriggerWord(t, baseURL, adminToken, word, "capture")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := chatWithWord(t, baseURL, apiKey, word) // 单条消息即触发消息，无历史
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("capture no-history: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "No conversation history to save.") {
		t.Fatalf("capture no-history: expected empty-history reply, body: %s", string(body))
	}
}

// TestTrigger_Capture_Stream_CompleteSSESequence 验证 stream=true 的 Anthropic capture
// 返回完整 SSE 事件序列（message_start ... message_stop）且携带固定回复。
func TestTrigger_Capture_Stream_CompleteSSESequence(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2ecapstream")
	id := createTriggerWord(t, baseURL, adminToken, word, "capture")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	resp := anthropicCaptureStream(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("capture stream: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read capture stream: %v", err)
	}
	sse := string(body)
	for _, event := range []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"} {
		if !strings.Contains(sse, "event: "+event) {
			t.Fatalf("capture stream: missing event %s, sse: %s", event, sse)
		}
	}
	if !strings.Contains(sse, "Context saved.") {
		t.Fatalf("capture stream: missing fixed reply, sse: %s", sse)
	}
}

// TestTrigger_Capture_WordOnlyInHistory_Forwards 验证触发词只出现在历史消息（非最后一条
// user 提问）时 capture 不生效，请求照常转发（200，不返回固定回复）。
func TestTrigger_Capture_WordOnlyInHistory_Forwards(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	word := uniqueWord("e2ecaphist")
	id := createTriggerWord(t, baseURL, adminToken, word, "capture")
	defer deleteTriggerWord(t, baseURL, adminToken, id)

	body := map[string]any{
		"model": "gpt-5.5",
		"messages": []map[string]string{
			{"role": "user", "content": "history mentions " + word},
			{"role": "assistant", "content": "ok"},
			{"role": "user", "content": "a normal follow-up question"},
		},
		"stream":     false,
		"max_tokens": 50,
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("history-only capture word: expected 200 (forwarded), got %d, body: %s", resp.StatusCode, string(respBody))
	}
	respBody, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(respBody), "Context saved.") {
		t.Fatalf("history-only capture word should not capture, got fixed reply: %s", string(respBody))
	}
}

// TestTrigger_Mixed_DenyWinsOverCapture 验证同时命中 deny 与 capture 词时 deny 优先
// （200 content_filter 拦截消息，不保存上下文、不返回 capture 固定回复）。
func TestTrigger_Mixed_DenyWinsOverCapture(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustTriggerE2EEnv(t)

	denyWord := uniqueWord("e2edencap")
	captureWord := uniqueWord("e2ecapdeny")
	denyID := createTriggerWord(t, baseURL, adminToken, denyWord, "deny")
	captureID := createTriggerWord(t, baseURL, adminToken, captureWord, "capture")
	defer deleteTriggerWord(t, baseURL, adminToken, denyID)
	defer deleteTriggerWord(t, baseURL, adminToken, captureID)

	body := map[string]any{
		"model": "gpt-5.5",
		"messages": []map[string]string{
			{"role": "user", "content": "first question"},
			{"role": "user", "content": captureWord + " " + denyWord},
		},
		"stream":     false,
		"max_tokens": 50,
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	assertOpenAIContentFilterResp(t, resp)
	respBody, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(respBody), "Context saved.") {
		t.Fatalf("deny+capture: deny wins, capture reply must not appear, body: %s", string(respBody))
	}
}

// TestE2E_TriggerDeleteNoPermission 验证统一错误契约：普通用户删除触发词
// 返回 HTTP 200 + error 10002（旧契约为 403），前端据此识别失败而非误判成功。
// 需要 USER_TOKEN（普通用户 JWT）；缺失时跳过。
func TestE2E_TriggerDeleteNoPermission(t *testing.T) {
	t.Parallel()
	baseURL, _, _ := mustTriggerE2EEnv(t)
	userToken := os.Getenv("USER_TOKEN")
	if userToken == "" {
		t.Skip("USER_TOKEN is required for no-permission trigger delete e2e test")
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, baseURL+"/api/v1/trigger?ids=1", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send no-permission delete request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unified contract: expected 200 for no-permission delete, got %d, body: %s", resp.StatusCode, string(respBody))
	}
	var rsp struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(respBody, &rsp); err != nil || rsp.Error == nil || rsp.Error.Code != 10002 {
		t.Fatalf("expected error code 10002 for no-permission delete, body=%s", string(respBody))
	}
}
