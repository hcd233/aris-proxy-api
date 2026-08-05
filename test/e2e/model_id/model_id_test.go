// Package model_id 验证业务 modelId 字段的管理 API 全链路行为。
//
// 需求背景（feature/model-id-management）：
//   - models 表新增 model_id 列（业务模型 ID，string）；
//   - 创建 model 时 modelId 默认值 = alias；
//   - PATCH 可更新 modelId 的值；空串更新必须被业务层拒绝。
//
// 环境变量：
//   - BASE_URL   API 根地址（必填）
//   - JWT_TOKEN  管理员 JWT（必填，调用 model 管理接口需 admin 权限）
package model_id

import (
	"bytes"
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
	ID      uint   `json:"id"`
	Alias   string `json:"alias"`
	ModelID string `json:"modelId"`
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

// createModel 创建模型（不显式传 modelId，验证服务端默认=alias）。
func createModel(t *testing.T, baseURL, jwtToken string, client *http.Client, endpointID uint, alias string) (rsp *commandRsp, traceID string) {
	t.Helper()
	status, traceID, raw := doJSON(t, client, http.MethodPost, baseURL+"/api/v1/model", jwtToken, map[string]any{
		"alias":           alias,
		"upstreamModel":   "e2e-upstream-model",
		"endpointID":      endpointID,
		"contextLength":   128000,
		"maxOutputTokens": 64000,
	})
	if status != http.StatusOK {
		t.Fatalf("create model alias=%s status=%d traceID=%s body=%s", alias, status, traceID, string(raw))
	}
	var out commandRsp
	if err := sonic.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal create rsp failed: %v body=%s", err, string(raw))
	}
	return &out, traceID
}

// createModelWithModelID 创建模型（显式传 modelId，验证服务端按传入值落库）。
func createModelWithModelID(t *testing.T, baseURL, jwtToken string, client *http.Client, endpointID uint, alias, modelID string) (rsp *commandRsp, traceID string) {
	t.Helper()
	status, traceID, raw := doJSON(t, client, http.MethodPost, baseURL+"/api/v1/model", jwtToken, map[string]any{
		"alias":           alias,
		"modelId":         modelID,
		"upstreamModel":   "e2e-upstream-model",
		"endpointID":      endpointID,
		"contextLength":   128000,
		"maxOutputTokens": 64000,
	})
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

// updateModelID PATCH 更新 modelId；expectError=true 时断言返回业务错误。
func updateModelID(t *testing.T, baseURL, jwtToken string, client *http.Client, modelID uint, modelIDValue string, expectError bool) {
	t.Helper()
	status, traceID, raw := doJSON(t, client, http.MethodPatch, fmt.Sprintf("%s/api/v1/model?id=%d", baseURL, modelID), jwtToken, map[string]any{"modelId": modelIDValue})
	if status != http.StatusOK {
		t.Fatalf("update model status=%d traceID=%s body=%s", status, traceID, string(raw))
	}
	var rsp commandRsp
	if err := sonic.Unmarshal(raw, &rsp); err != nil {
		t.Fatalf("unmarshal update rsp failed: %v body=%s", err, string(raw))
	}
	if expectError {
		if rsp.Error == nil {
			t.Fatalf("expected business error for modelId=%q but got success (traceID=%s)", modelIDValue, traceID)
		}
		return
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

// TestModelID_CreateDefaultsToAlias 验证创建 model 时 modelId 默认值 = alias。
func TestModelID_CreateDefaultsToAlias(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	alias := fmt.Sprintf("e2e-mid-default-%d", time.Now().UnixNano())
	var modelID uint
	defer func() { cleanupModel(t, baseURL, jwtToken, client, &modelID) }()

	rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias)
	if rsp.Error != nil {
		t.Fatalf("create model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}

	item := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if item == nil {
		t.Fatalf("created model alias=%s not found in list", alias)
	}
	modelID = item.ID
	if item.ModelID != alias {
		t.Fatalf("modelId after create=%q, want default alias=%q", item.ModelID, alias)
	}
}

// TestModelID_CreateExplicitModelID 验证创建 model 时显式传入 modelId 生效（不落默认 alias）。
func TestModelID_CreateExplicitModelID(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	alias := fmt.Sprintf("e2e-mid-explicit-%d", time.Now().UnixNano())
	customID := "custom-model-id-created"
	var modelID uint
	defer func() { cleanupModel(t, baseURL, jwtToken, client, &modelID) }()

	rsp, traceID := createModelWithModelID(t, baseURL, jwtToken, client, endpointID, alias, customID)
	if rsp.Error != nil {
		t.Fatalf("create model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}

	item := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if item == nil {
		t.Fatalf("created model alias=%s not found in list", alias)
	}
	modelID = item.ID
	if item.ModelID != customID {
		t.Fatalf("modelId after explicit create=%q, want %q", item.ModelID, customID)
	}
}

// TestModelID_UpdateModelID 验证 PATCH 更新 modelId 生效，且空串更新被业务层拒绝。
func TestModelID_UpdateModelID(t *testing.T) {
	t.Parallel()
	baseURL, jwtToken := mustE2EEnv(t)
	client := newE2EClient()
	endpointID := pickEndpointID(t, baseURL, jwtToken, client)

	alias := fmt.Sprintf("e2e-mid-update-%d", time.Now().UnixNano())
	var modelID uint
	defer func() { cleanupModel(t, baseURL, jwtToken, client, &modelID) }()

	rsp, traceID := createModel(t, baseURL, jwtToken, client, endpointID, alias)
	if rsp.Error != nil {
		t.Fatalf("create model error: code=%s msg=%s traceID=%s", rsp.Error.Code, rsp.Error.Message, traceID)
	}
	item := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if item == nil {
		t.Fatalf("created model alias=%s not found in list", alias)
	}
	modelID = item.ID

	// 更新为自定义 modelId
	customID := "custom-model-id-001"
	updateModelID(t, baseURL, jwtToken, client, modelID, customID, false)

	updated := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if updated == nil {
		t.Fatalf("model alias=%s missing after modelId update", alias)
	}
	if updated.ModelID != customID {
		t.Fatalf("modelId after update=%q, want %q", updated.ModelID, customID)
	}

	// 空串更新必须报业务校验错误，且 modelId 保持不变
	updateModelID(t, baseURL, jwtToken, client, modelID, "", true)

	afterEmpty := getModelByAlias(t, baseURL, jwtToken, client, alias)
	if afterEmpty == nil {
		t.Fatalf("model alias=%s missing after rejected empty update", alias)
	}
	if afterEmpty.ModelID != customID {
		t.Fatalf("modelId after rejected empty update=%q, want unchanged %q", afterEmpty.ModelID, customID)
	}
}
