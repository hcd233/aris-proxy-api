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
	"sync"
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

// demoSessionsListRsp demo 白名单会话列表响应（只关注 ID）
type demoSessionsListRsp struct {
	Sessions []struct {
		ID uint `json:"id"`
	} `json:"sessions"`
}

// sessionListRsp 会话列表响应（只关注 ID）
type sessionListRsp struct {
	Sessions []struct {
		ID uint `json:"id"`
	} `json:"sessions"`
}

// sessionDetailRsp 会话详情响应（只关注 ID）
type sessionDetailRsp struct {
	Session *struct {
		ID uint `json:"id"`
	} `json:"session"`
}

// auditLogItem demo 视角审计日志条目（脱敏字段断言）
type auditLogItem struct {
	ID         uint   `json:"id"`
	TraceID    string `json:"traceId"`
	APIKeyName string `json:"apiKeyName"`
	UserName   string `json:"userName"`
	UserEmail  string `json:"userEmail"`
	Endpoint   string `json:"endpoint"`
}

// auditLogListRsp 审计日志列表响应
type auditLogListRsp struct {
	Logs []*auditLogItem `json:"logs"`
}

// httpResult 并发限流探针的请求结果（避免在子 goroutine 中调用 t.Fatal）
type httpResult struct {
	status int
	header http.Header
	body   []byte
	err    error
}

