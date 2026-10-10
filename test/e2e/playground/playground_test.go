// Package playground 验证 Playground 调试链路：转发成功、审计留痕、会话不落地。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//   - USER_TOKEN   普通用户 JWT（必填，playground 要求权限 ≥ user）
//   - USER_API_KEY_ID USER_TOKEN 用户名下的 API Key ID（必填，playground 调用按该 Key 归属审计）
//   - DEMO_TOKEN   Demo JWT（可选，验证 demo 被拒）
//
// 进程内的归属/越权守护见 test/e2e/playground_attribution（无需外部环境）。
//
// 假上游由测试进程内 httptest 提供，要求被测服务能访问 127.0.0.1。
// 用例会真实创建/删除 endpoint 与 model，经 e2eguard 拒绝误打生产。
package playground

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/test/e2e/e2eguard"
)

const e2eHTTPTimeout = 30 * time.Second

type env struct {
	baseURL    string
	adminToken string
	userToken  string
	userKeyID  string
	demoToken  string
}

func mustE2EEnv(t *testing.T) env {
	t.Helper()
	baseURL := os.Getenv("BASE_URL")
	adminToken := os.Getenv("ADMIN_TOKEN")
	userToken := os.Getenv("USER_TOKEN")
	userKeyID := os.Getenv("USER_API_KEY_ID")
	if baseURL == "" || adminToken == "" || userToken == "" || userKeyID == "" {
		t.Skip("BASE_URL, ADMIN_TOKEN, USER_TOKEN and USER_API_KEY_ID are required for e2e test")
	}
	e2eguard.GuardLiveTarget(t, baseURL)
	return env{
		baseURL:    strings.TrimRight(baseURL, "/"),
		adminToken: adminToken,
		userToken:  userToken,
		userKeyID:  userKeyID,
		demoToken:  os.Getenv("DEMO_TOKEN"),
	}
}

func doJSON(t *testing.T, client *http.Client, method, url, token string, body []byte) (statusCode int, respBody []byte) {
	t.Helper()
	var reader io.Reader = http.NoBody
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, reader)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+token)
	if body != nil {
		req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s failed: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ = io.ReadAll(resp.Body)
	return resp.StatusCode, respBody
}

// TestPlaygroundChat 转发成功 + 审计有记录 + 会话不落地。
func TestPlaygroundChat(t *testing.T) {
	t.Parallel()
	e := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}
	startedAt := time.Now().Add(-time.Minute)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-playground","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer upstream.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
	alias := "playground-alias-" + suffix

	epID := mustCreateEndpoint(t, client, e.baseURL, e.adminToken, "pg-ep-"+suffix, upstream.URL)
	defer mustDelete(t, client, e.baseURL, e.adminToken, "/api/web/v1/endpoint", epID)
	modelID := mustCreateModel(t, client, e.baseURL, e.adminToken, alias, epID)
	defer mustDelete(t, client, e.baseURL, e.adminToken, "/api/web/v1/model", modelID)

	// 1. playground 调用成功
	body := []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"stream":false}`, alias))
	code, resp := doJSON(t, client, http.MethodPost, e.baseURL+"/api/web/v1/playground/chat?apiKeyID="+e.userKeyID, e.userToken, body)
	if code != http.StatusOK {
		t.Fatalf("playground chat failed: %d %s", code, resp)
	}
	if !bytes.Contains(resp, []byte("chatcmpl-playground")) {
		t.Fatalf("unexpected rsp: %s", resp)
	}

	// 2. 审计有记录（playground 调用可见）
	auditCode, auditResp := doJSON(t, client, http.MethodGet, e.baseURL+"/api/web/v1/audit/model/log/list?pageSize=10", e.userToken, nil)
	if auditCode != http.StatusOK {
		t.Fatalf("audit list failed: %d %s", auditCode, auditResp)
	}
	if !bytes.Contains(auditResp, []byte(alias)) {
		t.Fatalf("审计应包含 playground 调用记录, got: %s", auditResp)
	}

	// 3. 会话不落地（playground 流量不进会话库）
	sessCode, sessResp := doJSON(t, client, http.MethodGet, e.baseURL+"/api/web/v1/session/list?pageSize=10", e.userToken, nil)
	if sessCode != http.StatusOK {
		t.Fatalf("session list failed: %d %s", sessCode, sessResp)
	}
	if bytes.Contains(sessResp, []byte(alias)) {
		t.Fatalf("playground 流量不应落入会话库, got: %s", sessResp)
	}

	// 4. demo 被拒（权限不足）
	if e.demoToken != "" {
		code, _ = doJSON(t, client, http.MethodPost, e.baseURL+"/api/web/v1/playground/chat?apiKeyID="+e.userKeyID, e.demoToken, body)
		if code != http.StatusForbidden && code != http.StatusUnauthorized {
			t.Fatalf("demo 应被拒绝, got %d", code)
		}
	}
	_ = startedAt
}

func mustCreateEndpoint(t *testing.T, client *http.Client, baseURL, token, name, upstreamURL string) uint {
	t.Helper()
	body := []byte(fmt.Sprintf(
		`{"name":%q,"openaiBaseURL":%q,"anthropicBaseURL":"","apiKey":"sk-e2e","supportOpenAIChatCompletion":true,"supportOpenAIResponse":false,"supportAnthropicMessage":false}`,
		name, upstreamURL))
	code, resp := doJSON(t, client, http.MethodPost, baseURL+"/api/web/v1/endpoint", token, body)
	if code != http.StatusOK {
		t.Fatalf("create endpoint %s failed: %d %s", name, code, resp)
	}
	var parsed struct {
		Data struct {
			EndpointID uint `json:"endpointID"`
		} `json:"data"`
	}
	if err := sonic.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("parse endpoint rsp: %v", err)
	}
	return parsed.Data.EndpointID
}

func mustCreateModel(t *testing.T, client *http.Client, baseURL, token, alias string, endpointID uint) uint {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"alias":%q,"upstreamModel":"e2e-upstream","endpointID":%d}`, alias, endpointID))
	code, resp := doJSON(t, client, http.MethodPost, baseURL+"/api/web/v1/model", token, body)
	if code != http.StatusOK {
		t.Fatalf("create model failed: %d %s", code, resp)
	}
	listCode, listResp := doJSON(t, client, http.MethodGet, baseURL+"/api/web/v1/model/list?pageSize=100", token, nil)
	if listCode != http.StatusOK {
		t.Fatalf("list models failed: %d %s", listCode, listResp)
	}
	var parsed struct {
		Items []struct {
			ID    uint   `json:"id"`
			Alias string `json:"alias"`
			Ep    struct {
				ID uint `json:"id"`
			} `json:"endpoint"`
		} `json:"items"`
	}
	if err := sonic.Unmarshal(listResp, &parsed); err != nil {
		t.Fatalf("parse model list: %v", err)
	}
	for _, item := range parsed.Items {
		if item.Alias == alias && item.Ep.ID == endpointID {
			return item.ID
		}
	}
	t.Fatalf("created model %s not found in list", alias)
	return 0
}

func mustDelete(t *testing.T, client *http.Client, baseURL, token, resource string, id uint) {
	t.Helper()
	code, resp := doJSON(t, client, http.MethodDelete, fmt.Sprintf("%s%s?id=%d", baseURL, resource, id), token, nil)
	if code != http.StatusOK {
		t.Logf("cleanup %s?id=%d failed: %d %s", resource, id, code, resp)
	}
}
