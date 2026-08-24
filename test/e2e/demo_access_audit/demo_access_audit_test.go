// Package demo_access_audit 验证 Demo 访问审计闭环：demo 登录、访问开放模块、
// 探测锁定模块均产生审计记录，admin 可查询，demo 自身不可见。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填）
package demo_access_audit

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

const e2eHTTPTimeout = 30 * time.Second

func mustE2EEnv(t *testing.T) (baseURL, adminToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	adminToken = os.Getenv("ADMIN_TOKEN")
	if baseURL == "" || adminToken == "" {
		t.Skip("BASE_URL and ADMIN_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), adminToken
}

func doJSON(t *testing.T, client *http.Client, method, url, token string) (statusCode int, respBody []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	if token != "" {
		req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s failed: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return resp.StatusCode, body
}

func doJSONBody(t *testing.T, client *http.Client, method, url, token string, payload []byte) (statusCode int, respBody []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	if token != "" {
		req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s failed: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return resp.StatusCode, body
}

// TestE2E_DemoAccessAudit 审计闭环：
// 开入口 + 仅开 sessions → demo 登录 → 访问开放模块 → 探测锁定模块 →
// admin 查到三类记录；demo 查询接口被拒。
//
// 会临时修改全局 demo 配置，结束后恢复，串行执行。
//
//nolint:paralleltest // 修改共享 demo 配置，串行执行
func TestE2E_DemoAccessAudit(t *testing.T) {
	baseURL, adminToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 本次测试产生记录的时间基线（放宽 5 分钟吸收本地与服务器时钟偏差），
	// 用于把新记录与历史上 IP/UA 恒空的旧记录区分开
	startBaseline := time.Now().Add(-5 * time.Minute)

	// 记录当前 demo 配置，测试结束恢复
	var origConfig struct {
		Config struct {
			LoginEnabled bool     `json:"loginEnabled"`
			Modules      []string `json:"modules"`
		} `json:"config"`
	}
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/config", adminToken)
	if status != http.StatusOK {
		t.Fatalf("get demo config expected 200, got %d: %s", status, body)
	}
	if err := sonic.Unmarshal(body, &origConfig); err != nil {
		t.Fatalf("unmarshal demo config failed: %v", err)
	}

	t.Cleanup(func() {
		restorePatch := map[string]any{"config": map[string]any{
			"loginEnabled": origConfig.Config.LoginEnabled,
			"modules":      origConfig.Config.Modules,
		}}
		rb, _ := sonic.Marshal(restorePatch)
		_, _ = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", adminToken, rb)
	})

	// 1. 开放入口 + 仅开放 sessions（其余模块全部锁定，供探测）
	openPatch := map[string]any{"config": map[string]any{
		"loginEnabled": true,
		"modules":      []string{"sessions"},
	}}
	ob, _ := sonic.Marshal(openPatch)
	status, body = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", adminToken, ob)
	if status != http.StatusOK {
		t.Fatalf("open demo config expected 200, got %d: %s", status, body)
	}

	// 2. demo 登录（产生 login 审计）
	status, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/demo/login", "")
	if status != http.StatusOK {
		t.Fatalf("demo login expected 200, got %d: %s", status, body)
	}
	var login struct {
		AccessToken string `json:"accessToken"`
	}
	if err := sonic.Unmarshal(body, &login); err != nil || login.AccessToken == "" {
		t.Fatalf("demo login response missing token: %s", body)
	}
	demoToken := login.AccessToken

	// 3. 访问开放模块 sessions 列表（产生 module_access 审计）
	status, _ = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/session/list?page=1&pageSize=10", demoToken)
	if status != http.StatusOK {
		t.Fatalf("demo list sessions expected 200, got %d", status)
	}

	// 4. 探测锁定模块 models（产生 module_denied 审计）
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/model/list?page=1&pageSize=10", demoToken)
	if status == http.StatusOK && !strings.Contains(string(body), "error") {
		t.Fatalf("demo probe locked module expected business error, got 200: %s", body)
	}

	// 5. admin 查询审计列表，断言三类记录齐全
	// 审计经协程池异步落库，轮询重试直至出现或超时（同 cron_trigger e2e 的 ticker 轮询模式）
	found := map[string]bool{"login": false, "module_access": false, "module_denied": false}
	deadline := time.Now().Add(e2eHTTPTimeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for !allFound(found) {
		listRsp := queryDemoAccessAudits(t, client, baseURL, adminToken, "")
		for _, item := range listRsp.Logs {
			if _, ok := found[item.Action]; ok {
				found[item.Action] = true
			}
		}
		if allFound(found) {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		<-ticker.C
	}
	for action, ok := range found {
		if !ok {
			t.Errorf("expected audit record with action=%q not found within timeout", action)
		}
	}

	// 5.1 回归断言：本次测试产生的 module_access / module_denied 记录必须带 IP/UA
	// （曾有埋点从 ctx key 读 IP/UA、但管理路由未挂注入中间件导致恒空的缺陷）
	finalRsp := queryDemoAccessAudits(t, client, baseURL, adminToken, "")
	for _, item := range finalRsp.Logs {
		if item.Action != enum.DemoAccessActionModuleAccess && item.Action != enum.DemoAccessActionModuleDenied {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
		if err != nil || createdAt.Before(startBaseline) {
			continue
		}
		if item.IP == "" || item.UserAgent == "" {
			t.Errorf("audit record (id=%d action=%s path=%s) missing IP/UA", item.ID, item.Action, item.Path)
		}
	}

	// 6. demo 查询审计列表被拒
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/audit/demo/log/list?page=1&pageSize=10", demoToken)
	if status == http.StatusOK && !strings.Contains(string(body), "error") {
		t.Fatalf("demo query demo access audits expected business error, got 200: %s", body)
	}
}

type demoAuditListRsp struct {
	Logs []struct {
		ID        uint   `json:"id"`
		Action    string `json:"action"`
		Module    string `json:"module"`
		Path      string `json:"path"`
		IP        string `json:"ip"`
		UserAgent string `json:"userAgent"`
		CreatedAt string `json:"createdAt"`
	} `json:"logs"`
}

func queryDemoAccessAudits(t *testing.T, client *http.Client, baseURL, adminToken, filter string) demoAuditListRsp {
	t.Helper()
	url := baseURL + "/api/v1/audit/demo/log/list?page=1&pageSize=100&sort=desc&sortField=created_at"
	if filter != "" {
		url += "&filter=" + filter
	}
	status, body := doJSON(t, client, http.MethodGet, url, adminToken)
	if status != http.StatusOK {
		t.Fatalf("admin list demo access audits expected 200, got %d: %s", status, body)
	}
	var rsp demoAuditListRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal demo access audits failed: %v", err)
	}
	return rsp
}

func allFound(found map[string]bool) bool {
	for _, ok := range found {
		if !ok {
			return false
		}
	}
	return true
}
