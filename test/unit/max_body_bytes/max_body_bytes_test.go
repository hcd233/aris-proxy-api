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
	return registerEchoOp(t, app, maxBodyBytes)
}

// newTestAppWithBodyLimit 类似 newTestApp，但显式设置 fiber 层 BodyLimit（全局兜底），
// 并配合 huma MaxBodyBytes=-1（模拟 LLM 代理路由），使仅有 fiber 层限制生效。
func newTestAppWithBodyLimit(t *testing.T, bodyLimit int) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{BodyLimit: bodyLimit})
	unlimited := int64(-1)
	return registerEchoOp(t, app, &unlimited)
}

// registerEchoOp 在给定 fiber app 上注册 echoBody operation（huma）。
func registerEchoOp(t *testing.T, app *fiber.App, maxBodyBytes *int64) *fiber.App {
	t.Helper()
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

// TestFiberBodyLimit_AllowsBodyAboveDefault4MB 验证 fiber 层 BodyLimit
// 显式配置后，超过 fiber 默认 4MB 的 LLM 代理请求体可正常通过。
//
// 背景（Major，2026-08-12 review 发现）：
//   - 此前仅设置 huma MaxBodyBytes=-1，fiber 默认 BodyLimit=4MB（<=0 回落默认），
//     超过 4MB 的单图多模态请求仍 413，与「已放开限制」的注释不符。
//   - 修复：fiber.New 显式配置 constant.MaxHTTPBodyBytes（16MB）兜底。
func TestFiberBodyLimit_AllowsBodyAboveDefault4MB(t *testing.T) {
	t.Parallel()
	if constant.MaxHTTPBodyBytes <= 4*1024*1024 {
		t.Fatalf("constant.MaxHTTPBodyBytes = %d, want > 4MB", constant.MaxHTTPBodyBytes)
	}
	app := newTestAppWithBodyLimit(t, constant.MaxHTTPBodyBytes)

	// 5MB body，超过 fiber 默认 4MB（旧实现会 413），修复后应通过。
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/body", strings.NewReader(bigBody(5*1024*1024)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body := new(strings.Builder)
		_, _ = io.Copy(body, resp.Body)
		t.Fatalf("5MB body 状态码 = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, body)
	}
}

// TestFiberBodyLimit_RejectsBodyAboveLimit 验证 fiber 层 BodyLimit 之上
// 的超大请求体仍被拒绝（防内存 DoS 兜底）。
func TestFiberBodyLimit_RejectsBodyAboveLimit(t *testing.T) {
	t.Parallel()
	app := newTestAppWithBodyLimit(t, constant.MaxHTTPBodyBytes)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/body", strings.NewReader(bigBody(constant.MaxHTTPBodyBytes+1)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		// fiber/fasthttp 在读取阶段即拦截超限 body（测试环境无 HTTP 响应对象）
		t.Logf("超限 body 被拦截: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		body := new(strings.Builder)
		_, _ = io.Copy(body, resp.Body)
		t.Fatalf("超限 body 状态码 = %d, want %d, body=%s", resp.StatusCode, http.StatusRequestEntityTooLarge, body)
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
