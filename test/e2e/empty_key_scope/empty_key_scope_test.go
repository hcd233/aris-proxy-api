// Package empty_key_scope 端到端回归：名下无 API Key 的普通用户不得越权
// 查看全平台数据（2026-08-25 越权修复）。
//
// 背景：audit 图表（6 个 stats 接口）、trace 列表、dataset 导出预览的仓储层
// 旧实现用 `if len(scope) > 0` 决定是否加范围过滤——无 Key 用户拿到的空列表
// 使过滤被整体跳过，退化为全平台查询。
//
// 运行前提：BASE_URL + USER_TOKEN（名下无 API Key 的普通用户 token）；
// 可选 ADMIN_TOKEN 用于对照验证（admin 能查到数据时，user 必须为空；
// admin 也查不到数据则 Skip，避免环境无数据造成假绿）。
package empty_key_scope

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

const e2eHTTPTimeout = 30 * time.Second

// statsReqParams 覆盖近一年时间范围，保证环境有数据时越权必然可见
const statsReqParams = "startTime=2025-09-01T00:00:00Z&endTime=2026-09-01T00:00:00Z&granularity=day"

func mustEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skip(key + " is required for e2e test")
	}
	return v
}

func doGetJSON(t *testing.T, client *http.Client, target, token string) (status int, body []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	return resp.StatusCode, body
}

func bizErrorCode(body []byte) int {
	var result struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(body, &result); err != nil {
		return -1
	}
	if result.Error == nil {
		return 0
	}
	return result.Error.Code
}

// TestEmptyKeyUser_SeesNoPlatformData 无 Key 用户对所有按 owner 维度隔离的
// 读接口必须拿到空数据，而不是全平台聚合结果。
func TestEmptyKeyUser_SeesNoPlatformData(t *testing.T) {
	t.Parallel()

	baseURL := strings.TrimRight(mustEnv(t, "BASE_URL"), "/")
	userToken := mustEnv(t, "USER_TOKEN")
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 对照（可选）：admin 查近一年模型趋势，有数据才证明环境可验证隔离
	if adminToken := os.Getenv("ADMIN_TOKEN"); adminToken != "" {
		status, body := doGetJSON(t, client,
			baseURL+"/api/v1/audit/stats/model/trend?"+statsReqParams, adminToken)
		if status != http.StatusOK {
			t.Fatalf("admin model trend expected 200, got %d: %s", status, body)
		}
		var adminRsp struct {
			Data []any `json:"data"`
		}
		if err := sonic.Unmarshal(body, &adminRsp); err != nil {
			t.Fatalf("unmarshal admin trend failed: %v", err)
		}
		if len(adminRsp.Data) == 0 {
			t.Skip("admin sees no data in range; environment cannot verify isolation")
		}
	}

	// 1. audit 六个图表接口：无 Key 用户必须拿到空 data
	statsPaths := []string{
		"/api/v1/audit/stats/model/trend",
		"/api/v1/audit/stats/request/rate",
		"/api/v1/audit/stats/token/throughput",
		"/api/v1/audit/stats/token/rate",
		"/api/v1/audit/stats/model/usage",
		"/api/v1/audit/stats/token/latency",
	}
	for _, p := range statsPaths {
		status, body := doGetJSON(t, client, baseURL+p+"?"+statsReqParams, userToken)
		if status != http.StatusOK {
			t.Errorf("%s expected 200, got %d: %s", p, status, body)
			continue
		}
		if code := bizErrorCode(body); code != 0 {
			t.Errorf("%s unexpected biz error code %d: %s", p, code, body)
			continue
		}
		var rsp struct {
			Data []any `json:"data"`
		}
		if err := sonic.Unmarshal(body, &rsp); err != nil {
			t.Errorf("%s unmarshal failed: %v", p, err)
			continue
		}
		if len(rsp.Data) != 0 {
			t.Errorf("%s: no-key user sees platform data (len=%d), scope isolation broken", p, len(rsp.Data))
		}
	}

	// 2. trace 列表：无 Key 用户必须拿到空列表
	status, body := doGetJSON(t, client,
		baseURL+"/api/v1/trace/list?page=1&pageSize=10", userToken)
	if status != http.StatusOK {
		t.Fatalf("trace list expected 200, got %d: %s", status, body)
	}
	if code := bizErrorCode(body); code != 0 {
		t.Fatalf("trace list unexpected biz error code %d: %s", code, body)
	}
	var traceRsp struct {
		Traces []any `json:"traces"`
	}
	if err := sonic.Unmarshal(body, &traceRsp); err != nil {
		t.Fatalf("unmarshal trace list failed: %v", err)
	}
	if len(traceRsp.Traces) != 0 {
		t.Errorf("trace list: no-key user sees platform traces (len=%d), scope isolation broken", len(traceRsp.Traces))
	}

	// 3. dataset 导出统计预览：无 Key 用户必须拿到 0 会话
	previewQuery := url.Values{}
	previewQuery.Set("startTime", "2025-09-01T00:00:00Z")
	previewQuery.Set("endTime", "2026-09-01T00:00:00Z")
	status, body = doGetJSON(t, client,
		baseURL+"/api/v1/dataset/preview?"+previewQuery.Encode(), userToken)
	if status != http.StatusOK {
		t.Fatalf("dataset preview expected 200, got %d: %s", status, body)
	}
	if code := bizErrorCode(body); code != 0 {
		t.Fatalf("dataset preview unexpected biz error code %d: %s", code, body)
	}
	var previewRsp struct {
		TotalSessions int `json:"totalSessions"`
	}
	if err := sonic.Unmarshal(body, &previewRsp); err != nil {
		t.Fatalf("unmarshal dataset preview failed: %v", err)
	}
	if previewRsp.TotalSessions != 0 {
		t.Errorf("dataset preview: no-key user sees %d platform sessions, scope isolation broken", previewRsp.TotalSessions)
	}
}
