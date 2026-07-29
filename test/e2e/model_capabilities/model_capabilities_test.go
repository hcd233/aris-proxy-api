// Package model_capabilities 验证模型能力（capabilities）配置的管理 API 全链路行为。
//
// 需求背景（feature/model-capabilities）：
//   - models 表新增 capabilities 列（输入模态集合，serializer:json，默认 ["text"]）；
//   - 管理 API create / list / update 需对 capabilities 做全链路 round-trip；
//   - 非法集合（空 / 不含 text / 未知成员）必须被业务层拒绝。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT（必填，调用 model 管理接口需 admin 权限）
package model_capabilities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

const e2eHTTPTimeout = 30 * time.Second

type bizError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type listEndpointsRsp struct {
	Endpoints []struct {
		ID uint `json:"id"`
	} `json:"endpoints"`
	Error *bizError `json:"error,omitempty"`
}

type modelItem struct {
	ID           uint     `json:"id"`
	Alias        string   `json:"alias"`
	Capabilities []string `json:"capabilities"`
}

type listModelsRsp struct {
	Models []modelItem `json:"models"`
	Error  *bizError   `json:"error,omitempty"`
}

type commandRsp struct {
	Error *bizError `json:"error,omitempty"`
}

func mustE2EEnv(t *testing.T) (baseURL, jwtToken string) {
	t.Helper()
	baseURL = os.Getenv("BASE_URL")
	jwtToken = os.Getenv("JWT_TOKEN")
	if baseURL == "" || jwtToken == "" {
		t.Skip("BASE_URL and JWT_TOKEN are required for e2e test")
	}
	return strings.TrimRight(baseURL, "/"), jwtToken
}

func newE2EClient() *http.Client {
	return &http.Client{Timeout: e2eHTTPTimeout}
}

// doJSON 发出请求并返回状态码、TraceID 与原始响应体。
func doJSON(t *testing.T, client *http.Client, method, url, jwtToken string, reqBody map[string]any) (statusCode int, traceID string, respBody []byte) {
	t.Helper()
	var reader io.Reader
	if reqBody != nil {
		body, err := sonic.Marshal(reqBody)
		if err != nil {
			t.Fatalf("marshal request body failed: %v", err)
		}
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, reader)
	if err != nil {
		t.Fatalf("build request failed: %v", err)
	}
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+jwtToken)
	if reqBody != nil {
		req.Header.Set("Content-Type", constant.HTTPContentTypeJSON)
	}
	httpResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s failed: %v", method, url, err)
	}
	defer func() { _ = httpResp.Body.Close() }()
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}
	return httpResp.StatusCode, httpResp.Header.Get(constant.HTTPHeaderTraceID), bodyBytes
}

// pickEndpointID 选一个可用 endpoint 挂载模型。
func pickEndpointID(t *testing.T, baseURL, jwtToken string, client *http.Client) uint {
	t.Helper()
	status, traceID, body := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/endpoint/list?page=1&pageSize=1", jwtToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list endpoints status=%d traceID=%s body=%s", status, traceID, string(body))
	}
	var rsp listEndpointsRsp
	if err := sonic.Unmarshal(body, &rsp); err != nil {
		t.Fatalf("unmarshal endpoints failed: %v body=%s", err, string(body))
	}
	if rsp.Error != nil {
		t.Fatalf("list endpoints error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}
	if len(rsp.Endpoints) == 0 {
		t.Skip("no endpoint available to attach model")
	}
	return rsp.Endpoints[0].ID
}

// createModel 创建模型；capabilities 传 nil 表示不显式发送该字段。
func createModel(t *testing.T, baseURL, jwtToken string, client *http.Client, endpointID uint, alias string, capabilities []string) (rsp *commandRsp, traceID string) {
	t.Helper()
	body := map[string]any{
		"alias":           alias,
		"modelName":       "e2e-upstream-model",
		"endpointID":      endpointID,
		"contextLength":   128000,
		"maxOutputTokens": 64000,
	}
	if capabilities != nil {
		body["capabilities"] = capabilities
	}
	status, traceID, raw := doJSON(t, client, http.MethodPost, baseURL+"/api/v1/model", jwtToken, body)
	if status != http.StatusOK {
		t.Fatalf("create model alias=%s status=%d traceID=%s body=%s", alias, status, traceID, string(raw))
	}
	var out commandRsp
	if err := sonic.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal create rsp failed: %v body=%s", err, string(raw))
	}
	return &out, traceID
}

