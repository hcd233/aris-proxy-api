package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initTriggerRouter(group huma.API, handler handler.TriggerHandler, demoAccessor demoport.DemoModuleAccessor, auditSubmitter demoport.DemoSubmitter) {
	huma.Register(group, huma.Operation{
		OperationID: "createTrigger",
		Method:      http.MethodPost,
		Path:        "",
		Summary:     "CreateTrigger",
		Tags:        []string{constant.TagTrigger},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("createTrigger", enum.PermissionAdmin),
		},
	}, handler.HandleCreateTrigger)

	huma.Register(group, huma.Operation{
		OperationID: "listTrigger",
		Method:      http.MethodGet,
		Path:        constant.RoutePathList,
		Summary:     "ListTrigger",
		Tags:        []string{constant.TagTrigger},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionWithDemoMiddleware("listTrigger", enum.PermissionAdmin, enum.DemoModuleTrigger, demoAccessor, auditSubmitter),
		},
	}, handler.HandleListTrigger)

	huma.Register(group, huma.Operation{
		OperationID: "updateTrigger",
		Method:      http.MethodPatch,
		Path:        "",
		Summary:     "UpdateTrigger",
		Tags:        []string{constant.TagTrigger},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("updateTrigger", enum.PermissionAdmin),
		},
	}, handler.HandleUpdateTrigger)

	huma.Register(group, huma.Operation{
		OperationID: "deleteTrigger",
		Method:      http.MethodDelete,
		Path:        "",
		Summary:     "DeleteTrigger",
		Tags:        []string{constant.TagTrigger},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteTrigger", enum.PermissionAdmin),
		},
	}, handler.HandleDeleteTrigger)
}
