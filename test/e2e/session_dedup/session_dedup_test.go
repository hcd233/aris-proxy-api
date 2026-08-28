// Package session_dedup 验证 session 前缀去重在插入后实时生效。
//
// 环境变量（缺省 skip）：
//   - BASE_URL   API 根地址
//   - API_KEY    代理密钥（OpenAI 协议）
//   - JWT_TOKEN  管理员 JWT（session/list）
//
// 流程：同一对话连续两轮 chat completions（轮次1消息带随机标记）→
// 轮次1快照应被轮次2快照实时取代 → 以标记为 keyword 轮询 session/list，
// 最终只剩 1 条且 messageCount == 轮次2消息数（含 assistant 回复）。
//
//	@author centonhuang
//	@update 2026-08-29 10:00:00
package session_dedup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const e2eHTTPTimeout = 90 * time.Second

type sessionSummary struct {
	ID           uint `json:"id"`
	MessageCount int  `json:"messageCount"`
}

type listSessionsRsp struct {
	Sessions []sessionSummary `json:"sessions"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

func mustE2EEnv(t *testing.T) (baseURL, apiKey, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	apiKey = os.Getenv("API_KEY")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || apiKey == "" || jwtToken == "" {
		t.Skip("BASE_URL, API_KEY and JWT_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), apiKey, jwtToken
}

// postOnce 发送一轮非流式 chat completions 并返回 assistant 回复内容
func postOnce(t *testing.T, baseURL, apiKey string, messages []chatMessage) string {
	t.Helper()
	body, err := sonic.Marshal(map[string]any{
		"model":      "gpt-5.5",
		"messages":   messages,
		"stream":     false,
		"max_tokens": 10,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		baseURL+"/api/openai/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: e2eHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send chat completion: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("chat completion status = %d, body: %s", resp.StatusCode, string(respBody))
	}
	var completion struct {
		Choices []chatChoice `json:"choices"`
	}
	if err := sonic.Unmarshal(respBody, &completion); err != nil {
		t.Fatalf("unmarshal completion: %v", err)
	}
	if len(completion.Choices) == 0 {
		t.Fatalf("no choices in response: %s", string(respBody))
	}
	return completion.Choices[0].Message.Content
}

// listSessionsByKeyword 以 keyword 过滤会话列表
func listSessionsByKeyword(t *testing.T, baseURL, jwtToken, keyword string) []sessionSummary {
	t.Helper()
	q := url.Values{}
	q.Set("page", "1")
	q.Set("pageSize", "20")
	q.Set("keyword", keyword)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		baseURL+"/api/web/v1/session/list?"+q.Encode(), http.NoBody)
	if err != nil {
		t.Fatalf("build list request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	client := &http.Client{Timeout: e2eHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send list request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read list response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session list status = %d, body: %s", resp.StatusCode, string(respBody))
	}
	var rsp listSessionsRsp
	if err := sonic.Unmarshal(respBody, &rsp); err != nil {
		t.Fatalf("unmarshal list rsp: %v", err)
	}
	return rsp.Sessions
}

func TestE2E_SessionPrefixDedup_RealtimeAfterInsert(t *testing.T) {
	t.Parallel()
	baseURL, apiKey, jwtToken := mustE2EEnv(t)
	marker := fmt.Sprintf("dedup-e2e-%d", time.Now().UnixNano())

	turn1 := []chatMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Reply with exactly: OK " + marker},
	}
	assistantContent := postOnce(t, baseURL, apiKey, turn1)

	turn2 := append(append([]chatMessage{}, turn1...),
		chatMessage{Role: "assistant", Content: assistantContent},
		chatMessage{Role: "user", Content: "Thanks " + marker},
	)
	postOnce(t, baseURL, apiKey, turn2)

	// store 消息 = 请求消息 + assistant 回复
	expectedCount := len(turn2) + 1

	// 去重完成后 keyword 只命中轮次2快照；轮次1快照（messageCount 更小）应已软删。
	// 用 ticker 轮询而非 time.Sleep（lintconv testing.sleep 规则）。
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	deadline := time.Now().Add(90 * time.Second)
	for {
		sessions := listSessionsByKeyword(t, baseURL, jwtToken, marker)
		if len(sessions) == 1 && sessions[0].MessageCount == expectedCount {
			t.Logf("deduped: single session id=%d messageCount=%d", sessions[0].ID, sessions[0].MessageCount)
			return
		}
		if time.Now().After(deadline) {
			counts := make([]int, 0, len(sessions))
			for _, s := range sessions {
				counts = append(counts, s.MessageCount)
			}
			t.Fatalf("dedup not settled within 90s: sessions=%d want=1, messageCounts=%v want=[%d]",
				len(sessions), counts, expectedCount)
		}
		<-ticker.C
	}
}
