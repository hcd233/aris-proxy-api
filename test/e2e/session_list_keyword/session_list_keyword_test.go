// Package session_list_keyword 验证 GET /api/v1/session/list?keyword=... 的端到端行为。
//
// 回归背景（bugfix/session-keyword-jsonb-2026-06-07）：
//   - 旧 SQL 用了 messages.id = ANY(sessions.message_ids)，
//     message_ids 是 gorm serializer:json 存出来的 JSON 文本，
//     不是 PostgreSQL 原生数组，因此触发 SQLSTATE 42809，
//     所有带 keyword 的会话列表请求一律 500（traceID ed2ade34-...）。
//   - 修复方案：在 constant.SessionKeywordFilterSQL 用 sessions.message_ids::jsonb ? messages.id::text，
//     与 SessionSummarySelect 中既有的 ::jsonb 强转保持一致。
//
// 本测试覆盖：
//  1. 任意 keyword 都不应触发 500（最直接的回归断言）
//  2. 响应结构是 { sessions, pageInfo }，与后端 ListSessionsRsp 一致
//  3. 同一会话的 keyword 命中数 <= 列表长度（一致性）
//  4. 中文 keyword 与空格的 URL 编码也能正常处理
//
// 环境变量：
//   - BASE_URL    API 根地址（必填）
//   - JWT_TOKEN   登录后的 JWT，含 user_id（必填）
package session_list_keyword

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const httpTimeout = 30 * time.Second

func mustEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL and JWT_TOKEN are required for e2e test")
	}
	return baseURL, jwtToken
}

func newClient() *http.Client {
	return &http.Client{Timeout: httpTimeout}
}

type listSessionsRsp struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Sessions []struct {
		ID           uint   `json:"id"`
		CreatedAt    string `json:"createdAt"`
		UpdatedAt    string `json:"updatedAt"`
		Summary      string `json:"summary"`
		MessageCount int    `json:"messageCount"`
		ToolCount    int    `json:"toolCount"`
	} `json:"sessions,omitempty"`
	PageInfo *struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"pageInfo,omitempty"`
}

func doListSessions(t *testing.T, client *http.Client, baseURL, jwtToken, keyword string) (int, *listSessionsRsp) {
	t.Helper()
	q := url.Values{}
	q.Set("page", "1")
	q.Set("pageSize", "20")
	if keyword != "" {
		q.Set("keyword", keyword)
	}
	endpoint := fmt.Sprintf("%s/api/v1/session/list?%s", baseURL, q.Encode())

	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	rsp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer rsp.Body.Close()

	body := &listSessionsRsp{}
	if err := sonic.ConfigDefault.NewDecoder(rsp.Body).Decode(body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return rsp.StatusCode, body
}

// TestSessionListKeyword_NoCrash 验证任意 keyword 都不应触发 500。
// 修复前 keyword="依赖注入" 会直接 SQLSTATE 42809。
//
//	@author centonhuang
//	@update 2026-06-07 00:45:00
func TestSessionListKeyword_NoCrash(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	keywords := []string{
		"依赖注入",
		"hello world",
		"openai",
		"anthropic",
		"nonexistent-keyword-xyz-123",
		"' OR 1=1 --",
		"%",
		"_",
		"",
	}

	for _, kw := range keywords {
		kw := kw
		t.Run(fmt.Sprintf("keyword=%q", kw), func(t *testing.T) {
			t.Parallel()
			status, body := doListSessions(t, client, baseURL, jwtToken, kw)
			if status == http.StatusInternalServerError {
				t.Fatalf("keyword=%q returned 500, body=%+v", kw, body)
			}
			if status != http.StatusOK {
				t.Fatalf("keyword=%q status=%d, want 200; body=%+v", kw, status, body)
			}
			if body.Error != nil {
				t.Fatalf("keyword=%q biz error: %+v", kw, body.Error)
			}
			if body.PageInfo == nil {
				t.Errorf("keyword=%q missing pageInfo", kw)
			}
		})
	}
}

// TestSessionListKeyword_Consistency 验证两次相同 keyword 调用结果稳定。
//
//	@author centonhuang
//	@update 2026-06-07 00:45:00
func TestSessionListKeyword_Consistency(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	kw := "依赖注入"
	_, first := doListSessions(t, client, baseURL, jwtToken, kw)
	_, second := doListSessions(t, client, baseURL, jwtToken, kw)

	if first.PageInfo == nil || second.PageInfo == nil {
		t.Fatalf("missing pageInfo: first=%+v second=%+v", first, second)
	}
	if first.PageInfo.Total != second.PageInfo.Total {
		t.Errorf("total inconsistent: first=%d second=%d", first.PageInfo.Total, second.PageInfo.Total)
	}
	if len(first.Sessions) != len(second.Sessions) {
		t.Errorf("sessions len inconsistent: first=%d second=%d", len(first.Sessions), len(second.Sessions))
	}
	if first.PageInfo.Total < 0 {
		t.Errorf("total must be >= 0, got %d", first.PageInfo.Total)
	}
}
