package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
)

func initModelRouter(modelGroup huma.API, modelHandler handler.ModelHandler, cache *redis.Client) {
	modelGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache, "modelManage", constant.CtxKeyUserID, constant.PeriodManageAPIKey, constant.LimitManageAPIKey,
	))

	huma.Register(modelGroup, huma.Operation{
		OperationID: "createModel",
		Method:      http.MethodPost,
		Path:        "",
		Summary:     "CreateModel",
		Description: "Create a new model mapping",
		Tags:        []string{constant.TagModel},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createModel", enum.PermissionUser),
		},
	}, modelHandler.HandleCreateModel)

	huma.Register(modelGroup, huma.Operation{
		OperationID: "updateModel",
		Method:      http.MethodPatch,
		Path:        "",
		Summary:     "UpdateModel",
		Description: "Update a model mapping",
		Tags:        []string{constant.TagModel},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("updateModel", enum.PermissionUser),
		},
	}, modelHandler.HandleUpdateModel)

	huma.Register(modelGroup, huma.Operation{
		OperationID: "deleteModel",
		Method:      http.MethodDelete,
		Path:        "",
		Summary:     "DeleteModel",
		Description: "Delete a model mapping",
		Tags:        []string{constant.TagModel},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteModel", enum.PermissionUser),
		},
	}, modelHandler.HandleDeleteModel)
}
