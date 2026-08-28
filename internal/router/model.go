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

	// OperationID 不能叫 listModels：该 ID 已被 OpenAI 兼容路由 /models 占用
	// （huma 对重复 OperationID 直接 panic）。沿用 anthropicListModels 的前缀式命名。
	huma.Register(modelGroup, huma.Operation{
		OperationID: "listWebModels",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListWebModels",
		Description: "List models in a flat paginated view",
		Tags:        []string{constant.TagModel},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listWebModels", enum.PermissionUser),
		},
	}, modelHandler.HandleListModels)

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
