// Package apikeys 验证 API Key 列表接口嵌套所属用户信息（user 列）。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填）
package apikeys

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

type apiKeyUser struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type apiKeyItem struct {
	ID   uint        `json:"id"`
	Name string      `json:"name"`
	Key  string      `json:"key"`
	User *apiKeyUser `json:"user"`
}

type listAPIKeysRsp struct {
	Keys []apiKeyItem `json:"keys"`
}

type currentUserRsp struct {
	User struct {
		ID     uint   `json:"id"`
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	} `json:"user"`
}

// TestE2E_AdminListAPIKeysUserNesting 验证 admin 列表可解析且每条携带 user 的条目字段完整。
// 生产库可能存在 legacy key（user_id=0，无 user 嵌套），因此只断言非空 user 的字段约束。
func TestE2E_AdminListAPIKeysUserNesting(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/apikey/list?page=1&pageSize=50", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list apikeys expected 200, got %d: %s", status, body)
	}
	var rsp listAPIKeysRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}
	nested, legacy := 0, 0
	for _, k := range rsp.Keys {
		if k.User == nil {
			legacy++
			continue
		}
		nested++
		if k.User.ID == 0 || k.User.Name == "" {
			t.Errorf("key %d carries user with zero ID or empty name: %+v", k.ID, k.User)
		}
	}
	t.Logf("admin apikey list: %d nested users, %d legacy keys without user", nested, legacy)
}

// TestE2E_UserListAPIKeysMatchesCurrentUser 验证普通用户视角每条 key 嵌套的 user
// 与 /user/current 返回的当前用户一致（用户自己的 key 必有 user）。
func TestE2E_UserListAPIKeysMatchesCurrentUser(t *testing.T) {
	t.Parallel()
	baseURL, _, userToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/current", userToken)
	if status != http.StatusOK {
		t.Fatalf("get current user expected 200, got %d: %s", status, body)
	}
	var cur currentUserRsp
	if err := sonic.Unmarshal(body, &cur); err != nil {
		t.Fatalf("unmarshal current user failed: %v", err)
	}
	if cur.User.ID == 0 {
		t.Fatalf("current user ID is zero: %s", body)
	}

	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/apikey/list?page=1&pageSize=50", userToken)
	if status != http.StatusOK {
		t.Fatalf("list apikeys expected 200, got %d: %s", status, body)
	}
	var rsp listAPIKeysRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}
	if len(rsp.Keys) == 0 {
		t.Skip("user has no api keys; nothing to assert")
	}
	for _, k := range rsp.Keys {
		if k.User == nil {
			t.Fatalf("key %d (owned by current user %d) has no user nesting", k.ID, cur.User.ID)
		}
		if k.User.ID != cur.User.ID || k.User.Name != cur.User.Name || k.User.Avatar != cur.User.Avatar {
			t.Errorf("key %d user nesting %+v != current user %+v", k.ID, k.User, cur.User)
		}
	}
}
