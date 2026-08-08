// Package max_body_bytes LLM 代理请求体大小限制回归测试。
//
// 背景：huma 对未显式设置 MaxBodyBytes 的 operation 默认限制请求体 1MB（huma.go
// ensureMaxBodyBytes）。LLM 代理路由（openai/anthropic）请求体可能包含长上下文、
// 多模态 base64 内容，远超 1MB，导致 413 {"code":10000,"message":"request body
// is too large limit=1048576 bytes"}。本测试验证：
//  1. 显式设置 constant.MaxLLMProxyBodyBytes（-1，不限制）后大 body 请求可达 handler；
//  2. 未设置的 operation（管理路由）仍保留默认 1MB 限制，防止误放宽管理接口。
package max_body_bytes

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// bigBody 构造超过 1MB 默认限制的 JSON 请求体。
func bigBody(size int) string {
	return `{"data":"` + strings.Repeat("a", size) + `"}`
}

// newTestApp 注册一个带 Body 的 operation 并返回 fiber app，模拟路由注册路径。
//
// maxBodyBytes 为 nil 时不设置（走 huma 默认 1MB），否则应用该值。
func newTestApp(t *testing.T, maxBodyBytes *int64) *fiber.App {
	t.Helper()
	app := fiber.New()
	humaAPI := humafiber.New(app, huma.Config{
		OpenAPI:       &huma.OpenAPI{},
		Formats:       huma.DefaultFormats,
		DefaultFormat: constant.DefaultFormatJSON,
	})
	op := huma.Operation{
		OperationID: "echoBody",
		Method:      http.MethodPost,
		Path:        "/body",
	}
	if maxBodyBytes != nil {
		op.MaxBodyBytes = *maxBodyBytes
	}
	// echoRsp 返回带 body 的响应，确保成功路径为 200 而非 204。
	type echoRsp struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}
	huma.Register(humaAPI, op, func(ctx context.Context, input *struct {
		Body struct {
			Data string `json:"data"`
		}
	}) (*echoRsp, error) {
		return &echoRsp{}, nil
	})
	return app
}

// TestLLMProxyBodyLimit_AllowsOversizedBody 验证 LLM 代理路由（MaxBodyBytes=-1）
// 不再对超大请求体返回 413。
//
//	@author centonhuang
//	@update 2026-08-09 10:00:00
func TestLLMProxyBodyLimit_AllowsOversizedBody(t *testing.T) {
	t.Parallel()
	if constant.MaxLLMProxyBodyBytes != -1 {
		t.Fatalf("constant.MaxLLMProxyBodyBytes = %d, want -1 (unlimited)", constant.MaxLLMProxyBodyBytes)
	}
	maxBytes := constant.MaxLLMProxyBodyBytes // Go 常量不可取地址，先拷贝到局部变量
	app := newTestApp(t, &maxBytes)

	// 2MB body，远超默认 1MB 限制。
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/body", strings.NewReader(bigBody(2*1024*1024)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := new(strings.Builder)
		_, _ = io.Copy(body, resp.Body)
		t.Fatalf("LLM 代理路由大 body 请求状态码 = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, body)
	}
}

// TestDefaultBodyLimit_RejectsOversizedBody 验证未显式设置 MaxBodyBytes 的路由
// （管理类接口）仍保留 huma 默认 1MB 限制：2MB body 应返回 413。
//
//	@author centonhuang
//	@update 2026-08-09 10:00:00
func TestDefaultBodyLimit_RejectsOversizedBody(t *testing.T) {
	t.Parallel()
	app := newTestApp(t, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/body", strings.NewReader(bigBody(2*1024*1024)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		body := new(strings.Builder)
		_, _ = io.Copy(body, resp.Body)
		t.Fatalf("默认限制路由大 body 请求状态码 = %d, want %d, body=%s", resp.StatusCode, http.StatusRequestEntityTooLarge, body)
	}
}

// TestDefaultBodyLimit_AllowsNormalBody 验证未显式设置 MaxBodyBytes 的路由
// 正常小请求体不受影响（1MB 内正常通过）。
//
//	@author centonhuang
//	@update 2026-08-09 10:00:00
func TestDefaultBodyLimit_AllowsNormalBody(t *testing.T) {
	t.Parallel()
	app := newTestApp(t, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/body", strings.NewReader(bigBody(1024)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := new(strings.Builder)
		_, _ = io.Copy(body, resp.Body)
		t.Fatalf("默认限制路由正常请求状态码 = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, body)
	}
}
