package blocked_e2e

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

// mustBlockedE2EEnv 返回 (baseURL, apiKey, adminToken) 或 t.Skip。
// E2E 默认离线 skip，只有显式提供环境变量时才打到目标环境。
// ADMIN_TOKEN 是管理后台 JWT（admin 权限），用于创建/修改/删除敏感词。
func mustBlockedE2EEnv(t *testing.T) (baseURL, apiKey, adminToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	adminToken = os.Getenv("ADMIN_TOKEN")
	if baseURL == "" || apiKey == "" || adminToken == "" {
		t.Skip("BASE_URL, API_KEY and ADMIN_TOKEN are required for blocked e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey, adminToken
}

// createBlockedWord 创建敏感词并返回其 ID（管理接口，admin JWT）。
func createBlockedWord(t *testing.T, baseURL, adminToken, word, action string) uint {
	t.Helper()
	body := map[string]string{"word": word}
	if action != "" {
		body["action"] = action
	}
	payload, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal create body: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/api/v1/block", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create blocked word: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("create blocked word status = %d, body: %s", resp.StatusCode, string(respBody))
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
		t.Fatalf("create blocked word returned unexpected response: %s", string(respBody))
	}
	if rsp.Error != nil {
		t.Fatalf("create blocked word failed: %s", string(respBody))
	}

	// 通过 list 查询该词的 ID
	id, ok := findBlockedID(t, baseURL, adminToken, word)
	if !ok {
		t.Fatalf("blocked word %q not found after create", word)
	}
	return id
}

// findBlockedID 在分页列表中查找指定敏感词的 ID。
func findBlockedID(t *testing.T, baseURL, adminToken, word string) (uint, bool) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/v1/block/list?page=1&pageSize=100", http.NoBody)
	if err != nil {
		t.Fatalf("failed to create list request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to list blocked words: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read list response: %v", err)
	}

	var rsp struct {
		Data struct {
			Blocked []struct {
				ID     uint   `json:"id"`
				Word   string `json:"word"`
				Action string `json:"action"`
			} `json:"blocked"`
		} `json:"data"`
	}
	if err := sonic.Unmarshal(respBody, &rsp); err != nil {
		t.Fatalf("failed to unmarshal list response: %s", string(respBody))
	}
	for _, b := range rsp.Data.Blocked {
		if b.Word == word {
			return b.ID, true
		}
	}
	return 0, false
}

// updateBlockedAction 修改敏感词 action（管理接口，admin JWT）。
func updateBlockedAction(t *testing.T, baseURL, adminToken string, id uint, action string) {
	t.Helper()
	payload, err := sonic.Marshal(map[string]string{"action": action})
	if err != nil {
		t.Fatalf("failed to marshal update body: %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, fmt.Sprintf("%s/api/v1/block?id=%d", baseURL, id), strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to update blocked word: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("update blocked word status = %d, body: %s", resp.StatusCode, string(respBody))
	}
}

// deleteBlockedWord 删除敏感词（管理接口，admin JWT）。
func deleteBlockedWord(t *testing.T, baseURL, adminToken string, id uint) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, fmt.Sprintf("%s/api/v1/block?id=%d", baseURL, id), http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to delete blocked word: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("delete blocked word status = %d, body: %s", resp.StatusCode, string(respBody))
	}
}

// chatWithWord 发送一次非流式 chat completion，消息内容包含指定敏感词。
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

// uniqueWord 生成本次测试唯一的敏感词，避免污染环境中的存量数据。
func uniqueWord(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, os.Getpid())
}

// TestBlocked_Deny_Returns403 验证 deny 型敏感词命中时返回 403（现状行为回归）。
func TestBlocked_Deny_Returns403(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustBlockedE2EEnv(t)

	word := uniqueWord("e2edeny")
	id := createBlockedWord(t, baseURL, adminToken, word, "deny")
	defer deleteBlockedWord(t, baseURL, adminToken, id)

	resp := chatWithWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("deny word: expected 403, got %d, body: %s", resp.StatusCode, string(body))
	}
}

// TestBlocked_Allow_ForwardsToUpstream 验证 allow 型敏感词命中时请求照常转发（200 + 正常响应）。
func TestBlocked_Allow_ForwardsToUpstream(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustBlockedE2EEnv(t)

	word := uniqueWord("e2eallow")
	id := createBlockedWord(t, baseURL, adminToken, word, "allow")
	defer deleteBlockedWord(t, baseURL, adminToken, id)

	resp := chatWithWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("allow word: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read chat response: %v", err)
	}
	var obj map[string]any
	if err := sonic.Unmarshal(body, &obj); err != nil {
		t.Fatalf("allow word: response is not valid JSON: %s", string(body))
	}
	if obj["choices"] == nil {
		t.Fatalf("allow word: missing choices, body: %s", string(body))
	}
}

// TestBlocked_UpdateAction_Switches 验证 PATCH 修改 action 后行为切换（deny → 403、allow → 200）。
func TestBlocked_UpdateAction_Switches(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustBlockedE2EEnv(t)

	word := uniqueWord("e2eswitch")
	id := createBlockedWord(t, baseURL, adminToken, word, "deny")
	defer deleteBlockedWord(t, baseURL, adminToken, id)

	resp := chatWithWord(t, baseURL, apiKey, word)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("before update: expected 403, got %d", resp.StatusCode)
	}

	updateBlockedAction(t, baseURL, adminToken, id, "allow")

	resp = chatWithWord(t, baseURL, apiKey, word)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("after update to allow: expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
}

// TestBlocked_Mixed_DenyWins 验证同时命中 deny 与 allow 型词时 deny 优先（403）。
func TestBlocked_Mixed_DenyWins(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, adminToken := mustBlockedE2EEnv(t)

	denyWord := uniqueWord("e2emixdeny")
	allowWord := uniqueWord("e2emixallow")
	denyID := createBlockedWord(t, baseURL, adminToken, denyWord, "deny")
	allowID := createBlockedWord(t, baseURL, adminToken, allowWord, "allow")
	defer deleteBlockedWord(t, baseURL, adminToken, denyID)
	defer deleteBlockedWord(t, baseURL, adminToken, allowID)

	resp := chatWithWord(t, baseURL, apiKey, denyWord+" "+allowWord)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("mixed hit: expected 403 (deny wins), got %d, body: %s", resp.StatusCode, string(body))
	}
}
