package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// initDatasetRouter 注册数据集导出路由
//
//	@author centonhuang
//	@update 2026-07-03 10:00:00
func initDatasetRouter(datasetGroup huma.API, datasetHandler handler.DatasetHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	datasetGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))

	huma.Register(datasetGroup, huma.Operation{
		OperationID: "previewDataset",
		Method:      http.MethodGet,
		Path:        "/preview",
		Summary:     "PreviewDataset",
		Description: "Preview dataset export statistics (session count, score distribution, model distribution) for the given filter. Admin sees all; user sees only their own API key sessions.",
		Tags:        []string{constant.TagDataset},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("previewDataset", enum.PermissionUser)},
	}, datasetHandler.HandlePreview)

	huma.Register(datasetGroup, huma.Operation{
		OperationID: "exportDataset",
		Method:      http.MethodGet,
		Path:        "/export",
		Summary:     "ExportDataset",
		Description: "Stream export filtered sessions as ShareGPT-format JSONL. Each line is one conversation. Admin sees all; user sees only their own API key sessions.",
		Tags:        []string{constant.TagDataset},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("exportDataset", enum.PermissionUser)},
	}, datasetHandler.HandleExport)
}
