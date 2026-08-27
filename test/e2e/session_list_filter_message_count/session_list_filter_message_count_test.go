// Package session_list_filter_message_count 验证 GET /api/web/v1/session/option/list?field=messageCount
// 与 GET /api/web/v1/session/list?filter=messageCount:min-max 的端到端行为。
//
// 环境变量：
//   - BASE_URL    API 根地址（必填）
//   - JWT_TOKEN   登录后的 JWT，含 user_id（必填）
package session_list_filter_message_count

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const httpTimeout = 30 * time.Second

var rangeRe = regexp.MustCompile(`^(\d+)-(\d+)$`)

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
		MessageCount int    `json:"messageCount"`
		CreatedAt    string `json:"createdAt"`
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
	q.Set("pageSize", "50")
	if filter != "" {
		q.Set("filter", filter)
	}
	endpoint := fmt.Sprintf("%s/api/web/v1/session/list?%s", baseURL, q.Encode())

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
	endpoint := fmt.Sprintf("%s/api/web/v1/session/option/list?%s", baseURL, q.Encode())

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

// parseRange 解析 "min-max" 区间值
func parseRange(t *testing.T, v string) (minVal, maxVal int) {
	t.Helper()
	m := rangeRe.FindStringSubmatch(v)
	if m == nil {
		t.Fatalf("invalid range option %q", v)
	}
	minVal, _ = strconv.Atoi(m[1])
	maxVal, _ = strconv.Atoi(m[2])
	if maxVal < minVal {
		t.Fatalf("range %q has max < min", v)
	}
	return minVal, maxVal
}

// TestSessionListFilterMessageCount_OptionList 验证 /api/web/v1/session/option/list?field=messageCount
// 返回 200 且 items 均为合法 "min-max" 区间。
//
//	@author centonhuang
//	@update 2026-08-12 16:00:00
func TestSessionListFilterMessageCount_OptionList(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	status, body := doListSessionOptions(t, client, baseURL, jwtToken, "messageCount")
	if status != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%+v", status, body)
	}
	if body.Error != nil {
		t.Fatalf("biz error: %+v", body.Error)
	}
	if body.Items == nil {
		t.Fatalf("items should not be nil")
	}
	for _, item := range body.Items {
		parseRange(t, item)
	}
}

// TestSessionListFilterMessageCount_NoCrash 验证消息数区间 filter 表达式不触发 500 且返回正常结构。
//
//	@author centonhuang
//	@update 2026-08-12 16:00:00
func TestSessionListFilterMessageCount_NoCrash(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	filters := []string{
		"messageCount:0-10",
		"messageCount:0-10|11-50",
		"messageCount:!0-10",
		"score:5 messageCount:11-50",
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

// TestSessionListFilterMessageCount_InvalidRange 验证非法区间值返回 400 而非 500。
//
//	@author centonhuang
//	@update 2026-08-12 16:00:00
func TestSessionListFilterMessageCount_InvalidRange(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	filters := []string{
		"messageCount:abc",
		"messageCount:10-5",
		"messageCount:0-10-20",
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
				t.Fatalf("filter=%q unified contract expected 200, got %d; body=%+v", f, status, body)
			}
			if body.Error == nil || body.Error.Code != 10006 {
				t.Fatalf("filter=%q expected error code 10006, body=%+v", f, body)
			}
		})
	}
}

// TestSessionListFilterMessageCount_FilterSemantics 从 options 取第一个区间过滤，验证返回
// 会话的消息数全部落在选中区间并集内（过滤语义正确性）。
//
//	@author centonhuang
//	@update 2026-08-12 16:00:00
func TestSessionListFilterMessageCount_FilterSemantics(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustEnv(t)
	client := newClient()

	status, optBody := doListSessionOptions(t, client, baseURL, jwtToken, "messageCount")
	if status != http.StatusOK || optBody.Error != nil || len(optBody.Items) == 0 {
		t.Skip("no message count options available in current range")
	}

	filter := "messageCount:" + strings.Join(optBody.Items, "|")
	status, body := doListSessions(t, client, baseURL, jwtToken, filter)
	if status != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%+v", status, body)
	}
	if body.Error != nil {
		t.Fatalf("biz error: %+v", body.Error)
	}

	// 计算所有选中区间的并集边界
	minBound, maxBound := int(^uint(0)>>1), -1
	for _, item := range optBody.Items {
		minV, maxV := parseRange(t, item)
		if minV < minBound {
			minBound = minV
		}
		if maxV > maxBound {
			maxBound = maxV
		}
	}

	for _, s := range body.Sessions {
		if s.MessageCount < minBound || s.MessageCount > maxBound {
			t.Errorf("session %d messageCount=%d outside selected ranges %v", s.ID, s.MessageCount, optBody.Items)
		}
	}
}
