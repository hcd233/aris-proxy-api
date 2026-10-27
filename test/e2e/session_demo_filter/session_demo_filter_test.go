// Package session_demo_filter 验证 demo 视角的会话筛选真正生效（回归：
// 修复前 demo 分支忽略 filter / keyword / 时间范围，选什么都没变化）。
//
// 环境变量：
//   - BASE_URL         API 根地址（必填）
//   - DEMO_JWT_TOKEN   demo 账户的 JWT（必填；用登录页「以 Demo 账户继续」获取）
package session_demo_filter

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

func mustEnv(t *testing.T) (baseURL, jwt string) {
	t.Helper()

	baseURL = os.Getenv("BASE_URL")
	jwt = os.Getenv("DEMO_JWT_TOKEN")
	if baseURL == "" || jwt == "" {
		t.Skip("BASE_URL and DEMO_JWT_TOKEN are required for e2e test")
	}
	return baseURL, jwt
}

type listRsp struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Sessions []struct {
		ID    uint `json:"id"`
		Score *int `json:"score"`
	} `json:"sessions,omitempty"`
	PageInfo *struct {
		Total int64 `json:"total"`
	} `json:"pageInfo,omitempty"`
}

func listDemoSessions(t *testing.T, baseURL, jwt, query string) listRsp {
	t.Helper()

	target := fmt.Sprintf("%s/api/web/v1/session/list?page=1&pageSize=20%s", baseURL, query)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)

	rsp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		t.Fatalf("list demo sessions: %v", err)
	}
	defer func() { _ = rsp.Body.Close() }()

	var out listRsp
	if err := sonic.ConfigDefault.NewDecoder(rsp.Body).Decode(&out); err != nil {
		t.Fatalf("decode list rsp: %v", err)
	}
	if rsp.StatusCode != http.StatusOK {
		t.Fatalf("list demo sessions status=%d err=%+v", rsp.StatusCode, out.Error)
	}
	return out
}

// TestDemoSessionFiltersApply demo 视角下时间范围与关键词必须生效。
func TestDemoSessionFiltersApply(t *testing.T) {
	t.Parallel()

	baseURL, jwt := mustEnv(t)

	baseline := listDemoSessions(t, baseURL, jwt, "")
	if baseline.PageInfo == nil {
		t.Fatal("baseline response has no pageInfo")
	}

	// 未来时间窗：任何真实会话都不该落在里面（修复前会被忽略而返回全量）
	future := time.Now().UTC().Add(200 * 24 * time.Hour)
	window := fmt.Sprintf("&startTime=%s&endTime=%s",
		url.QueryEscape(future.Format(time.RFC3339)),
		url.QueryEscape(future.Add(24*time.Hour).Format(time.RFC3339)))
	if rsp := listDemoSessions(t, baseURL, jwt, window); len(rsp.Sessions) != 0 {
		t.Fatalf("future time window returned %d sessions, want 0", len(rsp.Sessions))
	}

	// 不可能命中的关键词：必须返回空集（修复前会被忽略）
	if rsp := listDemoSessions(t, baseURL, jwt, "&keyword=__no_such_session_keyword__"); len(rsp.Sessions) != 0 {
		t.Fatalf("unmatched keyword returned %d sessions, want 0", len(rsp.Sessions))
	}

	// 评分筛选：结果集只能包含该分数
	scored := listDemoSessions(t, baseURL, jwt, "&filter="+url.QueryEscape("score:5"))
	for _, s := range scored.Sessions {
		if s.Score == nil || *s.Score != 5 {
			t.Fatalf("score:5 filter returned session %d with score %v", s.ID, s.Score)
		}
	}
	if scored.PageInfo != nil && baseline.PageInfo != nil && scored.PageInfo.Total > baseline.PageInfo.Total {
		t.Fatalf("filtered total %d exceeds baseline %d", scored.PageInfo.Total, baseline.PageInfo.Total)
	}
}
