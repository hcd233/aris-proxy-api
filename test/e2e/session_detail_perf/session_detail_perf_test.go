// Package session_detail_perf 验证 Session 详情接口性能优化的端到端行为：
//   - GET /api/v1/session/metadata：返回 messageCount/toolCount，不含 IDs 数组
//   - GET /api/v1/session/message/list：offset+limit 分页，返回 OffsetPageInfo
//   - GET /api/v1/session/tool/list：同上
//
// 环境变量：
//   - BASE_URL    API 根地址（必填）
//   - JWT_TOKEN   登录后的 JWT，含 user_id（必填）
//   - SESSION_ID  当前 JWT 所属用户拥有的 sessionId（必填，正整数）
package session_detail_perf

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const httpTimeout = 30 * time.Second

func mustEnv(t *testing.T) (baseURL, jwtToken string, sessionID uint) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	sessIDStr := os.Getenv("SESSION_ID")
	if baseURL == "" || jwtToken == "" || sessIDStr == "" {
		t.Skip("BASE_URL, JWT_TOKEN and SESSION_ID are required for e2e test")
	}
	id64, err := strconv.ParseUint(sessIDStr, 10, 64)
	if err != nil || id64 == 0 {
		t.Skip("SESSION_ID must be a positive integer")
	}
	return baseURL, jwtToken, uint(id64)
}

func newClient() *http.Client {
	return &http.Client{Timeout: httpTimeout}
}

func doGetJSON(t *testing.T, client *http.Client, url, jwt string, out any) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	rsp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer rsp.Body.Close()
	if out != nil {
		if err := sonic.ConfigDefault.NewDecoder(rsp.Body).Decode(out); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return rsp.StatusCode
}

func TestSessionDetailPerf_GetMetadata_Success(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/metadata?sessionId=%d", baseURL, sessID)
	var rsp struct {
		Error   *struct{ Code int } `json:"error"`
		Session *struct {
			ID           uint   `json:"id"`
			APIKeyName   string `json:"apiKeyName"`
			MessageCount int    `json:"messageCount"`
			ToolCount    int    `json:"toolCount"`
			ShareID      string `json:"shareID"`
		} `json:"session"`
	}
	status := doGetJSON(t, client, url, jwt, &rsp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if rsp.Error != nil {
		t.Fatalf("biz error: %+v", rsp.Error)
	}
	if rsp.Session == nil {
		t.Fatalf("session is nil")
	}
	if rsp.Session.ID != sessID {
		t.Errorf("session.id = %d, want %d", rsp.Session.ID, sessID)
	}
	if rsp.Session.MessageCount < 0 {
		t.Errorf("messageCount must be >= 0")
	}
}

func TestSessionDetailPerf_ListMessages_Pagination(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/message/list?sessionId=%d&offset=0&limit=10", baseURL, sessID)
	var rsp struct {
		Error    *struct{ Code int } `json:"error"`
		Messages []map[string]any    `json:"messages"`
		PageInfo *struct {
			Offset int   `json:"offset"`
			Limit  int   `json:"limit"`
			Total  int64 `json:"total"`
		} `json:"pageInfo"`
	}
	status := doGetJSON(t, client, url, jwt, &rsp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if rsp.Error != nil {
		t.Fatalf("biz error: %+v", rsp.Error)
	}
	if rsp.PageInfo == nil {
		t.Fatalf("pageInfo is nil")
	}
	if rsp.PageInfo.Offset != 0 || rsp.PageInfo.Limit != 10 {
		t.Errorf("pageInfo.offset=%d limit=%d, want 0/10", rsp.PageInfo.Offset, rsp.PageInfo.Limit)
	}
	if int64(len(rsp.Messages)) > rsp.PageInfo.Total {
		t.Errorf("messages len %d > total %d", len(rsp.Messages), rsp.PageInfo.Total)
	}
}

func TestSessionDetailPerf_ListMessages_LimitRejected(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/message/list?sessionId=%d&offset=0&limit=999", baseURL, sessID)
	status := doGetJSON(t, client, url, jwt, nil)
	if status == http.StatusOK {
		t.Errorf("expected 4xx for limit=999, got 200")
	}
}

func TestSessionDetailPerf_ListTools_Pagination(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/tool/list?sessionId=%d&offset=0&limit=10", baseURL, sessID)
	var rsp struct {
		Error    *struct{ Code int } `json:"error"`
		Tools    []map[string]any    `json:"tools"`
		PageInfo *struct {
			Offset int   `json:"offset"`
			Limit  int   `json:"limit"`
			Total  int64 `json:"total"`
		} `json:"pageInfo"`
	}
	status := doGetJSON(t, client, url, jwt, &rsp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if rsp.Error != nil {
		t.Fatalf("biz error: %+v", rsp.Error)
	}
	if rsp.PageInfo == nil {
		t.Fatalf("pageInfo is nil")
	}
}

func TestSessionDetailPerf_CacheConsistency(t *testing.T) {
	baseURL, jwt, sessID := mustEnv(t)
	client := newClient()

	url := fmt.Sprintf("%s/api/v1/session/metadata?sessionId=%d", baseURL, sessID)
	var first, second struct {
		Session *struct {
			ID           uint   `json:"id"`
			MessageCount int    `json:"messageCount"`
			ToolCount    int    `json:"toolCount"`
			APIKeyName   string `json:"apiKeyName"`
		} `json:"session"`
	}
	if status := doGetJSON(t, client, url, jwt, &first); status != http.StatusOK {
		t.Fatalf("first status = %d", status)
	}
	if status := doGetJSON(t, client, url, jwt, &second); status != http.StatusOK {
		t.Fatalf("second status = %d", status)
	}
	if first.Session == nil || second.Session == nil {
		t.Fatalf("nil session")
	}
	if first.Session.MessageCount != second.Session.MessageCount {
		t.Errorf("messageCount inconsistent: first=%d second=%d", first.Session.MessageCount, second.Session.MessageCount)
	}
	if first.Session.APIKeyName != second.Session.APIKeyName {
		t.Errorf("apiKeyName inconsistent")
	}
}
