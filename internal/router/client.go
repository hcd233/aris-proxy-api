package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"gorm.io/gorm"
)

// initClientRouter 初始化客户端路由（API Key 鉴权）
//
//	@param clientGroup huma.API
//	@param clientHandler handler.ClientHandler
//	@param db *gorm.DB
//	@author centonhuang
//	@update 2026-08-26 15:35:00
func initClientRouter(clientGroup huma.API, clientHandler handler.ClientHandler, db *gorm.DB) {
	clientGroup.UseMiddleware(middleware.APIKeyMiddleware(db))

	huma.Register(clientGroup, huma.Operation{
		OperationID: "listClientModels",
		Method:      http.MethodGet,
		Path:        "/model/list",
		Summary:     "ListClientModels",
		Description: "List enabled models with capabilities for aris client configuration",
		Tags:        []string{constant.TagClient},
		Security: []map[string][]string{
			{constant.SecuritySchemeAPIKey: {}},
		},
	}, clientHandler.HandleListModels)
}
