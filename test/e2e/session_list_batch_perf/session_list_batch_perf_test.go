// Package session_list_batch_perf 验证 GET /api/v1/session/list 在
// 「空 summary fallback」场景下完成时间显著低于 6.1s 基准（traceID
// efe54869-d52d-4375-9ea4-366b26923283）。
//
// 回归背景（refactor/session-list-batch-perf-2026-06-07）：
//   - 旧实现 loadMessagesForEmptySummaries → BatchGetByField → FindInBatches(500)
//     对 ~12000 个 message IDs 发出 24 次顺序
//     SELECT ... IN (501 ids) AND id > last_id LIMIT 500，单接口总耗时 6.1s。
//   - 修复后：session_read_repository.FindMessagesByIDsChunked 把 IDs
//     排序去重后按 5000/块切分，3 条单次 SELECT ... WHERE id IN (?) 即可。
//
// 本测试只对"接口是否仍 200 + 不崩溃 + 响应时间回到合理量级"做粗粒度断言：
//  1. pageSize=200 + 大量空 summary session 时，接口必须 200 返回；
//  2. 响应耗时必须显著低于旧实现（>3s 视为回归）。
//  3. 响应结构与后端 ListSessionsRsp 一致，sessions 数组按 summary 字段可为空。
//
// 环境变量：
//   - BASE_URL    API 根地址（必填）
//   - JWT_TOKEN   登录后的 JWT，含 user_id（必填）
package session_list_batch_perf

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
	latencyBudgetOld = 6100 * time.Millisecond
	latencyBudgetNew = 1000 * time.Millisecond
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

func doListSessions(t *testing.T, client *http.Client, baseURL, jwtToken string, pageSize int) (int, time.Duration, *listSessionsRsp) {
	t.Helper()
	q := url.Values{}
	q.Set("page", "1")
	q.Set("pageSize", fmt.Sprintf("%d", pageSize))
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

// TestSessionListBatchPerf_NoCrashAndFast 验证 pageSize=200 时接口不崩溃且
// 显著低于旧实现的 6.1s 基准。
//
//	@author centonhuang
//	@update 2026-06-07 01:35:00
func TestSessionListBatchPerf_NoCrashAndFast(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	status, elapsed, body := doListSessions(t, client, baseURL, jwtToken, 200)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%+v", status, body)
	}
	if body.Error != nil {
		t.Fatalf("biz error: %+v", body.Error)
	}
	if body.PageInfo == nil {
		t.Fatalf("missing pageInfo: %+v", body)
	}
	t.Logf("pageSize=200 elapsed=%v pageInfo=%+v sessionCount=%d",
		elapsed, body.PageInfo, len(body.Sessions))

	if elapsed >= latencyBudgetNew {
		t.Errorf("elapsed=%v exceeds new latency budget %v (old baseline was %v). performance regression detected",
			elapsed, latencyBudgetNew, latencyBudgetOld)
	}
}

// TestSessionListBatchPerf_PageSize20StillFast 验证小分页耗时极小。
//
//	@author centonhuang
//	@update 2026-06-07 01:35:00
func TestSessionListBatchPerf_PageSize20StillFast(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	status, elapsed, body := doListSessions(t, client, baseURL, jwtToken, 20)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%+v", status, body)
	}
	if body.Error != nil {
		t.Fatalf("biz error: %+v", body.Error)
	}
	if body.PageInfo == nil {
		t.Fatalf("missing pageInfo: %+v", body)
	}
	t.Logf("pageSize=20 elapsed=%v pageInfo=%+v sessionCount=%d",
		elapsed, body.PageInfo, len(body.Sessions))

	if elapsed >= 1500*time.Millisecond {
		t.Errorf("pageSize=20 elapsed=%v exceeds 1.5s budget", elapsed)
	}
}
