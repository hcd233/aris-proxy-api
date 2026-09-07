// Package router CLI 分区路由（/api/cli/v1，API Key 鉴权）
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// RegisterCLIAPIRoutes 注册 CLI 分区路由（aris 客户端）。
//
// 路径契约：组内路径全部取 constant 的 RoutePath 常量，客户端可见的绝对路径
// 由 CLIAPIPrefix + RoutePath 同源派生（trace 上报两条与 model/list 一致，
// 不再各自硬编码）。
//
//	@param cliGroup huma.API
//	@param deps APIRouterDependencies
//	@author centonhuang
//	@update 2026-08-27
func RegisterCLIAPIRoutes(cliGroup huma.API, deps APIRouterDependencies) {
	cliGroup.UseMiddleware(middleware.APIKeyMiddleware(deps.DB))

	huma.Register(cliGroup, huma.Operation{
		OperationID: "listClientModels",
		Method:      http.MethodGet,
		Path:        constant.ClientModelsRoutePath,
		Summary:     "ListClientModels",
		Description: "List enabled models with capabilities for aris client configuration",
		Tags:        []string{constant.TagClient},
		Security: []map[string][]string{
			{constant.SecuritySchemeAPIKey: {}},
		},
	}, deps.ClientHandler.HandleListModels)

	huma.Register(cliGroup, huma.Operation{
		OperationID: "reportTraceEvent", Method: http.MethodPost, Path: constant.ArisClientIngestRoutePath,
		Summary: "ReportTraceEvent", Description: "Report a codex hook event (API key auth)",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleReportTraceEvent)

	huma.Register(cliGroup, huma.Operation{
		OperationID: "checkArisClientAPIKey", Method: http.MethodGet, Path: constant.ArisClientCheckRoutePath,
		Summary: "CheckArisClientAPIKey", Description: "Validate the aris client API key",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleCheckArisClient)

	// 旧路径兼容：#175 更名前的 check 路由，deprecated，供未重装的存量客户端二进制
	// （其 init/status 硬编码旧路径）继续工作；新客户端一律走 ArisClientCheckRoutePath。
	huma.Register(cliGroup, huma.Operation{
		OperationID: "checkTraceClientAPIKeyLegacy", Method: http.MethodGet, Path: constant.TraceClientCheckLegacyRoutePath,
		Summary: "CheckTraceClientAPIKey (legacy)", Description: "Deprecated legacy path of aris client API key check, kept for old client binaries",
		Tags:       []string{constant.TagTrace},
		Security:   []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
		Deprecated: true,
	}, deps.TraceHandler.HandleCheckArisClient)
}
