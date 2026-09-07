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
}
