// Package empty_key_scope 端到端回归：名下无 API Key 的普通用户不得越权
// 查看全平台数据（2026-08-25 越权修复）。
//
// 背景：audit 图表（6 个 stats 接口）、trace 列表、dataset 导出预览的仓储层
// 旧实现用 `if len(scope) > 0` 决定是否加范围过滤——无 Key 用户拿到的空列表
// 使过滤被整体跳过，退化为全平台查询。
//
// 运行前提：BASE_URL + USER_TOKEN（名下无 API Key 的普通用户 token）+ ADMIN_TOKEN；
// ADMIN_TOKEN 用于对照验证（admin 能查到数据时，user 必须为空；admin 也查不到数据
// 则 Skip，避免环境无数据造成假绿）。对照是必需项：缺省环境下「user 看到空」是
// 恒真断言，没有对照就无法证明隔离真的生效，故缺 ADMIN_TOKEN 时直接 Skip。
package empty_key_scope

import (
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

const e2eHTTPTimeout = 30 * time.Second

// timeWindow 近一年窗口，相对当前时间动态生成。
// 固定日期会随时间推移滑出数据范围，窗口内无数据时越权面观测不到。
func timeWindow() (start, end string) {
	now := time.Now().UTC()
	return now.AddDate(-1, 0, 0).Format(time.RFC3339), now.Format(time.RFC3339)
}

// statsReqParams 覆盖近一年时间范围，保证环境有数据时越权必然可见
func statsReqParams() string {
	start, end := timeWindow()
	return fmt.Sprintf("startTime=%s&endTime=%s&granularity=day", start, end)
}

// optionReqParams 同窗口的时间过滤参数（option 接口不带 granularity）
func optionReqParams() string {
	start, end := timeWindow()
	return fmt.Sprintf("startTime=%s&endTime=%s", start, end)
}

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
	// 对照 token 必需：环境无数据时「user 看到空」恒真，必须由 admin 对照
	// 证明环境可验证隔离，否则 Skip（而不是静默“通过”）
	adminToken := mustEnv(t, "ADMIN_TOKEN")
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 对照：admin 查近一年模型趋势，有数据才证明环境可验证隔离
	status, body := doGetJSON(t, client,
		baseURL+"/api/web/v1/audit/stats/model/trend?"+statsReqParams(), adminToken)
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

	// 1. audit 六个图表接口：无 Key 用户必须拿到空 data
	statsPaths := []string{
		"/api/web/v1/audit/stats/model/trend",
		"/api/web/v1/audit/stats/request/rate",
		"/api/web/v1/audit/stats/token/throughput",
		"/api/web/v1/audit/stats/token/rate",
		"/api/web/v1/audit/stats/model/usage",
		"/api/web/v1/audit/stats/token/latency",
	}
	for _, p := range statsPaths {
		status, body := doGetJSON(t, client, baseURL+p+"?"+statsReqParams(), userToken)
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
	status, body = doGetJSON(t, client,
		baseURL+"/api/web/v1/trace/list?page=1&pageSize=10", userToken)
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
	start, end := timeWindow()
	previewQuery := url.Values{}
	previewQuery.Set("startTime", start)
	previewQuery.Set("endTime", end)
	status, body = doGetJSON(t, client,
		baseURL+"/api/web/v1/dataset/preview?"+previewQuery.Encode(), userToken)
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

	// 4. 筛选选项接口（2026-08-26 修复）：此前对普通用户返回全平台维度
	// （用户名/邮箱等），与列表接口的 owner 隔离语义不一致；现在无 Key 用户
	// 必须拿到空选项列表。
	optionPaths := []string{
		"/api/web/v1/audit/model/option/list?field=user&" + optionReqParams(),
		"/api/web/v1/audit/model/option/list?field=model&" + optionReqParams(),
		"/api/web/v1/session/option/list?field=model&" + optionReqParams(),
	}
	for _, p := range optionPaths {
		status, body := doGetJSON(t, client, baseURL+p, userToken)
		if status != http.StatusOK {
			t.Errorf("%s expected 200, got %d: %s", p, status, body)
			continue
		}
		if code := bizErrorCode(body); code != 0 {
			t.Errorf("%s unexpected biz error code %d: %s", p, code, body)
			continue
		}
		var rsp struct {
			Items []any `json:"items"`
		}
		if err := sonic.Unmarshal(body, &rsp); err != nil {
			t.Errorf("%s unmarshal failed: %v", p, err)
			continue
		}
		if len(rsp.Items) != 0 {
			t.Errorf("%s: no-key user sees platform-wide filter options (len=%d), option scope broken", p, len(rsp.Items))
		}
	}
}
