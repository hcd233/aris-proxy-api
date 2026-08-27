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
// 路径契约：model/list 以 constant.ClientModelsAPIPrefix + ClientModelsRoutePath
// 对外承诺（客户端 SDK 同源派生），前缀已在常量层迁移至 /api/cli/v1。
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
		OperationID: "reportTraceEvent", Method: http.MethodPost, Path: "/trace/event",
		Summary: "ReportTraceEvent", Description: "Report a codex hook event (API key auth)",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleReportTraceEvent)

	huma.Register(cliGroup, huma.Operation{
		OperationID: "checkTraceClientAPIKey", Method: http.MethodGet, Path: "/trace/client/check",
		Summary: "CheckTraceClientAPIKey", Description: "Validate the trace client API key",
		Tags:     []string{constant.TagTrace},
		Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
	}, deps.TraceHandler.HandleCheckTraceClient)
}
