package model_not_found

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// proxyErrorResponse 是 HTTP 层观测到的模型不存在响应结构。
type proxyErrorResponse struct {
	Error struct {
		Message string  `json:"message"`
		Type    string  `json:"type"`
		Param   *string `json:"param"`
		Code    string  `json:"code"`
	} `json:"error"`
}

// TestSendOpenAIModelNotFoundError_BodyWrappedInError 回归用例：
// OpenAI 模型不存在错误 body 必须按官方格式包装为 {"error":{message,type,param,code}}，
// 否则 OpenAI SDK 的 e.body 为 undefined、错误消息带 "404 " 前缀（用户感知为"404 + 空 body"）。
func TestSendOpenAIModelNotFoundError_BodyWrappedInError(t *testing.T) {
	t.Parallel()
	proxyErr := proxyutil.SendOpenAIModelNotFoundError("nonexistent-model-xyz")
	if proxyErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", proxyErr.StatusCode, http.StatusNotFound)
	}
	if proxyErr.Protocol != enum.ProtocolKindOpenAI {
		t.Fatalf("protocol = %v, want %v", proxyErr.Protocol, enum.ProtocolKindOpenAI)
	}

	var rsp proxyErrorResponse
	if err := sonic.Unmarshal(proxyErr.Body, &rsp); err != nil {
		t.Fatalf("body is not valid JSON: %v; raw=%s", err, string(proxyErr.Body))
	}
	if rsp.Error.Message == "" {
		t.Fatalf("error.message is empty; raw=%s", string(proxyErr.Body))
	}
	if rsp.Error.Type != constant.OpenAIInvalidRequestErrorType {
		t.Fatalf("error.type = %q, want %q", rsp.Error.Type, constant.OpenAIInvalidRequestErrorType)
	}
	if rsp.Error.Code != constant.OpenAIModelNotFoundCode {
		t.Fatalf("error.code = %q, want %q", rsp.Error.Code, constant.OpenAIModelNotFoundCode)
	}
	if rsp.Error.Param != nil {
		t.Fatalf("error.param = %v, want null", *rsp.Error.Param)
	}
	// 必须有 error 顶层包装（SDK 通过 body.get("error") 解析）。
	if !strings.Contains(string(proxyErr.Body), `"error":`) {
		t.Fatalf("body missing top-level error wrapper: %s", string(proxyErr.Body))
	}
}

// TestProxyError404_HTTPBodyNonEmpty 回归用例：*port.ProxyError(404) 经 adapter
// 写出到 HTTP 的响应 body 非空且为标准 OpenAI 错误结构。
func TestProxyError404_HTTPBodyNonEmpty(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Aris Test", "1.0"))

	huma.Register(api, huma.Operation{
		OperationID: "chatCompletion",
		Method:      http.MethodPost,
		Path:        "/chat/completions",
	}, func(ctx context.Context, _ *struct{}) (*huma.StreamResponse, error) {
		return apiutil.AdaptProxyResult(ctx, nil, proxyutil.SendOpenAIModelNotFoundError("nonexistent-model-xyz"), nil)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/chat/completions",
		strings.NewReader(`{"model":"nonexistent-model-xyz","messages":[{"role":"user","content":"hi"}]}`))
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if resp.ContentLength <= 0 {
		t.Fatalf("body must be non-empty on 404 model not found (content-length=%d)", resp.ContentLength)
	}

	buf := make([]byte, resp.ContentLength)
	n, _ := resp.Body.Read(buf)
	var rsp proxyErrorResponse
	if err := sonic.Unmarshal(buf[:n], &rsp); err != nil {
		t.Fatalf("response body is not valid OpenAI error JSON: %v; raw=%q", err, string(buf[:n]))
	}
	if rsp.Error.Code != constant.OpenAIModelNotFoundCode {
		t.Fatalf("error.code = %q, want %q", rsp.Error.Code, constant.OpenAIModelNotFoundCode)
	}
}
