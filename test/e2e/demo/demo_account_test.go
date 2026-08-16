// Package demo 验证 Demo 演示账户闭环：入口状态、配置管理、登录、模块白名单与只读受限。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填）
package demo

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 30 * time.Second

func mustE2EEnv(t *testing.T) (baseURL, adminToken, userToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	adminToken = os.Getenv("ADMIN_TOKEN")
	userToken = os.Getenv("USER_TOKEN")
	if baseURL == "" || adminToken == "" || userToken == "" {
		t.Skip("BASE_URL, ADMIN_TOKEN and USER_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), adminToken, userToken
}

func doJSON(t *testing.T, client *http.Client, method, url, token string) (statusCode int, respBody []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
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
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
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

func bizErrorCode(body []byte) int {
	var rsp struct {
		Error *struct {
			Code int    `json:"code"`
			Msg  string `json:"message"`
		} `json:"error"`
	}
	_ = sonic.Unmarshal(body, &rsp)
	if rsp.Error == nil {
		return 0
	}
	return rsp.Error.Code
}

type listUsersRsp struct {
	Items []struct {
		ID         uint   `json:"id"`
		Permission string `json:"permission"`
	} `json:"items"`
}

// TestE2E_DemoStatusPublic 无鉴权状态端点可达
func TestE2E_DemoStatusPublic(t *testing.T) {
	t.Parallel()
	baseURL, _, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/status", "")
	if status != http.StatusOK {
		t.Fatalf("demo status expected 200, got %d: %s", status, body)
	}
	var rsp struct {
		LoginEnabled   bool `json:"loginEnabled"`
		DemoUserExists bool `json:"demoUserExists"`
	}
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal demo status failed: %v", err)
	}
}

// TestE2E_DemoConfigPermission 配置读写权限：普通用户只读，admin 可改；非法 modulus 拒绝
func TestE2E_DemoConfigPermission(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, userToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 任意登录用户可读
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/config", userToken)
	if status != http.StatusOK {
		t.Fatalf("user get demo config expected 200, got %d: %s", status, body)
	}

	// 普通用户 PATCH 拒绝
	patch := map[string]any{"config": map[string]any{"loginEnabled": true}}
	patchBody, _ := sonic.Marshal(patch)
	status, body = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", userToken, patchBody)
	if status != http.StatusOK && bizErrorCode(body) == 0 {
		t.Fatalf("user patch demo config expected business error, got %d: %s", status, body)
	}

	// admin 非法 modulus（=1）拒绝
	badPatch := map[string]any{"config": map[string]any{"sampleModulus": 1}}
	badBody, _ := sonic.Marshal(badPatch)
	status, body = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", adminToken, badBody)
	if status == http.StatusOK && bizErrorCode(body) == 0 {
		t.Fatalf("admin patch sampleModulus=1 expected validation error, got %d: %s", status, body)
	}
}

// TestE2E_DemoAccountLifecycle 完整闭环：设 demo → 开入口 → demo 登录 → 模块白名单/只读校验 → 恢复
// 生命周期测试顺序修改全局 demo 状态，保持串行
//
//nolint:paralleltest // 修改共享 demo 配置与账户状态，串行执行
func TestE2E_DemoAccountLifecycle(t *testing.T) {
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 1. 找一个普通用户（非 admin、非操作者本人）
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=50&permission=user", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list users expected 200, got %d: %s", status, body)
	}
	var users listUsersRsp
	if err := sonic.Unmarshal(body, &users); err != nil {
		t.Fatalf("unmarshal list users failed: %v", err)
	}
	if len(users.Items) == 0 {
		t.Skip("no regular users in environment, demo lifecycle skipped")
	}
	target := users.Items[0]

	// 记录当前 demo 配置，测试结束恢复
	var origConfig struct {
		Config struct {
			LoginEnabled  bool     `json:"loginEnabled"`
			SampleModulus uint     `json:"sampleModulus"`
			Modules       []string `json:"modules"`
		} `json:"config"`
	}
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/config", adminToken)
	if status == http.StatusOK {
		_ = sonic.Unmarshal(body, &origConfig)
	}

	// 若已有 demo 用户，先记下以便结束恢复
	var demoBefore uint
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=50&permission=demo", adminToken)
	if status == http.StatusOK {
		var demoUsers listUsersRsp
		if err := sonic.Unmarshal(body, &demoUsers); err == nil && len(demoUsers.Items) > 0 {
			demoBefore = demoUsers.Items[0].ID
		}
	}

	restore := func(t *testing.T) {
		t.Helper()
		// 恢复 demo 配置
		if len(origConfig.Config.Modules) > 0 || origConfig.Config.SampleModulus > 0 {
			restorePatch := map[string]any{"config": map[string]any{
				"loginEnabled":  origConfig.Config.LoginEnabled,
				"sampleModulus": origConfig.Config.SampleModulus,
				"modules":       origConfig.Config.Modules,
			}}
			rb, _ := sonic.Marshal(restorePatch)
			_, _ = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", adminToken, rb)
		}
		// 恢复 demo 账户状态
		if demoBefore > 0 {
			_, _ = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/demo/restore?id="+itoa(demoBefore), adminToken)
		} else {
			_, _ = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/demo/restore?id="+itoa(target.ID), adminToken)
		}
	}
	t.Cleanup(func() { restore(t) })

	// 2. 设为 demo
	status, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/demo?id="+itoa(target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("set demo user expected 200, got %d: %s", status, body)
	}

	// 3. 开放入口 + 仅开放 sessions 模块
	openPatch := map[string]any{"config": map[string]any{
		"loginEnabled":  true,
		"sampleModulus": 10,
		"modules":       []string{"sessions"},
	}}
	ob, _ := sonic.Marshal(openPatch)
	status, body = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", adminToken, ob)
	if status != http.StatusOK {
		t.Fatalf("open demo config expected 200, got %d: %s", status, body)
	}

	// 4. demo 登录
	status, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/demo/login", "")
	if status != http.StatusOK {
		t.Fatalf("demo login expected 200, got %d: %s", status, body)
	}
	var login struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := sonic.Unmarshal(body, &login); err != nil || login.AccessToken == "" {
		t.Fatalf("demo login response missing token: %s", body)
	}
	demoToken := login.AccessToken

	// 5. 当前用户 permission=demo
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/current", demoToken)
	if status != http.StatusOK {
		t.Fatalf("demo get current user expected 200, got %d: %s", status, body)
	}
	var cur struct {
		User struct {
			Permission string `json:"permission"`
		} `json:"user"`
	}
	if err := sonic.Unmarshal(body, &cur); err != nil {
		t.Fatalf("unmarshal current user failed: %v", err)
	}
	if cur.User.Permission != "demo" {
		t.Fatalf("expected demo permission, got %s", cur.User.Permission)
	}

	// 6. 开放模块（sessions）可访问
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/session/list?page=1&pageSize=5", demoToken)
	if status != http.StatusOK {
		t.Fatalf("demo list sessions (open module) expected 200, got %d: %s", status, body)
	}

	// 7. shares 接口拒绝（shares 不在 demo 模块白名单，越权回归：修复前 demo 可直调）
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/session/share/list", demoToken)
	if status == http.StatusOK && bizErrorCode(body) == 0 {
		t.Fatalf("demo list shares expected error, got 200: %s", body)
	}
	createSharePayload := map[string]any{"body": map[string]any{"sessionId": 1}}
	csb, _ := sonic.Marshal(createSharePayload)
	status, body = doJSONBody(t, client, http.MethodPost, baseURL+"/api/v1/session/share", demoToken, csb)
	if status == http.StatusOK && bizErrorCode(body) == 0 {
		t.Fatalf("demo create share expected error, got 200: %s", body)
	}

	// 8. 未开放模块（audit 图表）拒绝
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/audit/stats/model/trend?startTime=2026-01-01T00:00:00Z&endTime=2026-12-31T23:59:59Z&granularity=day", demoToken)
	if status == http.StatusOK && bizErrorCode(body) == 0 {
		t.Fatalf("demo closed-module audit expected error, got 200: %s", body)
	}

	// 9. 写接口拒绝（签发 API Key）
	createKey := map[string]any{"body": map[string]any{"name": "e2e-demo-should-fail"}}
	cb, _ := sonic.Marshal(createKey)
	status, body = doJSONBody(t, client, http.MethodPost, baseURL+"/api/v1/apikey", demoToken, cb)
	if status == http.StatusOK && bizErrorCode(body) == 0 {
		t.Fatalf("demo write apikey expected error, got 200: %s", body)
	}

	// 10. 恢复 demo → user，入口失效
	status, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/demo/restore?id="+itoa(target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("restore demo user expected 200, got %d: %s", status, body)
	}
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
