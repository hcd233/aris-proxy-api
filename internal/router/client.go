package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"gorm.io/gorm"
)

// RegisterClientRoutes 注册客户端路由（API Key 鉴权）
//
// 客户端路由直接挂在 /api/v1 下（group 前缀为空），组内路径即绝对路径，
// 与客户端 SDK 使用的 constant.ClientModelsListPath 同源，防止路径脱节导致
// aris model export 拿到 404。
//
//	@param clientGroup huma.API
//	@param clientHandler handler.ClientHandler
//	@param db *gorm.DB
//	@author centonhuang
//	@update 2026-08-27 12:00:00
func RegisterClientRoutes(clientGroup huma.API, clientHandler handler.ClientHandler, db *gorm.DB) {
	clientGroup.UseMiddleware(middleware.APIKeyMiddleware(db))

	huma.Register(clientGroup, huma.Operation{
		OperationID: "listClientModels",
		Method:      http.MethodGet,
		Path:        constant.ClientModelsRoutePath,
		Summary:     "ListClientModels",
		Description: "List enabled models with capabilities for aris client configuration",
		Tags:        []string{constant.TagClient},
		Security: []map[string][]string{
			{constant.SecuritySchemeAPIKey: {}},
		},
	}, clientHandler.HandleListModels)
}
