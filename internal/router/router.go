// Package router 路由分区编排入口。
//
// 四个分区，各自唯一鉴权方式：
//
//   - Web:   /api/web/v1/*                          session JWT（公开入口在区内 public 子组，仅限流）
//
//   - CLI:   /api/cli/v1/*                          API Key（aris 客户端）
//
//   - Proxy: /api/openai/v1、/api/anthropic/v1      API Key（前缀对外契约不变）
//
//   - Ops:   根路径健康检查/文档/pprof 等             无鉴权（见 RegisterOpsRouter）
//
//     @author centonhuang
//     @update 2026-08-27
package router

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
)

// APIRouterDependencies API 路由依赖
//
//	@author centonhuang
//	@update 2026-08-16 10:00:00
type APIRouterDependencies struct {
	DB                 *gorm.DB
	Cache              *redis.Client
	AccessSigner       jwt.TokenSigner
	DemoModuleAccessor demoport.DemoModuleAccessor
	DemoAuditSubmitter demoport.DemoSubmitter
	PingHandler        handler.PingHandler
	TokenHandler       handler.TokenHandler
	Oauth2Handler      handler.Oauth2Handler
	UserHandler        handler.UserHandler
	DemoHandler        handler.DemoHandler
	APIKeyHandler      handler.APIKeyHandler
	SessionHandler     handler.SessionHandler
	EndpointHandler    handler.EndpointHandler
	ModelHandler       handler.ModelHandler
	AuditHandler       handler.AuditHandler
	CronHandler        handler.CronHandler
	OpenAIHandler      handler.OpenAIHandler
	AnthropicHandler   handler.AnthropicHandler
	TriggerHandler     handler.TriggerHandler
	MetricsHandler     handler.MetricsHandler
	DatasetHandler     handler.DatasetHandler
	TraceHandler       handler.TraceHandler
	ClientHandler      handler.ClientHandler
	UpstreamHandler    handler.UpstreamHandler
}

// RegisterAPIRouter 注册 API 路由（分区编排入口）。
//
// 各分区的具体路由见 web.go / cli.go 与 proxy 两个 init 函数；
// 运维分区（根路径无鉴权）在 bootstrap 中经 RegisterOpsRouter 单独注册。
//
//	@author centonhuang
//	@update 2025-11-10 17:26:08
func RegisterAPIRouter(humaAPI huma.API, deps APIRouterDependencies) {
	// ── Web 分区 ──
	webRoot := huma.NewGroup(humaAPI, "/api/web/v1")
	RegisterWebAPIRoutes(webRoot, deps)

	// ── CLI 分区 ──
	cliGroup := huma.NewGroup(humaAPI, "/api/cli/v1")
	RegisterCLIAPIRoutes(cliGroup, deps)

	// ── Proxy 分区（前缀不变） ──
	openaiGroup := huma.NewGroup(humaAPI, "/api/openai/v1")
	initOpenAIRouter(openaiGroup, deps.OpenAIHandler, deps.DB, deps.Cache)

	anthropicGroup := huma.NewGroup(humaAPI, "/api/anthropic/v1")
	initAnthropicRouter(anthropicGroup, deps.AnthropicHandler, deps.DB, deps.Cache)
}
