// Package session_list_keyword_perf 验证 GET /api/v1/session/list?keyword=...
// 在带搜索关键词时仍然满足时延预算。
//
// 回归背景（feature/session-keyword-trgm-perf-2026-06-07）：
//   - 旧实现 GET /api/v1/session/list?keyword=xxx 在 messages 表体量增长后
//     单接口 6s+（"messages.message::text ILIKE '%kw%'" 是顺序扫描）。
//   - 修复方案：在 database migrate 阶段建 pg_trgm + 两类 GIN 索引
//     （idx_messages_message_trgm / idx_sessions_message_ids_gin），
//     ILIKE 退化为 trigram bitmap 扫描。
//
// 本测试只对"接口是否仍 200 + 不崩溃 + 响应时间回到合理量级"做粗粒度断言：
//  1. keyword 命中 + 不命中两种 case 都必须 200 返回；
//  2. 响应耗时显著低于顺序扫描基线（>2.5s 视为回归）；
//  3. 响应结构与后端 ListSessionsRsp 一致。
//
// 环境变量：
//   - BASE_URL    API 根地址（必填）
//   - JWT_TOKEN   登录后的 JWT，含 user_id（必填）
package session_list_keyword_perf

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

const (
	httpTimeout      = 30 * time.Second
	latencyBudgetOld = 6000 * time.Millisecond
	latencyBudgetNew = 2500 * time.Millisecond
)

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

func doListSessions(t *testing.T, client *http.Client, baseURL, jwtToken, keyword string, pageSize int) (int, time.Duration, *listSessionsRsp) {
	t.Helper()
	q := url.Values{}
	q.Set("page", "1")
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
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

	start := time.Now()
	rsp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer rsp.Body.Close()

	body := &listSessionsRsp{}
	if err := sonic.ConfigDefault.NewDecoder(rsp.Body).Decode(body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return rsp.StatusCode, elapsed, body
}

// TestSessionListKeywordPerf_HitNoCrashAndFast 验证 keyword 命中场景
// pageSize=200 时接口不崩溃且显著低于顺序扫描基线。
//
//	@author centonhuang
//	@update 2026-06-07 02:00:00
func TestSessionListKeywordPerf_HitNoCrashAndFast(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	status, elapsed, body := doListSessions(t, client, baseURL, jwtToken, "依赖注入", 200)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%+v", status, body)
	}
	if body.Error != nil {
		t.Fatalf("biz error: %+v", body.Error)
	}
	if body.PageInfo == nil {
		t.Fatalf("missing pageInfo: %+v", body)
	}
	t.Logf("keyword=依赖注入 pageSize=200 elapsed=%v pageInfo=%+v sessionCount=%d",
		elapsed, body.PageInfo, len(body.Sessions))

	if elapsed >= latencyBudgetNew {
		t.Errorf("elapsed=%v exceeds new latency budget %v (old baseline was %v). "+
			"regression: pg_trgm / GIN index likely not created on the production DB "+
			"or query plan is not picking it up",
			elapsed, latencyBudgetNew, latencyBudgetOld)
	}
}

// TestSessionListKeywordPerf_MissNoCrashAndFast 验证 keyword 不命中场景
// 也满足预算，确保 NOT-INDEXABLE 的 case 不会因为 GIN 索引退化而变慢。
//
//	@author centonhuang
//	@update 2026-06-07 02:00:00
func TestSessionListKeywordPerf_MissNoCrashAndFast(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	status, elapsed, body := doListSessions(t, client, baseURL, jwtToken, "nonexistent-keyword-xyz-123", 200)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%+v", status, body)
	}
	if body.Error != nil {
		t.Fatalf("biz error: %+v", body.Error)
	}
	if body.PageInfo == nil {
		t.Fatalf("missing pageInfo: %+v", body)
	}
	t.Logf("keyword=nonexistent-keyword-xyz-123 pageSize=200 elapsed=%v pageInfo=%+v sessionCount=%d",
		elapsed, body.PageInfo, len(body.Sessions))

	if elapsed >= latencyBudgetNew {
		t.Errorf("elapsed=%v exceeds new latency budget %v (old baseline was %v)",
			elapsed, latencyBudgetNew, latencyBudgetOld)
	}
}