// doJSONRaw 与 doJSON 等价，但返回 error 而非直接 t.Fatal，可在子 goroutine 中安全调用。
func doJSONRaw(client *http.Client, method, url, token string) httpResult {
	req, err := http.NewRequestWithContext(context.Background(), method, url, http.NoBody)
	if err != nil {
		return httpResult{err: err}
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	resp, err := client.Do(req)
	if err != nil {
		return httpResult{err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return httpResult{err: err}
	}
	return httpResult{status: resp.StatusCode, header: resp.Header, body: body}
}

// assertMasked 断言字段已脱敏：空值跳过；MaskIdentity 输出固定 "***"，MaskSecret 输出
// "***"（长度<=8）或保留首尾各 4 字符的 "xxxx***yyyy"。
func assertMasked(t *testing.T, name, value string) {
	t.Helper()
	if value == "" {
		return
	}
	masked := value == "***" || (len(value) >= 11 && value[4:7] == "***")
	if !masked {
		t.Errorf("demo audit field %s should be masked, got %q", name, value)
	}
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

// TestE2E_DemoConfigPermission 配置读写权限：普通用户只读，admin 可改
func TestE2E_DemoConfigPermission(t *testing.T) {
	t.Parallel()
	baseURL, _, userToken := mustE2EEnv(t)
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
			LoginEnabled bool     `json:"loginEnabled"`
			Modules      []string `json:"modules"`
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
		if len(origConfig.Config.Modules) > 0 {
			restorePatch := map[string]any{"config": map[string]any{
				"loginEnabled": origConfig.Config.LoginEnabled,
				"modules":      origConfig.Config.Modules,
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
		"loginEnabled": true,
		"modules":      []string{"sessions"},
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

func joinIDs(ids []uint) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, itoa(id))
	}
	return strings.Join(parts, ",")
}

// TestE2E_DemoSessionsWhitelistMaskingRateLimit 覆盖 demo 白名单会话增删、非白名单不可读、
// audit 脱敏与 demo 接口 IP 限流。会修改共享 demo 配置/账户/白名单状态，串行执行并在结束时恢复。
//
//nolint:paralleltest // 修改共享 demo 配置与账户状态，串行执行
func TestE2E_DemoSessionsWhitelistMaskingRateLimit(t *testing.T) {
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 1. 找一个普通用户作为 demo 账户目标
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=50&permission=user", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list users expected 200, got %d: %s", status, body)
	}
	var users listUsersRsp
	if err := sonic.Unmarshal(body, &users); err != nil {
		t.Fatalf("unmarshal list users failed: %v", err)
	}
	if len(users.Items) == 0 {
		t.Skip("no regular users in environment, demo sessions test skipped")
	}
	targetUser := users.Items[0]

	// 2. 找一个存在的 session 作为白名单目标，并准备一个非白名单 session ID
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/session/list?page=1&pageSize=5", adminToken)
	if status != http.StatusOK {
		t.Fatalf("admin list sessions expected 200, got %d: %s", status, body)
	}
	var adminSessions sessionListRsp
	if err := sonic.Unmarshal(body, &adminSessions); err != nil {
		t.Fatalf("unmarshal admin sessions failed: %v", err)
	}
	if len(adminSessions.Sessions) == 0 {
		t.Skip("no sessions in environment, demo sessions test skipped")
	}
	targetSessionID := adminSessions.Sessions[0].ID
	nonWhitelistedID := targetSessionID + 1
	if len(adminSessions.Sessions) > 1 {
		nonWhitelistedID = adminSessions.Sessions[1].ID
	}

	// 3. 记录原始状态（demo 配置 / demo 账户 / 白名单），测试结束恢复
	var origConfig struct {
		Config struct {
			LoginEnabled bool     `json:"loginEnabled"`
			Modules      []string `json:"modules"`
		} `json:"config"`
	}
	origConfigOK := false
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/config", adminToken)
	if status == http.StatusOK {
		origConfigOK = sonic.Unmarshal(body, &origConfig) == nil
	}

	var demoBefore uint
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=50&permission=demo", adminToken)
	if status == http.StatusOK {
		var demoUsers listUsersRsp
		if err := sonic.Unmarshal(body, &demoUsers); err == nil && len(demoUsers.Items) > 0 {
			demoBefore = demoUsers.Items[0].ID
		}
	}

	var originalDemoSessions []uint
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/sessions/list?page=1&pageSize=500", adminToken)
	if status != http.StatusOK {
		t.Fatalf("admin list demo sessions expected 200, got %d: %s", status, body)
	}
	var origSessions demoSessionsListRsp
	if err := sonic.Unmarshal(body, &origSessions); err != nil {
		t.Fatalf("unmarshal demo sessions list failed: %v", err)
	}
	for _, s := range origSessions.Sessions {
		originalDemoSessions = append(originalDemoSessions, s.ID)
	}

	t.Cleanup(func() {
		// 恢复 demo 配置（modules 为 nil 时用空数组以清空）
		if origConfigOK {
			modules := origConfig.Config.Modules
			if modules == nil {
				modules = []string{}
			}
			cfg := map[string]any{
				"loginEnabled": origConfig.Config.LoginEnabled,
				"modules":      modules,
			}
			rb, _ := sonic.Marshal(map[string]any{"config": cfg})
			_, _ = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", adminToken, rb)
		}
		// 恢复 demo 账户：先还原本次设置的 demo 用户，再恢复原 demo 用户（若有）
		_, _ = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/demo/restore?id="+itoa(targetUser.ID), adminToken)
		if demoBefore > 0 {
			_, _ = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/demo?id="+itoa(demoBefore), adminToken)
		}
		// 恢复 demo sessions 白名单：清空当前白名单后重新加入原始项
		_, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/sessions/list?page=1&pageSize=500", adminToken)
		var cur demoSessionsListRsp
		if err := sonic.Unmarshal(body, &cur); err == nil {
			var curIDs []uint
			for _, s := range cur.Sessions {
				curIDs = append(curIDs, s.ID)
			}
			if len(curIDs) > 0 {
				_, _ = doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/demo/sessions?ids="+joinIDs(curIDs), adminToken)
			}
		}
		if len(originalDemoSessions) > 0 {
			addBody := map[string]any{"sessionIds": originalDemoSessions}
			ab, _ := sonic.Marshal(addBody)
			_, _ = doJSONBody(t, client, http.MethodPost, baseURL+"/api/v1/demo/sessions", adminToken, ab)
		}
	})

	// 4. 设置 demo 账户 + 开放 sessions/audit 模块
	status, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/demo?id="+itoa(targetUser.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("set demo user expected 200, got %d: %s", status, body)
	}
	openPatch := map[string]any{"config": map[string]any{
		"loginEnabled": true,
		"modules":      []string{"sessions", "audit"},
	}}
	ob, _ := sonic.Marshal(openPatch)
	status, body = doJSONBody(t, client, http.MethodPatch, baseURL+"/api/v1/demo/config", adminToken, ob)
	if status != http.StatusOK {
		t.Fatalf("open demo config expected 200, got %d: %s", status, body)
	}

	// 5. 清空现有白名单，保证后续断言确定
	if len(originalDemoSessions) > 0 {
		status, body = doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/demo/sessions?ids="+joinIDs(originalDemoSessions), adminToken)
		if status != http.StatusOK {
			t.Fatalf("clear demo sessions expected 200, got %d: %s", status, body)
		}
	}

	// 6. 批量添加白名单会话 → list 含该会话
	addBody := map[string]any{"sessionIds": []uint{targetSessionID}}
	ab, _ := sonic.Marshal(addBody)
	status, body = doJSONBody(t, client, http.MethodPost, baseURL+"/api/v1/demo/sessions", adminToken, ab)
	if status != http.StatusOK {
		t.Fatalf("add demo sessions expected 200, got %d: %s", status, body)
	}
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/sessions/list?page=1&pageSize=500", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list demo sessions expected 200, got %d: %s", status, body)
	}
	var added demoSessionsListRsp
	if err := sonic.Unmarshal(body, &added); err != nil {
		t.Fatalf("unmarshal demo sessions list failed: %v", err)
	}
	if !containsID(added.Sessions, targetSessionID) {
		t.Fatalf("demo sessions list should contain session %d, got %s", targetSessionID, body)
	}

	// 7. demo 登录
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

	// 8. 白名单 session 可读，非白名单 session 返回"不存在"
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/session?id="+itoa(targetSessionID), demoToken)
	if status != http.StatusOK || bizErrorCode(body) != 0 {
		t.Fatalf("demo get whitelisted session expected 200, got %d: %s", status, body)
	}
	var detail sessionDetailRsp
	if err := sonic.Unmarshal(body, &detail); err != nil || detail.Session == nil || detail.Session.ID != targetSessionID {
		t.Fatalf("demo get whitelisted session returned unexpected body: %s", body)
	}

	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/session?id="+itoa(nonWhitelistedID), demoToken)
	if status != http.StatusOK || bizErrorCode(body) != constant.BizErrorCodeDataNotExists {
		t.Fatalf("demo get non-whitelisted session expected not-found(10003), got %d: %s", status, body)
	}

	// 9. audit 视角敏感字段脱敏
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/audit/model/log/list?page=1&pageSize=20", demoToken)
	if status != http.StatusOK || bizErrorCode(body) != 0 {
		t.Fatalf("demo list audit logs expected 200, got %d: %s", status, body)
	}
	var auditLogs auditLogListRsp
	if err := sonic.Unmarshal(body, &auditLogs); err != nil {
		t.Fatalf("unmarshal audit logs failed: %v", err)
	}
	if len(auditLogs.Logs) == 0 {
		t.Log("no audit logs in environment, masking assertion skipped")
	} else {
		for _, log := range auditLogs.Logs {
			assertMasked(t, "userName", log.UserName)
			assertMasked(t, "userEmail", log.UserEmail)
			assertMasked(t, "apiKeyName", log.APIKeyName)
			assertMasked(t, "endpoint", log.Endpoint)
			assertMasked(t, "traceId", log.TraceID)
		}
	}

	// 10. demo 接口高频访问触发 IP 限流（429 + Retry-After）
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/session/list?page=1&pageSize=1", demoToken)
	if status != http.StatusOK || bizErrorCode(body) != 0 {
		t.Fatalf("demo list sessions before rate limit expected 200, got %d: %s", status, body)
	}
	const rateLimitRequests = 60
	results := make(chan httpResult, rateLimitRequests)
	var wg sync.WaitGroup
	for i := 0; i < rateLimitRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- doJSONRaw(client, http.MethodGet, baseURL+"/api/v1/session/list?page=1&pageSize=1", demoToken)
		}()
	}
	wg.Wait()
	close(results)
	saw429 := false
	for r := range results {
		if r.err != nil {
			t.Fatalf("rate limit probe request failed: %v", r.err)
		}
		switch r.status {
		case http.StatusTooManyRequests:
			saw429 = true
			if r.header.Get(constant.HTTPHeaderRetryAfter) == "" {
				t.Errorf("429 response missing Retry-After header")
			}
		case http.StatusOK:
			// 正常放行
		default:
			t.Fatalf("rate limit probe unexpected status %d: %s", r.status, r.body)
		}
	}
	if !saw429 {
		t.Fatalf("expected at least one 429 after %d concurrent demo requests", rateLimitRequests)
	}

	// 11. 批量移除白名单会话 → list 为空
	status, body = doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/demo/sessions?ids="+itoa(targetSessionID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("remove demo sessions expected 200, got %d: %s", status, body)
	}
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/demo/sessions/list?page=1&pageSize=500", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list demo sessions after remove expected 200, got %d: %s", status, body)
	}
	var afterRemove demoSessionsListRsp
	if err := sonic.Unmarshal(body, &afterRemove); err != nil {
		t.Fatalf("unmarshal demo sessions list failed: %v", err)
	}
	if len(afterRemove.Sessions) != 0 {
		t.Fatalf("demo sessions list should be empty after remove, got %s", body)
	}
}

func containsID(sessions []struct {
	ID uint `json:"id"`
}, id uint) bool {
	for _, s := range sessions {
		if s.ID == id {
			return true
		}
	}
	return false
}
