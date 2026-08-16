// Package users 验证用户审核闭环：admin 列表+批准 pending 用户，普通用户 403。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填）
package users

import (
	"context"
	"fmt"
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

// assertErrorCode 验证响应体携带统一错误结构且 code 命中允许列表（新契约：HTTP 200 + {error}）。
func assertErrorCode(t *testing.T, body []byte, allowedCodes ...int) {
	t.Helper()
	var rsp struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if rsp.Error == nil {
		t.Fatalf("response carries no error structure: %s", string(body))
	}
	for _, c := range allowedCodes {
		if rsp.Error.Code == c {
			return
		}
	}
	t.Fatalf("error code %d not in allowed %v, body=%s", rsp.Error.Code, allowedCodes, string(body))
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

type listUsersRsp struct {
	Items []struct {
		ID         uint   `json:"id"`
		Name       string `json:"name"`
		Permission string `json:"permission"`
	} `json:"items"`
}

func TestE2E_AdminCanListUsers(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// admin 列表接口可达且结构正确
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list users expected 200, got %d: %s", status, body)
	}
	var rsp listUsersRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal list response failed: %v", err)
	}

	// 权限筛选 pending 用户
	status, body = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20&permission=pending", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list pending users expected 200, got %d: %s", status, body)
	}
	var pending listUsersRsp
	if err := sonic.Unmarshal(body, &pending); err != nil {
		t.Fatalf("unmarshal pending response failed: %v", err)
	}
	if len(pending.Items) > 0 {
		for _, u := range pending.Items {
			if u.Permission != "pending" {
				t.Fatalf("expected all items pending, got %s", u.Permission)
			}
		}
	}
}

func TestE2E_ApprovePendingUserFlow(t *testing.T) {
	t.Parallel()
	baseURL, adminToken, _ := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 找到第一个 pending 用户并批准
	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20&permission=pending", adminToken)
	if status != http.StatusOK {
		t.Fatalf("list pending users expected 200, got %d: %s", status, body)
	}
	var pending listUsersRsp
	if err := sonic.Unmarshal(body, &pending); err != nil {
		t.Fatalf("unmarshal pending response failed: %v", err)
	}
	if len(pending.Items) == 0 {
		t.Skip("no pending users in environment, approve flow skipped")
	}
	target := pending.Items[0]

	// 批准 → 200
	status, body = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/approve?id=%d", baseURL, target.ID), adminToken)
	if status != http.StatusOK {
		t.Fatalf("approve user expected 200, got %d: %s", status, body)
	}

	// 重复批准同一用户应失败（业务错误，非 200）
	status, body = doJSON(t, client, http.MethodPost, fmt.Sprintf("%s/api/v1/user/approve?id=%d", baseURL, target.ID), adminToken)
	if status == http.StatusOK {
		t.Fatalf("re-approve expected non-200, got 200: %s", body)
	}
}

func TestE2E_RegularUserGetsForbidden(t *testing.T) {
	t.Parallel()
	baseURL, _, userToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	status, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/user/list?page=1&pageSize=20", userToken)
	if status != http.StatusOK {
		t.Fatalf("regular user list unified contract expected 200, got %d: %s", status, body)
	}
	assertErrorCode(t, body, 10002, 10001)

	status, body = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/user/approve?id=1", userToken)
	if status != http.StatusOK {
		t.Fatalf("regular user approve unified contract expected 200, got %d: %s", status, body)
	}
	assertErrorCode(t, body, 10002, 10001)
}
