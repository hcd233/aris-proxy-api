// Package client_models 验证客户端模型列表接口行为。
//
// 离线单测直连 handler（fake 端口），验证响应结构与字段裁剪；
// 在线 e2e 由 BASE_URL / API_KEY 环境变量门控。
//
// 本文件补充路由组装回归：客户端 SDK 请求的路径必须出现在服务端真实注册的
// 路由表里，否则 aris model export 会以 404 失败；同时守护公共路由
// （oauth2/login）不得被 client 路由的 API Key 中间件误伤。
package client_models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/router"
)

// fakeOauth2Handler 让 oauth2/login 可被真实调用（不 panic），返回固定重定向 URL
type fakeOauth2Handler struct{}

func (f *fakeOauth2Handler) HandleLogin(_ context.Context, _ *dto.LoginReq) (*dto.HTTPResponse[*dto.LoginResp], error) {
	return apiutil.WrapHTTPResponse(&dto.LoginResp{RedirectURL: "https://github.com/login/oauth/authorize"}, nil)
}

func (f *fakeOauth2Handler) HandleCallback(_ context.Context, _ *dto.CallbackReq) (*dto.HTTPResponse[*dto.CallbackRsp], error) {
	return apiutil.WrapHTTPResponse(&dto.CallbackRsp{}, nil)
}

var _ handler.Oauth2Handler = (*fakeOauth2Handler)(nil)

// TestOAuth2LoginRouteNotAPIKeyProtected 公共登录入口不得被 API Key 中间件拦截。
//
// 回归背景：commit 214fa450 修复 model export 404 时，把 client 路由从
// /api/web/v1/client group 改挂 v1Group，但 RegisterClientRoutes 内的
// UseMiddleware(APIKeyMiddleware) 作用到了 v1Group 上。huma 的
// Group.Middlewares() 会把父组中间件并入之后注册的每个路由，导致
// /api/web/v1/oauth2/login、/api/web/v1/token/refresh 等全部公共路由对无凭据
// 请求（及仅持 JWT 的请求）返回 401，GitHub/Google 登录全线失败。
//
// 本用例与 TestClientModelsRouteMatchesSDKPath 互补：前者守护
// 「client 路由必须可达且鉴权生效」，本用例守护「鉴权不得泄漏到公共路由」。
func TestOAuth2LoginRouteNotAPIKeyProtected(t *testing.T) {
	t.Parallel()
	db := newRouteTestDB(t)

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("oauth2 route leak", "1.0"))
	router.RegisterAPIRouter(api, router.APIRouterDependencies{
		DB:               db,
		PingHandler:      &stubPingHandler{},
		TraceHandler:     &stubTraceHandler{},
		TokenHandler:     &stubTokenHandler{},
		Oauth2Handler:    &fakeOauth2Handler{},
		UserHandler:      &stubUserHandler{},
		DemoHandler:      &stubDemoHandler{},
		APIKeyHandler:    &stubAPIKeyHandler{},
		SessionHandler:   &stubSessionHandler{},
		EndpointHandler:  &stubEndpointHandler{},
		ModelHandler:     &stubModelHandler{},
		UpstreamHandler:  &stubUpstreamHandler{},
		AuditHandler:     &stubAuditHandler{},
		CronHandler:      &stubCronHandler{},
		TriggerHandler:   &stubTriggerHandler{},
		OpenAIHandler:    &stubOpenAIHandler{},
		AnthropicHandler: &stubAnthropicHandler{},
		MetricsHandler:   &stubMetricsHandler{},
		DatasetHandler:   &stubDatasetHandler{},
		ClientHandler:    handler.NewClientHandler(handler.ClientDependencies{List: &fakeListClientModels{}}),
	})

	// 无 Authorization 头：未登录用户发起 GitHub 登录的真实形态
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/web/v1/oauth2/login?platform=github", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("oauth2/login is a public login entry and must NOT require an API key, got 401")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from oauth2/login, got %d", resp.StatusCode)
	}

	// 对照：client 模型分发路由的 API Key 鉴权必须仍然生效（无 key → 401），
	// 防止修复走向另一个极端（鉴权被整体摘除）。
	clientReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, constant.ClientModelsListPath, http.NoBody)
	clientResp, clientErr := app.Test(clientReq)
	if clientErr != nil {
		t.Fatalf("app.Test error on client route: %v", clientErr)
	}
	defer func() { _ = clientResp.Body.Close() }()
	if clientResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("client model list must still require an API key, got %d", clientResp.StatusCode)
	}
}
