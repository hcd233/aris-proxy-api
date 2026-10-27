// Package agent_harness 验证 agent harness 识别链路的端到端契约：
// 规则 CRUD → UA 预览 → 会话筛选 → 调用量走势。
//
// 环境变量：
//   - BASE_URL    API 根地址（必填）
//   - JWT_TOKEN   admin 账号的 JWT（必填；规则管理仅 admin 可用）
package agent_harness

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
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

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ruleItem struct {
	ID      uint   `json:"id"`
	Pattern string `json:"pattern"`
	Name    string `json:"name"`
	Sort    int    `json:"sort"`
}

type listRulesRsp struct {
	Error *apiError   `json:"error,omitempty"`
	Rules []*ruleItem `json:"rules,omitempty"`
}

type ruleRsp struct {
	Error *apiError `json:"error,omitempty"`
	Rule  *ruleItem `json:"rule,omitempty"`
}

type previewRsp struct {
	Error   *apiError `json:"error,omitempty"`
	Harness string    `json:"harness"`
	Rule    *ruleItem `json:"rule,omitempty"`
}

func doJSON(t *testing.T, httpClient *http.Client, method, target, jwt, body string, out any) int {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, target, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rsp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer func() { _ = rsp.Body.Close() }()

	if out != nil {
		if err := sonic.ConfigDefault.NewDecoder(rsp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s %s: %v", method, target, err)
		}
	}
	return rsp.StatusCode
}

// TestHarnessRuleLifecycle 新增 → 列表 → 预览 → 改名 → 删除。
func TestHarnessRuleLifecycle(t *testing.T) {
	t.Parallel()

	baseURL, jwt := mustEnv(t)
	httpClient := &http.Client{Timeout: httpTimeout}

	pattern := fmt.Sprintf("harness-e2e-%d/", time.Now().UnixNano())
	name := "E2EHarness"

	// 新增
	var created ruleRsp
	status := doJSON(t, httpClient, http.MethodPost, baseURL+"/api/web/v1/harness/rule", jwt,
		fmt.Sprintf(`{"pattern":%q,"name":%q,"sort":900}`, pattern, name), &created)
	if status != http.StatusOK || created.Rule == nil {
		t.Fatalf("create rule: status=%d err=%+v", status, created.Error)
	}
	defer func() {
		doJSON(t, httpClient, http.MethodDelete,
			fmt.Sprintf("%s/api/web/v1/harness/rule?id=%d", baseURL, created.Rule.ID), jwt, "", nil)
	}()

	// 列表包含新规则
	var listed listRulesRsp
	if status := doJSON(t, httpClient, http.MethodGet, baseURL+"/api/web/v1/harness/rule/list", jwt, "", &listed); status != http.StatusOK {
		t.Fatalf("list rules: status=%d err=%+v", status, listed.Error)
	}
	found := false
	for _, rule := range listed.Rules {
		if rule.ID == created.Rule.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("created rule %d not present in list", created.Rule.ID)
	}

	// 预览命中该规则
	var preview previewRsp
	ua := pattern + "1.0 test"
	if status := doJSON(t, httpClient, http.MethodPost, baseURL+"/api/web/v1/harness/rule/preview", jwt,
		fmt.Sprintf(`{"ua":%q}`, ua), &preview); status != http.StatusOK {
		t.Fatalf("preview: status=%d err=%+v", status, preview.Error)
	}
	if preview.Harness != name || preview.Rule == nil || preview.Rule.Pattern != pattern {
		t.Fatalf("preview = %+v, want harness %q via %q", preview, name, pattern)
	}

	// 改名后同一条 UA 立即解析成新名字（规则即时生效，无需发版）
	renamed := name + "V2"
	var updated ruleRsp
	if status := doJSON(t, httpClient, http.MethodPatch,
		fmt.Sprintf("%s/api/web/v1/harness/rule?id=%d", baseURL, created.Rule.ID), jwt,
		fmt.Sprintf(`{"pattern":%q,"name":%q,"sort":900}`, pattern, renamed), &updated); status != http.StatusOK {
		t.Fatalf("update rule: status=%d err=%+v", status, updated.Error)
	}
	var preview2 previewRsp
	if status := doJSON(t, httpClient, http.MethodPost, baseURL+"/api/web/v1/harness/rule/preview", jwt,
		fmt.Sprintf(`{"ua":%q}`, ua), &preview2); status != http.StatusOK {
		t.Fatalf("preview after rename: status=%d err=%+v", status, preview2.Error)
	}
	if preview2.Harness != renamed {
		t.Fatalf("preview after rename = %q, want %q", preview2.Harness, renamed)
	}
}

