// Package session_list_filter_model 验证 GET /api/v1/session/list?filter=model:... 的端到端行为。
//
// 环境变量：
//   - BASE_URL    API 根地址（必填）
//   - JWT_TOKEN   登录后的 JWT，含 user_id（必填）
package session_list_filter_model

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
		ID           uint     `json:"id"`
		CreatedAt    string   `json:"createdAt"`
		UpdatedAt    string   `json:"updatedAt"`
		Summary      string   `json:"summary"`
		MessageCount int      `json:"messageCount"`
		ToolCount    int      `json:"toolCount"`
		Models       []string `json:"models,omitempty"`
	} `json:"sessions,omitempty"`
	PageInfo *struct {
		Page     int   `json:"page"`
		PageSize int   `json:"pageSize"`
		Total    int64 `json:"total"`
	} `json:"pageInfo,omitempty"`
}

func doListSessions(t *testing.T, client *http.Client, baseURL, jwtToken, filter string) (int, *listSessionsRsp) {
	t.Helper()
	q := url.Values{}
	q.Set("page", "1")
	q.Set("pageSize", "20")
	if filter != "" {
		q.Set("filter", filter)
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

type optionListRsp struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Items []string `json:"items"`
}

func doListSessionOptions(t *testing.T, client *http.Client, baseURL, jwtToken, field string) (int, *optionListRsp) {
	t.Helper()
	q := url.Values{}
	q.Set("field", field)
	endpoint := fmt.Sprintf("%s/api/v1/session/option/list?%s", baseURL, q.Encode())

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

	body := &optionListRsp{}
	if err := sonic.ConfigDefault.NewDecoder(rsp.Body).Decode(body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return rsp.StatusCode, body
}

// TestSessionListFilterModel_NoCrash 验证各种 model filter 表达式不触发 500 且返回正常结构。
//
//	@author centonhuang
//	@update 2026-06-16 15:00:00
func TestSessionListFilterModel_NoCrash(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	filters := []string{
		"model:gpt-4o",
		"model:gpt-4o|claude-3-5-sonnet",
		"model:!gpt-4o",
		"score:5 model:gpt-4o",
		"model:'gpt-4o'",
	}

	for _, f := range filters {
		f := f
		t.Run(fmt.Sprintf("filter=%q", f), func(t *testing.T) {
			t.Parallel()
			status, body := doListSessions(t, client, baseURL, jwtToken, f)
			if status == http.StatusInternalServerError {
				t.Fatalf("filter=%q returned 500, body=%+v", f, body)
			}
			if status != http.StatusOK {
				t.Fatalf("filter=%q status=%d, want 200; body=%+v", f, status, body)
			}
			if body.Error != nil {
				t.Fatalf("filter=%q biz error: %+v", f, body.Error)
			}
			if body.PageInfo == nil {
				t.Errorf("filter=%q missing pageInfo", f)
			}
		})
	}
}

// TestSessionListFilterModel_OptionList 验证 /api/v1/session/option/list?field=model 返回正常。
//
//	@author centonhuang
//	@update 2026-06-16 15:00:00
func TestSessionListFilterModel_OptionList(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	status, body := doListSessionOptions(t, client, baseURL, jwtToken, "model")
	if status != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%+v", status, body)
	}
	if body.Error != nil {
		t.Fatalf("biz error: %+v", body.Error)
	}
	if body.Items == nil {
		t.Errorf("items should not be nil")
	}
}
