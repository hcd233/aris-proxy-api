// Package endpoint_fallback 验证跨端点 fallback：首候选上游 5xx → 自动切换次候选成功。
//
// 环境变量：
//   - BASE_URL     API 根地址（必填）
//   - ADMIN_TOKEN  管理员 JWT（必填）
//
// 假上游由测试进程内 httptest 提供，要求被测服务能访问 127.0.0.1（本地跑服务时成立）。
// 用例会真实创建/删除 endpoint 与 model，经 e2eguard 拒绝误打生产。
package endpoint_fallback

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

func mustE2EEnv(t *testing.T) (baseURL, adminToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	adminToken = os.Getenv("ADMIN_TOKEN")
	if baseURL == "" || adminToken == "" {
		t.Skip("BASE_URL and ADMIN_TOKEN are required for e2e test")
	}
	e2eguard.GuardLiveTarget(t, baseURL)
	return strings.TrimRight(baseURL, "/"), adminToken
}

func doJSON(t *testing.T, client *http.Client, method, url, token string, body []byte) (statusCode int, respBody []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = http.NoBody
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

// TestEndpointFallback 首候选 5xx → 换次候选成功 → 审计记成功端点。
func TestEndpointFallback(t *testing.T) {
	t.Parallel()
	baseURL, adminToken := mustE2EEnv(t)
	client := &http.Client{Timeout: e2eHTTPTimeout}

	// 假上游 1：永远 500；假上游 2：正常 JSON completion
	badUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
	}))
	defer badUpstream.Close()
	goodUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-fallback","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer goodUpstream.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
	alias := "fallback-alias-" + suffix

	// 创建两个端点：bad（priority 0 首选）与 good（priority 1 次选）
	badEpID := mustCreateEndpoint(t, client, baseURL, adminToken, "ep-bad-"+suffix, badUpstream.URL)
	goodEpID := mustCreateEndpoint(t, client, baseURL, adminToken, "ep-good-"+suffix, goodUpstream.URL)
	defer mustDelete(t, client, baseURL, adminToken, "/api/web/v1/endpoint", badEpID)
	defer mustDelete(t, client, baseURL, adminToken, "/api/web/v1/endpoint", goodEpID)

	badModelID := mustCreateModel(t, client, baseURL, adminToken, alias, badEpID, 0)
	goodModelID := mustCreateModel(t, client, baseURL, adminToken, alias, goodEpID, 1)
	defer mustDelete(t, client, baseURL, adminToken, "/api/web/v1/model", badModelID)
	defer mustDelete(t, client, baseURL, adminToken, "/api/web/v1/model", goodModelID)

	// 创建 ProxyAPIKey 并发起 chat 请求
	apiKey := mustCreateAPIKey(t, client, baseURL, adminToken, "fallback-key-"+suffix)
	reqBody := []byte(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}],"stream":false}`, alias))
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		baseURL+"/api/openai/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("build chat request: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAPIKey, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send chat request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fallback 后应成功, got %d: %s", resp.StatusCode, respBody)
	}
	if !bytes.Contains(respBody, []byte("chatcmpl-fallback")) {
		t.Fatalf("响应应来自 good 上游, got: %s", respBody)
	}
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

func mustCreateModel(t *testing.T, client *http.Client, baseURL, token, alias string, endpointID uint, priority int) uint {
	t.Helper()
	body := []byte(fmt.Sprintf(
		`{"alias":%q,"upstreamModel":"e2e-upstream","endpointID":%d,"priority":%d,"weight":1}`,
		alias, endpointID, priority))
	code, resp := doJSON(t, client, http.MethodPost, baseURL+"/api/web/v1/model", token, body)
	if code != http.StatusOK {
		t.Fatalf("create model failed: %d %s", code, resp)
	}
	// 创建响应无 model ID 时经列表反查（alias + endpoint 唯一）
	listCode, listResp := doJSON(t, client, http.MethodGet, baseURL+"/api/web/v1/model/list?pageSize=100", token, nil)
	if listCode != http.StatusOK {
		t.Fatalf("list models failed: %d %s", listCode, listResp)
	}
	var parsed struct {
		Items []struct {
			ID         uint   `json:"id"`
			Alias      string `json:"alias"`
			EndpointID struct {
				ID uint `json:"id"`
			} `json:"endpoint"`
		} `json:"items"`
	}
	if err := sonic.Unmarshal(listResp, &parsed); err != nil {
		t.Fatalf("parse model list: %v", err)
	}
	for _, item := range parsed.Items {
		if item.Alias == alias && item.EndpointID.ID == endpointID {
			return item.ID
		}
	}
	t.Fatalf("created model %s not found in list", alias)
	return 0
}

func mustCreateAPIKey(t *testing.T, client *http.Client, baseURL, token, name string) string {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"name":%q}`, name))
	code, resp := doJSON(t, client, http.MethodPost, baseURL+"/api/web/v1/apikey", token, body)
	if code != http.StatusOK {
		t.Fatalf("create apikey failed: %d %s", code, resp)
	}
	var parsed struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := sonic.Unmarshal(resp, &parsed); err != nil {
		t.Fatalf("parse apikey rsp: %v", err)
	}
	if parsed.Data.Key == "" {
		t.Fatalf("apikey secret empty: %s", resp)
	}
	return parsed.Data.Key
}

func mustDelete(t *testing.T, client *http.Client, baseURL, token, resource string, id uint) {
	t.Helper()
	code, resp := doJSON(t, client, http.MethodDelete, fmt.Sprintf("%s%s?id=%d", baseURL, resource, id), token, nil)
	if code != http.StatusOK {
		t.Logf("cleanup %s?id=%d failed: %d %s", resource, id, code, resp)
	}
}
