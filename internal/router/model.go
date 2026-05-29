package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initModelRouter(modelGroup huma.API, modelHandler handler.ModelHandler, db *gorm.DB, cache *redis.Client) {
	modelGroup.UseMiddleware(middleware.JwtMiddleware(db, cache))
	modelGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache, "modelManage", constant.CtxKeyUserID, constant.PeriodManageAPIKey, constant.LimitManageAPIKey,
	))

	huma.Register(modelGroup, huma.Operation{
		OperationID: "createModel",
		Method:      http.MethodPost,
		Path:        "/",
		Summary:     "CreateModel",
		Description: "Create a new model mapping",
		Tags:        []string{"Model"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createModel", enum.PermissionAdmin),
		},
	}, modelHandler.HandleCreateModel)

	huma.Register(modelGroup, huma.Operation{
		OperationID: "listModelMappings",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListModels",
		Description: "List all model mappings",
		Tags:        []string{"Model"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listModels", enum.PermissionAdmin),
		},
	}, modelHandler.HandleListModels)

	huma.Register(modelGroup, huma.Operation{
		OperationID: "updateModel",
		Method:      http.MethodPatch,
		Path:        "/",
		Summary:     "UpdateModel",
		Description: "Update a model mapping",
		Tags:        []string{"Model"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("updateModel", enum.PermissionAdmin),
		},
	}, modelHandler.HandleUpdateModel)

	huma.Register(modelGroup, huma.Operation{
		OperationID: "deleteModel",
		Method:      http.MethodDelete,
		Path:        "/",
		Summary:     "DeleteModel",
		Description: "Delete a model mapping",
		Tags:        []string{"Model"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteModel", enum.PermissionAdmin),
		},
	}, modelHandler.HandleDeleteModel)
}