// getModelByAlias 按别名查模型；未命中返回 nil。
func getModelByAlias(t *testing.T, baseURL, jwtToken string, client *http.Client, alias string) *modelItem {
	t.Helper()
	status, traceID, raw := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/model/list?page=1&pageSize=50&query="+alias, jwtToken, nil)
	if status != http.StatusOK {
		t.Fatalf("list models status=%d traceID=%s body=%s", status, traceID, string(raw))
	}
	var rsp listModelsRsp
	if err := sonic.Unmarshal(raw, &rsp); err != nil {
		t.Fatalf("unmarshal models failed: %v body=%s", err, string(raw))
	}
	for i := range rsp.Models {
		if rsp.Models[i].Alias == alias {
			return &rsp.Models[i]
		}
	}
	return nil
}

func updateCapabilities(t *testing.T, baseURL, jwtToken string, client *http.Client, modelID uint, capabilities []string) {
	t.Helper()
	status, traceID, raw := doJSON(t, client, http.MethodPatch, fmt.Sprintf("%s/api/v1/model?id=%d", baseURL, modelID), jwtToken, map[string]any{"capabilities": capabilities})
	if status != http.StatusOK {
		t.Fatalf("update model status=%d traceID=%s body=%s", status, traceID, string(raw))
	}
	var rsp commandRsp
	if err := sonic.Unmarshal(raw, &rsp); err != nil {
		t.Fatalf("unmarshal update rsp failed: %v body=%s", err, string(raw))
	}
	if rsp.Error != nil {
		t.Fatalf("update model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}
}

func cleanupModel(t *testing.T, baseURL, jwtToken string, client *http.Client, modelID *uint) {
	t.Helper()
	if modelID == nil || *modelID == 0 {
		return
	}
	status, traceID, raw := doJSON(t, client, http.MethodDelete, fmt.Sprintf("%s/api/v1/model?id=%d", baseURL, *modelID), jwtToken, nil)
	if status != http.StatusOK {
		t.Logf("cleanup delete model id=%d failed: status=%d traceID=%s body=%s", *modelID, status, traceID, string(raw))
	}
}

// TestModelCapabilities_RoundTrip 验证 create → list → update → list 的 capabilities 全链路读写一致。
func TestModelCapabilities_RoundTrip(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	alias := fmt.Sprintf("e2e-cap-roundtrip-%d", time.Now().UnixNano())
	var modelID uint
	defer func() { cleanupModel(t, baseURL, jwtToken, client, &modelID) }()

	rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias, []string{"text", "image"})
	if rsp.Error != nil {
		t.Fatalf("create model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}

	item := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if item == nil {
		t.Fatalf("created model alias=%s not found in list", alias)
	}
	modelID = item.ID
	if !slices.Equal(item.Capabilities, []string{"text", "image"}) {
		t.Fatalf("list after create: capabilities=%v want [text image]", item.Capabilities)
	}

	updateCapabilities(t, baseURL, jwtToken, client, modelID, []string{"text"})

	updated := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if updated == nil || !slices.Equal(updated.Capabilities, []string{"text"}) {
		t.Fatalf("list after update: capabilities=%v want [text]", updated)
	}
}

// TestModelCapabilities_CreateDefaultsToText 验证不传 capabilities 时服务端兜底为 ["text"]。
func TestModelCapabilities_CreateDefaultsToText(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	alias := fmt.Sprintf("e2e-cap-default-%d", time.Now().UnixNano())
	var modelID uint
	defer func() { cleanupModel(t, baseURL, jwtToken, client, &modelID) }()

	rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias, nil)
	if rsp.Error != nil {
		t.Fatalf("create model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}

	item := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if item == nil {
		t.Fatalf("created model alias=%s not found in list", alias)
	}
	modelID = item.ID
	if !slices.Equal(item.Capabilities, []string{"text"}) {
		t.Fatalf("default capabilities=%v want [text]", item.Capabilities)
	}
}

// TestModelCapabilities_CreateRejectsInvalid 验证非法能力集合被业务层拒绝（HTTP 200 + error 负载）。
func TestModelCapabilities_CreateRejectsInvalid(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	for _, caps := range [][]string{{"image"}, {"text", "blob"}} {
		alias := fmt.Sprintf("e2e-cap-bad-%d", time.Now().UnixNano())
		rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias, caps)
		if rsp.Error == nil {
			if m := getModelByAlias(t, baseURL, jwtToken, client, alias); m != nil {
				id := m.ID
				cleanupModel(t, baseURL, jwtToken, client, &id)
			}
			t.Fatalf("expected business error for capabilities=%v but got success (traceID=%s)", caps, traceID)
		}
	}
}