// TestHarnessSessionFilterContract 会话筛选的契约：未知 harness 命中空集，
// Other 与既有筛选一样可用且每条结果都带 agentHarness。
func TestHarnessSessionFilterContract(t *testing.T) {
	t.Parallel()

	baseURL, jwt := mustEnv(t)
	httpClient := &http.Client{Timeout: httpTimeout}

	type sessionListRsp struct {
		Error    *apiError `json:"error,omitempty"`
		Sessions []struct {
			ID           uint   `json:"id"`
			AgentHarness string `json:"agentHarness"`
		} `json:"sessions,omitempty"`
	}

	query := func(filter string) sessionListRsp {
		target := fmt.Sprintf("%s/api/web/v1/session/list?page=1&pageSize=20&filter=%s",
			baseURL, url.QueryEscape(filter))
		var rsp sessionListRsp
		if status := doJSON(t, httpClient, http.MethodGet, target, jwt, "", &rsp); status != http.StatusOK {
			t.Fatalf("list sessions with filter %q: status=%d err=%+v", filter, status, rsp.Error)
		}
		return rsp
	}

	if rsp := query("harness:__no_such_harness__"); len(rsp.Sessions) != 0 {
		t.Fatalf("unknown harness filter returned %d sessions, want 0", len(rsp.Sessions))
	}
	for _, s := range query("harness:Other").Sessions {
		if s.AgentHarness != "Other" {
			t.Fatalf("session %d agentHarness = %q, want Other", s.ID, s.AgentHarness)
		}
	}
}

// TestHarnessTrendContract 走势图接口契约：返回 series，Other 恒在最后。
func TestHarnessTrendContract(t *testing.T) {
	t.Parallel()

	baseURL, jwt := mustEnv(t)
	httpClient := &http.Client{Timeout: httpTimeout}

	end := time.Now().UTC()
	start := end.Add(-7 * 24 * time.Hour)
	target := fmt.Sprintf("%s/api/web/v1/audit/stats/harness/trend?startTime=%s&endTime=%s&granularity=day",
		baseURL, url.QueryEscape(start.Format(time.RFC3339)), url.QueryEscape(end.Format(time.RFC3339)))

	var rsp struct {
		Error *apiError `json:"error,omitempty"`
		Data  []struct {
			Harness string `json:"harness"`
			Points  []struct {
				Time  time.Time `json:"time"`
				Count int       `json:"count"`
			} `json:"points"`
		} `json:"data,omitempty"`
	}
	if status := doJSON(t, httpClient, http.MethodGet, target, jwt, "", &rsp); status != http.StatusOK {
		t.Fatalf("harness trend: status=%d err=%+v", status, rsp.Error)
	}
	for i := 1; i < len(rsp.Data); i++ {
		if rsp.Data[i].Harness == "Other" && rsp.Data[i-1].Harness != "Other" {
			t.Fatalf("Other must be the last series, got %v", rsp.Data)
		}
	}
}

// TestHarnessRuleAdminOnly 非 admin 访问规则管理必须被拒。
//
// 环境变量 USER_JWT_TOKEN（普通用户 JWT）缺失时跳过。
func TestHarnessRuleAdminOnly(t *testing.T) {
	t.Parallel()

	baseURL, _ := mustEnv(t)
	userJWT := os.Getenv("USER_JWT_TOKEN")
	if userJWT == "" {
		t.Skip("USER_JWT_TOKEN is required to verify admin-only access")
	}

	httpClient := &http.Client{Timeout: httpTimeout}
	var listed listRulesRsp
	status := doJSON(t, httpClient, http.MethodGet, baseURL+"/api/web/v1/harness/rule/list", userJWT, "", &listed)
	if status != http.StatusForbidden {
		t.Fatalf("non-admin list rules: status=%d, want 403", status)
	}
}
