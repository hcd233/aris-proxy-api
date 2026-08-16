// Package users 验证用户管理操作：降级（demote）与删除（delete）的权限与规则闭环。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填）
package users

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
)

type currentUserRsp struct {
	User struct {
		ID uint `json:"id"`
	} `json:"user"`
}

func adminSelfID(t *testing.T, baseURL, adminToken string) uint {
	t.Helper()
	client := &http.Client{Timeout: e2eHTTPTimeout}
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/current", adminToken)
	if status != http.StatusOK {
		t.Fatalf("get current user expected 200, got %d: %s", status, body)
	}
	var rsp currentUserRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal current user response failed: %v", err)
	}
	return rsp.User.ID
}

func TestE2E_RegularUserCannotManageUsers(t *testing.T) {
	t.Parallel()
	baseURL, _, userToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	for _, tc := range []struct {
		method string
		url    string
	}{
		{http.MethodPost, baseURL + "/api/v1/user/demote?id=1"},
		{http.MethodDelete, baseURL + "/api/v1/user/delete?id=1"},
	} {
		status, body := doJSON(t, client, tc.method, tc.url, userToken)
		if status != http.StatusOK {
			t.Fatalf("regular user %s %s unified contract expected 200, got %d: %s", tc.method, tc.url, status, body)
		}
		var rsp struct {
			Error *struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := sonic.Unmarshal(body, &rsp); err != nil || rsp.Error == nil || (rsp.Error.Code != 10002 && rsp.Error.Code != 10001) {
			t.Fatalf("regular user %s %s expected error 10002/10001, body=%s", tc.method, tc.url, string(body))
		}
	}
}

func TestE2E_AdminDemoteThenReapprove(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 找第一个 user 权限用户
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20&permission=user", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list user-permission users expected 200, got %d: %s", status, body)
	}
	var rsp listUsersRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}
	if len(rsp.Items) == 0 {
		t.Skip("no regular users in environment, demote flow skipped")
	}
	target := rsp.Items[0]

	// 降级 → 200
	status, body = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/demote?id=%d", baseURL, target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("demote user expected 200, got %d: %s", status, body)
	}

	// 恢复：批准 → 200（闭环无副作用）
	status, body = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/approve?id=%d", baseURL, target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("re-approve after demote expected 200, got %d: %s", status, body)
	}
}

func TestE2E_AdminCannotDeleteSelf(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	adminID := adminSelfID(t, baseURL, adminToken)
	status, body := doJSON(t, client, http.MethodDelete, fmt.Sprintf("%s/api/v1/user/delete?id=%d", baseURL, adminID), adminToken)
	if status == http.StatusOK {
		t.Fatalf("delete self expected non-200, got 200: %s", body)
	}
}

func TestE2E_AdminDeleteNonexistentUser(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	status, body := doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/user/delete?id=99999999", adminToken)
	if status == http.StatusOK {
		t.Fatalf("delete nonexistent user expected non-200, got 200: %s", body)
	}
}
